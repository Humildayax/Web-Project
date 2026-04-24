package models

import "time"

type Incident struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Author       string    `json:"author"`
	CreatedAt    time.Time `json:"created_at"`
	JiraSync     bool      `json:"jira_sync"`
	JiraIssueKey string    `json:"jira_issue_key,omitempty"`
	Metadata     any       `json:"metadata,omitempty"`
}
