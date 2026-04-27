package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"security-portal/internal/db"
	"security-portal/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IncidentRepository es la interfaz que ven services. Trabaja siempre con
// uuid.UUID y models.Incident; los strings y tipos pgtype.* del paquete db
// no salen de este archivo.
type IncidentRepository interface {
	Create(ctx context.Context, incident *models.Incident) error
	MarkSynced(ctx context.Context, id uuid.UUID, jiraKey string) error
	MarkSyncFailed(ctx context.Context, id uuid.UUID) error
	ListPendingSync(ctx context.Context, maxRetries, limit int) ([]models.Incident, error)
}

type postgresIncidentRepo struct {
	q *db.Queries
}

func NewIncidentRepository(pool *pgxpool.Pool) IncidentRepository {
	return &postgresIncidentRepo{q: db.New(pool)}
}

const opTimeout = 5 * time.Second

func (r *postgresIncidentRepo) Create(ctx context.Context, incident *models.Incident) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
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
	incident.CreatedAt = row.CreatedAt.Time
	incident.JiraSync = row.JiraSync
	return nil
}

func (r *postgresIncidentRepo) MarkSynced(ctx context.Context, id uuid.UUID, jiraKey string) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	return r.q.MarkIncidentSynced(ctx, db.MarkIncidentSyncedParams{
		ID:           id.String(),
		JiraIssueKey: pgtype.Text{String: jiraKey, Valid: true},
	})
}

func (r *postgresIncidentRepo) MarkSyncFailed(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	return r.q.MarkIncidentSyncFailed(ctx, id.String())
}

func (r *postgresIncidentRepo) ListPendingSync(ctx context.Context, maxRetries, limit int) ([]models.Incident, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	rows, err := r.q.ListPendingSync(ctx, db.ListPendingSyncParams{
		// sqlc nombra el field por la columna del WHERE (sync_retries < $1).
		// Lógicamente es "máximo de retries permitido".
		SyncRetries: int32(maxRetries),
		Limit:       int32(limit),
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
