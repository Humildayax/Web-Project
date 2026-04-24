package repository

import (
	"context"
	"encoding/json"
	"time"

	"security-portal/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type IncidentRepository interface {
	Create(ctx context.Context, incident *models.Incident) error
	MarkSynced(ctx context.Context, id, jiraKey string) error
	MarkSyncFailed(ctx context.Context, id string) error
	ListPendingSync(ctx context.Context, maxRetries, limit int) ([]models.Incident, error)
}

type postgresIncidentRepo struct {
	db *pgxpool.Pool
}

func NewIncidentRepository(db *pgxpool.Pool) IncidentRepository {
	return &postgresIncidentRepo{db: db}
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

	const q = `
		INSERT INTO incidents (title, description, author, jira_sync, metadata)
		VALUES ($1, $2, $3, FALSE, $4)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, q,
		incident.Title, incident.Description, incident.Author, metadataBytes,
	).Scan(&incident.ID, &incident.CreatedAt)
}

func (r *postgresIncidentRepo) MarkSynced(ctx context.Context, id, jiraKey string) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	const q = `
		UPDATE incidents
		SET jira_sync = TRUE,
		    jira_issue_key = $2,
		    last_sync_attempt = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, q, id, jiraKey)
	return err
}

func (r *postgresIncidentRepo) MarkSyncFailed(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	const q = `
		UPDATE incidents
		SET sync_retries = sync_retries + 1,
		    last_sync_attempt = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, q, id)
	return err
}

func (r *postgresIncidentRepo) ListPendingSync(ctx context.Context, maxRetries, limit int) ([]models.Incident, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	const q = `
		SELECT id, title, description, author, created_at
		FROM incidents
		WHERE jira_sync = FALSE
		  AND sync_retries < $1
		ORDER BY created_at ASC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, q, maxRetries, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Incident
	for rows.Next() {
		var inc models.Incident
		if err := rows.Scan(&inc.ID, &inc.Title, &inc.Description, &inc.Author, &inc.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}
