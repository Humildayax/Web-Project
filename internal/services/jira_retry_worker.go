package services

import (
	"context"
	"log/slog"
	"time"

	"security-portal/internal/jira"
	"security-portal/internal/repository"
)

type JiraRetryWorker struct {
	repo       repository.IncidentRepository
	jira       jira.Client
	interval   time.Duration
	maxRetries int
}

func NewJiraRetryWorker(repo repository.IncidentRepository, client jira.Client, interval time.Duration, maxRetries int) *JiraRetryWorker {
	return &JiraRetryWorker{repo: repo, jira: client, interval: interval, maxRetries: maxRetries}
}

func (w *JiraRetryWorker) Run(ctx context.Context) {
	slog.Info("jira retry worker iniciado", "interval", w.interval, "max_retries", w.maxRetries)
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
	pending, err := w.repo.ListPendingSync(ctx, w.maxRetries, 50)
	if err != nil {
		slog.Error("retry: listando pendientes", "err", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	slog.Info("retry: procesando pendientes", "count", len(pending))

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
