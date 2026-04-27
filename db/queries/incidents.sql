-- name: CreateIncident :one
INSERT INTO incidents (title, description, author, jira_sync, metadata)
VALUES ($1, $2, $3, FALSE, $4)
RETURNING id, title, description, author, created_at, jira_sync, jira_issue_key, sync_retries, last_sync_attempt, metadata;

-- name: MarkIncidentSynced :exec
UPDATE incidents
SET jira_sync = TRUE,
    jira_issue_key = $2,
    last_sync_attempt = NOW()
WHERE id = $1;

-- name: MarkIncidentSyncFailed :exec
UPDATE incidents
SET sync_retries = sync_retries + 1,
    last_sync_attempt = NOW()
WHERE id = $1;

-- name: ListPendingSync :many
SELECT id, title, description, author, created_at, jira_sync, jira_issue_key, sync_retries, last_sync_attempt, metadata
FROM incidents
WHERE jira_sync = FALSE
  AND sync_retries < $1
ORDER BY created_at ASC
LIMIT $2;
