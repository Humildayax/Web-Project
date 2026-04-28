package services

import (
	"context"
	"log/slog"
	"time"

	"security-portal/internal/jira"
	"security-portal/internal/repository"
)

type JiraRetryWorker struct {
	repo        repository.IncidentRepository
	jira        jira.Client
	interval    time.Duration
	baseBackoff time.Duration
	maxRetries  int
}

func NewJiraRetryWorker(repo repository.IncidentRepository, client jira.Client, interval, baseBackoff time.Duration, maxRetries int) *JiraRetryWorker {
	return &JiraRetryWorker{repo: repo, jira: client, interval: interval, baseBackoff: baseBackoff, maxRetries: maxRetries}
}

func (w *JiraRetryWorker) Run(ctx context.Context) {
	slog.Info("jira retry worker iniciado",
		"interval", w.interval,
		"base_backoff", w.baseBackoff,
		"max_retries", w.maxRetries,
	)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("jira retry worker detenido")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *JiraRetryWorker) runOnce(ctx context.Context) {
	// ClaimPendingSync hace en una sola sentencia atómica:
	//   1. SELECT … FOR UPDATE SKIP LOCKED (otras réplicas no ven estas filas)
	//   2. Filtra por backoff exponencial respecto a last_sync_attempt
	//   3. UPDATE last_sync_attempt = NOW() (claim blando)
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
			// Si el contador acaba de tocar el techo, este incidente no
			// volverá a entrar en el WHERE de ClaimPendingSync. Loggeamos
			// una sola vez como ERROR para que sea visible en alertas/SIEM:
			// pasa a ser "zombi" hasta que un operador lo destrabe.
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
