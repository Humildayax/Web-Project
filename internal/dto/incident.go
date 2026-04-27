package dto

import (
	"strings"
	"time"

	"security-portal/internal/models"
)

// CreateIncidentRequest es la entrada del endpoint POST /api/incidents.
// Las reglas `validate` las interpreta go-playground/validator.
type CreateIncidentRequest struct {
	Title       string `json:"title"       validate:"required,min=3,max=255"`
	Description string `json:"description" validate:"required,min=5,max=10000"`
	Author      string `json:"author"      validate:"omitempty,max=100"`
}

// Normalize limpia espacios y aplica defaults antes de validar.
func (r *CreateIncidentRequest) Normalize() {
	r.Title = strings.TrimSpace(r.Title)
	r.Description = strings.TrimSpace(r.Description)
	r.Author = strings.TrimSpace(r.Author)
}

// ToModel mapea el DTO al modelo de dominio que consumen services y repository.
func (r CreateIncidentRequest) ToModel() *models.Incident {
	author := r.Author
	if author == "" {
		author = "anonymous"
	}
	return &models.Incident{
		Title:       r.Title,
		Description: r.Description,
		Author:      author,
	}
}

// IncidentResponse es la representación pública del incidente.
// No expone metadata (IP, user-agent), retries ni last_sync_attempt.
type IncidentResponse struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Author       string    `json:"author"`
	CreatedAt    time.Time `json:"created_at"`
	JiraSync     bool      `json:"jira_sync"`
	JiraIssueKey string    `json:"jira_issue_key,omitempty"`
}

func IncidentResponseFromModel(m *models.Incident) IncidentResponse {
	return IncidentResponse{
		ID:           m.ID,
		Title:        m.Title,
		Description:  m.Description,
		Author:       m.Author,
		CreatedAt:    m.CreatedAt,
		JiraSync:     m.JiraSync,
		JiraIssueKey: m.JiraIssueKey,
	}
}
