package models

import (
	"time"

	"github.com/google/uuid"
)

// Attachment es un adjunto de un incidente, ya validado y guardado.
// Los datos del binario están en el storage; acá viajan solo metadatos.
type Attachment struct {
	ID               uuid.UUID `json:"id"`
	IncidentID       uuid.UUID `json:"incident_id"`
	FilenameOriginal string    `json:"filename_original"`
	FilenameStored   string    `json:"filename_stored"`
	MimeType         string    `json:"mime_type"`
	SizeBytes        int64     `json:"size_bytes"`
	SHA256           string    `json:"sha256"`
	CreatedAt        time.Time `json:"created_at"`
	// Punteros para representar nullables sin acoplarse a pgtype en el dominio.
	UploadedToJiraAt *time.Time `json:"uploaded_to_jira_at,omitempty"`
	PurgedAt         *time.Time `json:"purged_at,omitempty"`
}
