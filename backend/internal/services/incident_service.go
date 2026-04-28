package services

import (
	"context"
	"fmt"
	"log/slog"

	"security-portal/internal/jira"
	"security-portal/internal/models"
	"security-portal/internal/repository"
)

type IncidentService struct {
	repo repository.IncidentRepository
	jira jira.Client
}

func NewIncidentService(repo repository.IncidentRepository, jiraClient jira.Client) *IncidentService {
	return &IncidentService{repo: repo, jira: jiraClient}
}

// ProcessNewIncident sigue la política DB-first:
//  1. Guarda en Postgres (fuente de verdad) con jira_sync=false.
//  2. Intenta crear el ticket en JIRA.
//  3. Si JIRA falla, NO propaga el error al cliente: el incidente ya está
//     a salvo en la DB y el worker lo reintentará.
func (s *IncidentService) ProcessNewIncident(ctx context.Context, incident *models.Incident) error {
	if err := s.repo.Create(ctx, incident); err != nil {
		return fmt.Errorf("guardando en BD local: %w", err)
	}

	ref, err := s.jira.CreateIssue(ctx, incident)
	if err != nil {
		slog.Warn("sync inicial con jira falló; queda pendiente para retry",
			"incident_id", incident.ID, "err", err)
		return nil
	}

	if err := s.repo.MarkSynced(ctx, incident.ID, ref.Key); err != nil {
		slog.Error("jira creó el ticket pero el update local falló",
			"incident_id", incident.ID, "jira_key", ref.Key, "err", err)
		return nil
	}

	incident.JiraSync = true
	incident.JiraIssueKey = ref.Key
	return nil
}
