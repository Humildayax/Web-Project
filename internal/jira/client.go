package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"security-portal/internal/config"
	"security-portal/internal/models"
)

// Client abstrae la integración con JIRA (u otro tracker).
type Client interface {
	CreateIssue(ctx context.Context, incident *models.Incident) (IssueRef, error)
}

// IssueRef es lo que devuelve el tracker tras crear el ticket.
type IssueRef struct {
	Key string
	URL string
}

// -------- Implementación HTTP (JIRA Cloud REST API v3) --------

type httpClient struct {
	cfg  config.JiraConfig
	http *http.Client
}

func NewHTTPClient(cfg config.JiraConfig) Client {
	return &httpClient{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.HTTPTimeout},
	}
}

type createIssueReq struct {
	Fields issueFields `json:"fields"`
}

type issueFields struct {
	Project     projectRef   `json:"project"`
	Summary     string       `json:"summary"`
	IssueType   issueTypeRef `json:"issuetype"`
	Description adfDoc       `json:"description"`
}

type projectRef struct {
	Key string `json:"key"`
}

type issueTypeRef struct {
	Name string `json:"name"`
}

// Atlassian Document Format (ADF) mínimo: un párrafo con texto plano.
type adfDoc struct {
	Type    string      `json:"type"`
	Version int         `json:"version"`
	Content []adfBlock  `json:"content"`
}

type adfBlock struct {
	Type    string      `json:"type"`
	Content []adfInline `json:"content"`
}

type adfInline struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type createIssueResp struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

func (c *httpClient) CreateIssue(ctx context.Context, incident *models.Incident) (IssueRef, error) {
	body, err := json.Marshal(createIssueReq{
		Fields: issueFields{
			Project:   projectRef{Key: c.cfg.ProjectKey},
			Summary:   truncate(incident.Title, 255),
			IssueType: issueTypeRef{Name: c.cfg.IssueType},
			Description: adfDoc{
				Type:    "doc",
				Version: 1,
				Content: []adfBlock{{
					Type: "paragraph",
					Content: []adfInline{{
						Type: "text",
						Text: fmt.Sprintf("Reportado por: %s\n\n%s", incident.Author, incident.Description),
					}},
				}},
			},
		},
	})
	if err != nil {
		return IssueRef{}, fmt.Errorf("marshal: %w", err)
	}

	endpoint := fmt.Sprintf("%s/rest/api/3/issue", c.cfg.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return IssueRef{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.cfg.Email, c.cfg.APIToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return IssueRef{}, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return IssueRef{}, fmt.Errorf("jira status %d: %s", resp.StatusCode, string(errBody))
	}

	var parsed createIssueResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return IssueRef{}, fmt.Errorf("decode: %w", err)
	}

	return IssueRef{
		Key: parsed.Key,
		URL: fmt.Sprintf("%s/browse/%s", c.cfg.BaseURL, parsed.Key),
	}, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// -------- No-op (modo desarrollo sin credenciales reales) --------

type noopClient struct{}

func NewNoopClient() Client { return &noopClient{} }

func (noopClient) CreateIssue(_ context.Context, incident *models.Incident) (IssueRef, error) {
	slog.Info("jira noop: marcando como sincronizado sin llamar a la API", "incident_id", incident.ID)
	return IssueRef{Key: "NOOP-LOCAL"}, nil
}
