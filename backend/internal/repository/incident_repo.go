package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"security-portal/internal/db"
	"security-portal/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IncidentRepository es la interfaz que ven services. Trabaja siempre con
// uuid.UUID y models.Incident; los strings y tipos crudos del paquete db
// no salen de este archivo.
type IncidentRepository interface {
	Create(ctx context.Context, incident *models.Incident) error
	MarkSynced(ctx context.Context, id uuid.UUID, jiraKey string) error
	MarkSyncFailed(ctx context.Context, id uuid.UUID) error
	// ClaimPendingSync reclama atómicamente hasta `limit` incidentes
	// pendientes de sincronizar. Aplica FOR UPDATE SKIP LOCKED (seguro con
	// múltiples réplicas) y backoff exponencial (last_sync_attempt <
	// NOW() - baseBackoff * 2^sync_retries). Marca last_sync_attempt = NOW()
	// como claim blando antes de devolver.
	ClaimPendingSync(ctx context.Context, maxRetries int, baseBackoff time.Duration, limit int) ([]models.Incident, error)
	// PurgeOldMetadata pone metadata = NULL en incidentes con created_at <
	// olderThan. La fila se conserva; solo desaparece la PII (IP, user-agent,
	// received_at). Devuelve la cantidad de filas afectadas.
	PurgeOldMetadata(ctx context.Context, olderThan time.Time) (int64, error)
}

type postgresIncidentRepo struct {
	q         *db.Queries
	opTimeout time.Duration
}

func NewIncidentRepository(pool *pgxpool.Pool, opTimeout time.Duration) IncidentRepository {
	if opTimeout <= 0 {
		opTimeout = 5 * time.Second
	}
	return &postgresIncidentRepo{q: db.New(pool), opTimeout: opTimeout}
}

func (r *postgresIncidentRepo) Create(ctx context.Context, incident *models.Incident) error {
	ctx, cancel := context.WithTimeout(ctx, r.opTimeout)
	defer cancel()

	var metadataBytes []byte
	if incident.Metadata != nil {
		b, err := json.Marshal(incident.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		metadataBytes = b
	}

	row, err := r.q.CreateIncident(ctx, db.CreateIncidentParams{
		Title:       incident.Title,
		Description: incident.Description,
		Author:      incident.Author,
		Metadata:    metadataBytes,
	})
	if err != nil {
		return err
	}

	// row.ID es string (override sqlc); lo parseamos al tipo de dominio.
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return fmt.Errorf("uuid inválido devuelto por DB: %w", err)
	}
	incident.ID = id
	incident.CreatedAt = row.CreatedAt
	incident.JiraSync = row.JiraSync
	return nil
}

func (r *postgresIncidentRepo) MarkSynced(ctx context.Context, id uuid.UUID, jiraKey string) error {
	ctx, cancel := context.WithTimeout(ctx, r.opTimeout)
	defer cancel()

	return r.q.MarkIncidentSynced(ctx, db.MarkIncidentSyncedParams{
		ID:           id.String(),
		JiraIssueKey: &jiraKey,
	})
}

func (r *postgresIncidentRepo) MarkSyncFailed(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.opTimeout)
	defer cancel()
	return r.q.MarkIncidentSyncFailed(ctx, id.String())
}

func (r *postgresIncidentRepo) PurgeOldMetadata(ctx context.Context, olderThan time.Time) (int64, error) {
	// Más laxo que opTimeout: el UPDATE puede tocar muchas filas si la tabla
	// creció. 30s deja margen sin colgar el worker indefinidamente.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return r.q.PurgeOldMetadata(ctx, olderThan)
}

func (r *postgresIncidentRepo) ClaimPendingSync(ctx context.Context, maxRetries int, baseBackoff time.Duration, limit int) ([]models.Incident, error) {
	ctx, cancel := context.WithTimeout(ctx, r.opTimeout)
	defer cancel()

	// El claim necesita un base de backoff > 0; si llega 0 el SQL multiplicaría
	// por intervalo cero y todas las filas con last_sync_attempt no nulo
	// pasarían el filtro inmediatamente. Default conservador.
	if baseBackoff <= 0 {
		baseBackoff = 30 * time.Second
	}

	rows, err := r.q.ClaimPendingSync(ctx, db.ClaimPendingSyncParams{
		SyncRetries:     int32(maxRetries),
		BaseBackoffSecs: int32(baseBackoff.Seconds()),
		Limit:           int32(limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]models.Incident, 0, len(rows))
	for _, row := range rows {
		inc, err := toDomain(row)
		if err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, nil
}
