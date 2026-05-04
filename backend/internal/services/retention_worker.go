package services

import (
	"context"
	"log/slog"
	"time"

	"security-portal/internal/repository"
	"security-portal/internal/storage"
)

// RetentionWorker borra periódicamente la PII de incidentes viejos. Hace
// dos pasadas en cada tick:
//   - PurgeOldMetadata: UPDATE incidents SET metadata = NULL en bulk para
//     todas las filas con created_at < cutoff.
//   - PurgeOldAttachments: por cada adjunto purgable, borra el binario del
//     storage y marca purged_at en la fila DB. La fila se conserva como
//     audit trail (cuántos había, qué hashes, qué nombres).
//
// Los adjuntos se borran independientemente de si llegaron o no a JIRA.
// El TTL es para preservación de PII; si en N días el upload nunca tuvo
// éxito, el operador ya tuvo tiempo de notarlo en logs y es preferible
// cumplir la política de retención que conservar evidencia indefinidamente.
type RetentionWorker struct {
	repo        repository.IncidentRepository
	attachments repository.AttachmentRepository
	storage     storage.Storage
	maxAge      time.Duration
	interval    time.Duration
	batch       int
}

func NewRetentionWorker(
	repo repository.IncidentRepository,
	attachments repository.AttachmentRepository,
	store storage.Storage,
	maxAge, interval time.Duration,
) *RetentionWorker {
	return &RetentionWorker{
		repo:        repo,
		attachments: attachments,
		storage:     store,
		maxAge:      maxAge,
		interval:    interval,
		batch:       100,
	}
}

func (w *RetentionWorker) Run(ctx context.Context) {
	slog.Info("retention worker iniciado", "max_age", w.maxAge, "interval", w.interval)

	// Corremos una vez al arrancar (no esperar 24h en el primer arranque),
	// después seguimos con el ticker.
	w.runOnce(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("retention worker detenido")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *RetentionWorker) runOnce(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-w.maxAge)
	w.purgeMetadata(ctx, cutoff)
	w.purgeAttachments(ctx, cutoff)
}

func (w *RetentionWorker) purgeMetadata(ctx context.Context, cutoff time.Time) {
	n, err := w.repo.PurgeOldMetadata(ctx, cutoff)
	if err != nil {
		slog.Error("retention: purga metadata falló", "err", err, "cutoff", cutoff)
		return
	}
	if n > 0 {
		slog.Info("retention: metadata purgada", "rows", n, "cutoff", cutoff)
	}
}

func (w *RetentionWorker) purgeAttachments(ctx context.Context, cutoff time.Time) {
	purgeable, err := w.attachments.ListPurgeable(ctx, cutoff, w.batch)
	if err != nil {
		slog.Error("retention: listar adjuntos purgables", "err", err)
		return
	}
	if len(purgeable) == 0 {
		return
	}

	deleted, missing, failed := 0, 0, 0
	for _, att := range purgeable {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Si el adjunto nunca llegó a JIRA, lo loggeamos como WARN antes
		// de borrar — para que el operador sepa que la evidencia se
		// está perdiendo.
		if att.UploadedToJiraAt == nil {
			slog.Warn("retention: borrando binario que nunca llegó a JIRA",
				"attachment_id", att.ID, "incident_id", att.IncidentID,
				"created_at", att.CreatedAt)
		}

		if err := w.storage.Delete(ctx, att.IncidentID.String(), att.FilenameStored); err != nil {
			slog.Warn("retention: delete del storage falló",
				"attachment_id", att.ID, "file", att.FilenameStored, "err", err)
			failed++
			continue
		}
		if err := w.attachments.MarkPurged(ctx, att.ID); err != nil {
			slog.Error("retention: archivo borrado pero mark purged falló",
				"attachment_id", att.ID, "err", err)
			failed++
			continue
		}
		if att.UploadedToJiraAt == nil {
			missing++
		}
		deleted++
	}
	slog.Info("retention: adjuntos purgados",
		"deleted", deleted, "never_uploaded", missing, "failed", failed, "cutoff", cutoff)
}
