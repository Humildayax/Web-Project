-- name: CreateAttachment :one
INSERT INTO incident_attachments (
    incident_id, filename_original, filename_stored,
    mime_type, size_bytes, sha256
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, incident_id, filename_original, filename_stored,
          mime_type, size_bytes, sha256, created_at,
          uploaded_to_jira_at, purged_at;

-- name: ListAttachmentsByIncident :many
SELECT id, incident_id, filename_original, filename_stored,
       mime_type, size_bytes, sha256, created_at,
       uploaded_to_jira_at, purged_at
FROM incident_attachments
WHERE incident_id = $1
ORDER BY created_at ASC;

-- name: ListAttachmentsPendingJiraUpload :many
-- Adjuntos cuyos incidentes ya están sincronizados a JIRA (tienen issue
-- key) pero el archivo todavía no se subió al ticket. Excluye los purgados.
-- Devuelve también jira_issue_key para que el worker sepa a qué issue
-- subirlos sin un round-trip extra.
SELECT a.id, a.incident_id, a.filename_original, a.filename_stored,
       a.mime_type, a.size_bytes, a.sha256, a.created_at,
       a.uploaded_to_jira_at, a.purged_at,
       i.jira_issue_key
FROM incident_attachments a
JOIN incidents i ON i.id = a.incident_id
WHERE a.uploaded_to_jira_at IS NULL
  AND a.purged_at IS NULL
  AND i.jira_sync = TRUE
  AND i.jira_issue_key IS NOT NULL
ORDER BY a.created_at ASC
LIMIT $1;

-- name: MarkAttachmentUploadedToJira :exec
UPDATE incident_attachments
SET uploaded_to_jira_at = NOW()
WHERE id = $1;

-- name: ListAttachmentsPurgeable :many
-- Adjuntos viejos cuyo binario aún vive en disco. La fila se conserva
-- como audit trail (cuántos había, qué hashes); solo borramos el blob.
SELECT id, incident_id, filename_original, filename_stored,
       mime_type, size_bytes, sha256, created_at,
       uploaded_to_jira_at, purged_at
FROM incident_attachments
WHERE purged_at IS NULL
  AND created_at < $1
ORDER BY created_at ASC
LIMIT $2;

-- name: MarkAttachmentPurged :exec
UPDATE incident_attachments
SET purged_at = NOW()
WHERE id = $1;
