package services

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"security-portal/internal/jira"
	"security-portal/internal/repository"
	"security-portal/internal/storage"
)

// JiraRetryWorker corre cada `interval` y hace dos cosas en orden:
//  1. Reintenta sincronizar incidentes que aún no llegaron a JIRA
//     (incluye reclaim atómico con FOR UPDATE SKIP LOCKED y backoff
//     exponencial sobre last_sync_attempt).
//  2. Sube los binarios de adjuntos cuyos incidentes ya están sincronizados
//     pero el archivo aún no se subió al ticket.
//
// La semántica de delivery es at-least-once: si crasheamos entre el
// upload exitoso a JIRA y el MarkUploadedToJira en DB, en el próximo
// tick subimos el archivo otra vez (JIRA acepta el duplicado). Para
// "exactly once" haría falta un lock distribuido y/o idempotency keys
// del lado de JIRA — fuera de scope.
type JiraRetryWorker struct {
	repo            repository.IncidentRepository
	attachments     repository.AttachmentRepository
	storage         storage.Storage
	jira            jira.Client
	interval        time.Duration
	baseBackoff     time.Duration
	maxRetries      int
	attachmentBatch int
}

func NewJiraRetryWorker(
	repo repository.IncidentRepository,
	attachments repository.AttachmentRepository,
	store storage.Storage,
	client jira.Client,
	interval, baseBackoff time.Duration,
	maxRetries int,
	attachmentBatch int,
) *JiraRetryWorker {
	if attachmentBatch <= 0 {
		attachmentBatch = 20
	}
	return &JiraRetryWorker{
		repo:            repo,
		attachments:     attachments,
		storage:         store,
		jira:            client,
		interval:        interval,
		baseBackoff:     baseBackoff,
		maxRetries:      maxRetries,
		attachmentBatch: attachmentBatch,
	}
}

func (w *JiraRetryWorker) Run(ctx context.Context) {
	slog.Info("jira retry worker iniciado",
		"interval", w.interval,
		"base_backoff", w.baseBackoff,
		"max_retries", w.maxRetries,
		"attachment_batch", w.attachmentBatch,
	)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("jira retry worker detenido")
			return
		case <-ticker.C:
			w.processIncidents(ctx)
			w.processAttachments(ctx)
		}
	}
}

func (w *JiraRetryWorker) processIncidents(ctx context.Context) {
	pending, err := w.repo.ClaimPendingSync(ctx, w.maxRetries, w.baseBackoff, 50)
	if err != nil {
		slog.Error("retry: claim de pendientes", "err", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	slog.Info("retry: procesando claimed", "count", len(pending))

	for i := range pending {
		select {
		case <-ctx.Done():
			return
		default:
		}

		inc := &pending[i]
		ref, err := w.jira.CreateIssue(ctx, inc)
		if err != nil {
			slog.Warn("retry: sync falló", "incident_id", inc.ID, "err", err)
			if markErr := w.repo.MarkSyncFailed(ctx, inc.ID); markErr != nil {
				slog.Error("retry: no pude actualizar contador", "err", markErr)
				continue
			}
			if int(inc.SyncRetries)+1 >= w.maxRetries {
				slog.Error("retry: incidente alcanzó max_retries, queda sin sincronizar",
					"incident_id", inc.ID,
					"retries", inc.SyncRetries+1,
					"max_retries", w.maxRetries,
				)
			}
			continue
		}
		if err := w.repo.MarkSynced(ctx, inc.ID, ref.Key); err != nil {
			slog.Error("retry: sync ok pero update falló", "incident_id", inc.ID, "err", err)
			continue
		}
		slog.Info("retry: incidente sincronizado", "incident_id", inc.ID, "jira_key", ref.Key)
	}
}

// processAttachments sube los binarios pendientes a su ticket JIRA.
// Asume una sola réplica del backend (no usa SKIP LOCKED). Si dos
// réplicas corrieran a la vez podrían subir un mismo archivo dos veces.
func (w *JiraRetryWorker) processAttachments(ctx context.Context) {
	pending, err := w.attachments.ListPendingJiraUpload(ctx, w.attachmentBatch)
	if err != nil {
		slog.Error("retry: listar adjuntos pendientes", "err", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	slog.Info("retry: subiendo adjuntos a jira", "count", len(pending))

	for _, p := range pending {
		select {
		case <-ctx.Done():
			return
		default:
		}

		att := p.Attachment
		data, err := w.storage.Read(ctx, att.IncidentID.String(), att.FilenameStored)
		if err != nil {
			// Si el archivo no existe en disco (ya fue purgado o nunca
			// se escribió por algún error pasado), saltamos. Marcamos
			// purged_at para que no vuelva a aparecer en la lista.
			if errors.Is(err, os.ErrNotExist) {
				slog.Warn("retry: binario no existe en disco, marcando purged",
					"attachment_id", att.ID, "file", att.FilenameStored)
				if mErr := w.attachments.MarkPurged(ctx, att.ID); mErr != nil {
					slog.Error("retry: no pude marcar purged", "err", mErr)
				}
				continue
			}
			slog.Warn("retry: leer binario falló", "attachment_id", att.ID, "err", err)
			continue
		}

		// JIRA recibe el filename original (legible). El stored es solo
		// el path interno del FS.
		if err := w.jira.AttachToIssue(ctx, p.JiraIssueKey, att.FilenameOriginal, att.MimeType, data); err != nil {
			slog.Warn("retry: upload de adjunto a jira falló",
				"attachment_id", att.ID, "issue_key", p.JiraIssueKey, "err", err)
			continue
		}
		if err := w.attachments.MarkUploadedToJira(ctx, att.ID); err != nil {
			// Si falla acá: el archivo está en JIRA pero no marcamos en DB.
			// Próximo tick lo intentará de nuevo y JIRA quedará con dos
			// copias del adjunto. At-least-once por diseño.
			slog.Error("retry: upload ok pero mark uploaded falló",
				"attachment_id", att.ID, "issue_key", p.JiraIssueKey, "err", err)
			continue
		}
		slog.Info("retry: adjunto subido a jira",
			"attachment_id", att.ID, "issue_key", p.JiraIssueKey, "filename", att.FilenameOriginal)
	}
}
