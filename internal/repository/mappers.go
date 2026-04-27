package repository

import (
	"encoding/json"
	"fmt"

	"security-portal/internal/db"
	"security-portal/internal/models"

	"github.com/google/uuid"
)

// toDomain convierte el modelo de persistencia (db.Incident, generado por sqlc
// con id como string y tipos pgtype.*) al modelo de dominio (models.Incident).
//
// Esta función es la frontera entre la capa de repositorio y el resto del
// sistema: cualquier divergencia entre los modelos (campos solo-DB,
// versiones, soft-delete, etc.) se absorbe acá sin tocar services.
func toDomain(row db.Incident) (models.Incident, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return models.Incident{}, fmt.Errorf("uuid inválido %q en DB: %w", row.ID, err)
	}

	inc := models.Incident{
		ID:          id,
		Title:       row.Title,
		Description: row.Description,
		Author:      row.Author,
		CreatedAt:   row.CreatedAt.Time, // pgtype.Timestamptz -> time.Time (NOT NULL en schema)
		JiraSync:    row.JiraSync,
	}
	if row.JiraIssueKey.Valid {
		inc.JiraIssueKey = row.JiraIssueKey.String
	}
	if len(row.Metadata) > 0 {
		var m models.IncidentMetadata
		if err := json.Unmarshal(row.Metadata, &m); err != nil {
			return inc, fmt.Errorf("metadata unmarshal (id=%s): %w", id, err)
		}
		inc.Metadata = &m
	}
	return inc, nil
}
