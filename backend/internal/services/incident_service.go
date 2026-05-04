package services

import (
	"context"
	"fmt"
	"log/slog"

	"security-portal/internal/jira"
	"security-portal/internal/models"
	"security-portal/internal/repository"
	"security-portal/internal/storage"

	"github.com/google/uuid"
)

// ProcessedAttachment es el contrato que el handler le pasa al service
// después de validar y re-encodear cada archivo subido. El service no
// sabe nada de multipart ni de imágenes — recibe bytes opacos listos
// para guardar.
type ProcessedAttachment struct {
	FilenameOriginal string // sanitizado, listo para mostrar
	Data             []byte // binario re-encodeado, seguro
	MimeType         string // del binario re-encodeado
	Extension        string // ".jpg" o ".png", incluye el punto
	SHA256           string // hex
}

type IncidentService struct {
	repo            repository.IncidentRepository
	attachments     repository.AttachmentRepository
	storage         storage.Storage
	jira            jira.Client
}

func NewIncidentService(
	repo repository.IncidentRepository,
	attachments repository.AttachmentRepository,
	storage storage.Storage,
	jiraClient jira.Client,
) *IncidentService {
	return &IncidentService{
		repo:        repo,
		attachments: attachments,
		storage:     storage,
		jira:        jiraClient,
	}
}

// ProcessNewIncident sigue la política DB-first:
//  1. Guarda en Postgres (fuente de verdad) con jira_sync=false.
//  2. Persiste los adjuntos: cada archivo se escribe a disco y se registra
//     una fila en incident_attachments. Si algo falla acá, hacemos cleanup
//     best-effort de los archivos ya escritos para no dejar huérfanos.
//  3. Intenta crear el ticket en JIRA.
//  4. Si JIRA falla, NO propaga el error al cliente: el incidente ya está
//     a salvo en la DB y el worker lo reintentará. Los adjuntos por ahora
//     no se suben a JIRA (eso es la tanda 3b).
//
// Devuelve los attachments creados (para incluirlos en la respuesta HTTP).
func (s *IncidentService) ProcessNewIncident(
	ctx context.Context,
	incident *models.Incident,
	processed []ProcessedAttachment,
) ([]models.Attachment, error) {
	if err := s.repo.Create(ctx, incident); err != nil {
		return nil, fmt.Errorf("guardando en BD local: %w", err)
	}

	saved, err := s.persistAttachments(ctx, incident.ID, processed)
	if err != nil {
		// El incidente ya está creado y es válido sin adjuntos; dejarlo.
		// El cliente recibe 500 y puede reintentar — generará un incidente
		// nuevo. El primero queda como huérfano sin adjuntos.
		return nil, fmt.Errorf("guardando adjuntos: %w", err)
	}

	ref, err := s.jira.CreateIssue(ctx, incident)
	if err != nil {
		slog.Warn("sync inicial con jira falló; queda pendiente para retry",
			"incident_id", incident.ID, "err", err)
		return saved, nil
	}

	if err := s.repo.MarkSynced(ctx, incident.ID, ref.Key); err != nil {
		slog.Error("jira creó el ticket pero el update local falló",
			"incident_id", incident.ID, "jira_key", ref.Key, "err", err)
		return saved, nil
	}

	incident.JiraSync = true
	incident.JiraIssueKey = ref.Key
	return saved, nil
}

// persistAttachments escribe los archivos al storage y crea las filas en DB.
// Si algo falla a mitad, hace cleanup best-effort de lo ya escrito.
func (s *IncidentService) persistAttachments(
	ctx context.Context,
	incidentID uuid.UUID,
	processed []ProcessedAttachment,
) ([]models.Attachment, error) {
	if len(processed) == 0 {
		return nil, nil
	}

	saved := make([]models.Attachment, 0, len(processed))
	cleanup := func() {
		// Borrar archivos físicos. Las filas DB se borran por CASCADE
		// si en algún momento se borrara el incidente; acá solo
		// quitamos los binarios para no dejar basura en disco.
		for _, a := range saved {
			if err := s.storage.Delete(ctx, incidentID.String(), a.FilenameStored); err != nil {
				slog.Warn("cleanup: no pude borrar archivo",
					"incident_id", incidentID, "file", a.FilenameStored, "err", err)
			}
		}
	}

	for _, p := range processed {
		// Filename de disco: <uuid>.<ext>. El UUID evita colisiones y path
		// traversal (el storage valida el segmento igualmente).
		stored := uuid.NewString() + p.Extension

		if err := s.storage.Save(ctx, incidentID.String(), stored, p.Data); err != nil {
			cleanup()
			return nil, fmt.Errorf("storage save: %w", err)
		}

		att := &models.Attachment{
			IncidentID:       incidentID,
			FilenameOriginal: p.FilenameOriginal,
			FilenameStored:   stored,
			MimeType:         p.MimeType,
			SizeBytes:        int64(len(p.Data)),
			SHA256:           p.SHA256,
		}
		if err := s.attachments.Create(ctx, att); err != nil {
			// Borrar el archivo que acabamos de escribir + los anteriores.
			if delErr := s.storage.Delete(ctx, incidentID.String(), stored); delErr != nil {
				slog.Warn("cleanup: no pude borrar último archivo",
					"file", stored, "err", delErr)
			}
			cleanup()
			return nil, fmt.Errorf("attachment repo create: %w", err)
		}
		saved = append(saved, *att)
	}
	return saved, nil
}
