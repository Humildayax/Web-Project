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

-- name: ClaimPendingSync :many
-- Reclama hasta $3 incidentes pendientes para que esta réplica los procese.
--
-- - FOR UPDATE SKIP LOCKED: si otra réplica ya lockó la fila, la saltamos.
--   Eso evita que dos workers procesen el mismo incidente y manden el mismo
--   ticket a JIRA (la API no es idempotente).
-- - Backoff exponencial: solo entran filas cuyo último intento fue antes de
--   `base_backoff * 2^sync_retries` (o que nunca se intentaron). Si JIRA
--   está caído no martillamos cada RetryInterval; cada incidente espera
--   más en cada fallo sucesivo.
-- - El UPDATE inmediato de last_sync_attempt = NOW() actúa como "claim
--   blando": si crasheamos antes de poder llamar a MarkSynced/MarkSyncFailed,
--   el incidente queda visible recién después del backoff, no en el
--   próximo tick.
WITH claimed AS (
    SELECT id FROM incidents
    WHERE jira_sync = FALSE
      AND sync_retries < $1
      AND (last_sync_attempt IS NULL
           OR last_sync_attempt < NOW() - make_interval(secs => $2) * POWER(2::numeric, sync_retries))
    ORDER BY created_at ASC
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
UPDATE incidents
SET last_sync_attempt = NOW()
FROM claimed
WHERE incidents.id = claimed.id
RETURNING incidents.id, incidents.title, incidents.description, incidents.author,
          incidents.created_at, incidents.jira_sync, incidents.jira_issue_key,
          incidents.sync_retries, incidents.last_sync_attempt, incidents.metadata;

-- name: PurgeOldMetadata :execrows
-- Borra la metadata (IP, user-agent, received_at) de incidentes anteriores al
-- cutoff dado. Conserva la fila: el incidente sigue siendo consultable, lo que
-- desaparece es solo la PII. Devuelve la cantidad de filas afectadas.
UPDATE incidents
SET metadata = NULL
WHERE metadata IS NOT NULL
  AND created_at < $1;
