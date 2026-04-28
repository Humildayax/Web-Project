package services

import (
	"context"
	"log/slog"
	"time"

	"security-portal/internal/repository"
)

// RetentionWorker borra periódicamente la metadata operativa (IP, UA,
// received_at) de incidentes más viejos que MaxAge. La fila del incidente
// se conserva; lo que desaparece es la PII.
//
// Política DB-first como el JiraRetryWorker: si la corrida falla, se loggea
// y se reintenta en el siguiente tick. No hay reintentos dentro de un mismo
// ciclo (basta con esperar al próximo).
type RetentionWorker struct {
	repo     repository.IncidentRepository
	maxAge   time.Duration
	interval time.Duration
}

func NewRetentionWorker(repo repository.IncidentRepository, maxAge, interval time.Duration) *RetentionWorker {
	return &RetentionWorker{repo: repo, maxAge: maxAge, interval: interval}
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
	n, err := w.repo.PurgeOldMetadata(ctx, cutoff)
	if err != nil {
		slog.Error("retention: purga falló", "err", err, "cutoff", cutoff)
		return
	}
	if n > 0 {
		slog.Info("retention: metadata purgada", "rows", n, "cutoff", cutoff)
	}
}
