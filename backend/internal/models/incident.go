package models

import (
	"time"

	"github.com/google/uuid"
)

// IncidentMetadata es el contexto operativo del reporte (audit trail).
// Tipado fuerte a propósito: los tres campos son fijos y conocidos.
// Si alguna vez aparecen campos ad-hoc, se agrega un Extra map[string]any.
type IncidentMetadata struct {
	SourceIP   string    `json:"source_ip"`
	UserAgent  string    `json:"user_agent"`
	ReceivedAt time.Time `json:"received_at"`
}

type Incident struct {
	ID           uuid.UUID         `json:"id"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Author       string            `json:"author"`
	CreatedAt    time.Time         `json:"created_at"`
	JiraSync     bool              `json:"jira_sync"`
	JiraIssueKey string            `json:"jira_issue_key,omitempty"`
	Metadata     *IncidentMetadata `json:"metadata,omitempty"`
}
