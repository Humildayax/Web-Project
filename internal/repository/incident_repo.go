package repository

import (
	"context"
	"encoding/json"
	"time"

	"security-portal/internal/db"
	"security-portal/internal/models"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IncidentRepository es la interfaz que ven services. Trabaja siempre con
// models.Incident (dominio); los tipos generados por sqlc (db.Incident) no
// salen de este paquete.
type IncidentRepository interface {
	Create(ctx context.Context, incident *models.Incident) error
	MarkSynced(ctx context.Context, id, jiraKey string) error
	MarkSyncFailed(ctx context.Context, id string) error
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
			return err
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

	// El servicio le pasó un puntero: solo actualizamos lo que la DB generó.
	// row.CreatedAt es pgtype.Timestamptz; para el dominio queremos time.Time.
	incident.ID = row.ID
	incident.CreatedAt = row.CreatedAt.Time
	incident.JiraSync = row.JiraSync
	return nil
}

func (r *postgresIncidentRepo) MarkSynced(ctx context.Context, id, jiraKey string) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	return r.q.MarkIncidentSynced(ctx, db.MarkIncidentSyncedParams{
		ID:           id,
		JiraIssueKey: pgtype.Text{String: jiraKey, Valid: true},
	})
}

func (r *postgresIncidentRepo) MarkSyncFailed(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	return r.q.MarkIncidentSyncFailed(ctx, id)
}

func (r *postgresIncidentRepo) ListPendingSync(ctx context.Context, maxRetries, limit int) ([]models.Incident, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	rows, err := r.q.ListPendingSync(ctx, db.ListPendingSyncParams{
		// sqlc nombra el field por la columna del WHERE (sync_retries < $1).
		// El significado lógico es "máximo de retries permitido".
		SyncRetries: int32(maxRetries),
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]models.Incident, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}
