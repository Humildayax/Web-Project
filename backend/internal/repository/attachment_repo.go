package repository

import (
	"context"
	"fmt"
	"time"

	"security-portal/internal/db"
	"security-portal/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AttachmentRepository provee acceso a la tabla incident_attachments.
// Trabaja con uuid.UUID y models.Attachment; los strings y tipos crudos
// del paquete db no salen de este archivo.
type AttachmentRepository interface {
	Create(ctx context.Context, a *models.Attachment) error
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.Attachment, error)

	// ListPendingJiraUpload devuelve adjuntos cuyos incidentes ya están
	// sincronizados a JIRA pero el binario aún no se subió al ticket.
	// Devuelve también jira_issue_key del incidente padre para evitar
	// otro round-trip a la DB.
	ListPendingJiraUpload(ctx context.Context, limit int) ([]AttachmentForUpload, error)
	MarkUploadedToJira(ctx context.Context, id uuid.UUID) error

	// ListPurgeable devuelve adjuntos cuyo binario hay que borrar del
	// disco por TTL. La fila se conserva como audit (cuántos había, qué
	// hashes); solo se elimina el blob.
	ListPurgeable(ctx context.Context, olderThan time.Time, limit int) ([]models.Attachment, error)
	MarkPurged(ctx context.Context, id uuid.UUID) error
}

// AttachmentForUpload empaqueta un adjunto + el jira_issue_key del
// incidente padre, para que el worker pueda hacer el upload sin un
// round-trip extra a la DB.
type AttachmentForUpload struct {
	Attachment   models.Attachment
	JiraIssueKey string
}

type postgresAttachmentRepo struct {
	q         *db.Queries
	opTimeout time.Duration
}

func NewAttachmentRepository(pool *pgxpool.Pool, opTimeout time.Duration) AttachmentRepository {
	if opTimeout <= 0 {
		opTimeout = 5 * time.Second
	}
	return &postgresAttachmentRepo{q: db.New(pool), opTimeout: opTimeout}
}

func (r *postgresAttachmentRepo) Create(ctx context.Context, a *models.Attachment) error {
	ctx, cancel := context.WithTimeout(ctx, r.opTimeout)
	defer cancel()

	row, err := r.q.CreateAttachment(ctx, db.CreateAttachmentParams{
		IncidentID:       a.IncidentID.String(),
		FilenameOriginal: a.FilenameOriginal,
		FilenameStored:   a.FilenameStored,
		MimeType:         a.MimeType,
		SizeBytes:        a.SizeBytes,
		Sha256:           a.SHA256,
	})
	if err != nil {
		return err
	}
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return fmt.Errorf("uuid inválido devuelto por DB: %w", err)
	}
	a.ID = id
	a.CreatedAt = row.CreatedAt
	return nil
}

func (r *postgresAttachmentRepo) ListPendingJiraUpload(ctx context.Context, limit int) ([]AttachmentForUpload, error) {
	ctx, cancel := context.WithTimeout(ctx, r.opTimeout)
	defer cancel()

	rows, err := r.q.ListAttachmentsPendingJiraUpload(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]AttachmentForUpload, 0, len(rows))
	for _, row := range rows {
		// El WHERE garantiza jira_issue_key NOT NULL, pero defensiva.
		if row.JiraIssueKey == nil {
			continue
		}
		// Convertir el row del JOIN a un db.IncidentAttachment para reusar
		// el mapper toDomain.
		att, err := attachmentToDomain(db.IncidentAttachment{
			ID:               row.ID,
			IncidentID:       row.IncidentID,
			FilenameOriginal: row.FilenameOriginal,
			FilenameStored:   row.FilenameStored,
			MimeType:         row.MimeType,
			SizeBytes:        row.SizeBytes,
			Sha256:           row.Sha256,
			CreatedAt:        row.CreatedAt,
			UploadedToJiraAt: row.UploadedToJiraAt,
			PurgedAt:         row.PurgedAt,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, AttachmentForUpload{
			Attachment:   att,
			JiraIssueKey: *row.JiraIssueKey,
		})
	}
	return out, nil
}

func (r *postgresAttachmentRepo) MarkUploadedToJira(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.opTimeout)
	defer cancel()
	return r.q.MarkAttachmentUploadedToJira(ctx, id.String())
}

func (r *postgresAttachmentRepo) ListPurgeable(ctx context.Context, olderThan time.Time, limit int) ([]models.Attachment, error) {
	// Más laxo: el listing puede tocar índices grandes si la tabla creció.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := r.q.ListAttachmentsPurgeable(ctx, db.ListAttachmentsPurgeableParams{
		CreatedAt: olderThan,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]models.Attachment, 0, len(rows))
	for _, row := range rows {
		att, err := attachmentToDomain(row)
		if err != nil {
			return nil, err
		}
		out = append(out, att)
	}
	return out, nil
}

func (r *postgresAttachmentRepo) MarkPurged(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.opTimeout)
	defer cancel()
	return r.q.MarkAttachmentPurged(ctx, id.String())
}

func (r *postgresAttachmentRepo) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.Attachment, error) {
	ctx, cancel := context.WithTimeout(ctx, r.opTimeout)
	defer cancel()

	rows, err := r.q.ListAttachmentsByIncident(ctx, incidentID.String())
	if err != nil {
		return nil, err
	}

	out := make([]models.Attachment, 0, len(rows))
	for _, row := range rows {
		att, err := attachmentToDomain(row)
		if err != nil {
			return nil, err
		}
		out = append(out, att)
	}
	return out, nil
}

func attachmentToDomain(row db.IncidentAttachment) (models.Attachment, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return models.Attachment{}, fmt.Errorf("attachment uuid inválido %q: %w", row.ID, err)
	}
	incID, err := uuid.Parse(row.IncidentID)
	if err != nil {
		return models.Attachment{}, fmt.Errorf("incident uuid inválido %q: %w", row.IncidentID, err)
	}
	return models.Attachment{
		ID:               id,
		IncidentID:       incID,
		FilenameOriginal: row.FilenameOriginal,
		FilenameStored:   row.FilenameStored,
		MimeType:         row.MimeType,
		SizeBytes:        row.SizeBytes,
		SHA256:           row.Sha256,
		CreatedAt:        row.CreatedAt,
		UploadedToJiraAt: row.UploadedToJiraAt,
		PurgedAt:         row.PurgedAt,
	}, nil
}
