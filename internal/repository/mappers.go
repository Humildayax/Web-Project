package repository

import (
	"security-portal/internal/db"
	"security-portal/internal/models"
)

// toDomain convierte el modelo de persistencia (db.Incident, generado por sqlc
// con tipos pgtype.*) al modelo de dominio (models.Incident) que consumen
// services y handlers.
//
// Esta función es la frontera entre la capa de repositorio y el resto del
// sistema: cualquier divergencia futura entre los modelos (campos solo-DB,
// versiones, soft-delete, etc.) se absorbe acá sin tocar services.
func toDomain(row db.Incident) models.Incident {
	inc := models.Incident{
		ID:          row.ID,
		Title:       row.Title,
		Description: row.Description,
		Author:      row.Author,
		CreatedAt:   row.CreatedAt.Time, // pgtype.Timestamptz -> time.Time (NOT NULL en schema)
		JiraSync:    row.JiraSync,
	}
	if row.JiraIssueKey.Valid {
		inc.JiraIssueKey = row.JiraIssueKey.String
	}
	// La metadata se mantiene como []byte crudo a propósito: el repo no debería
	// imponer una forma. Si un consumer quiere leerla como objeto, json.Unmarshal
	// se hace en services o en el handler — no acá.
	if len(row.Metadata) > 0 {
		inc.Metadata = row.Metadata
	}
	return inc
}
