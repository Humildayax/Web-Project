-- pgcrypto provee gen_random_uuid() (reemplaza al legado uuid-ossp).
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    author VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Resiliencia JIRA
    jira_sync BOOLEAN NOT NULL DEFAULT FALSE,
    jira_issue_key TEXT,
    sync_retries INTEGER NOT NULL DEFAULT 0,
    last_sync_attempt TIMESTAMP WITH TIME ZONE,

    metadata JSONB
);

-- Índice parcial: solo indexa filas pendientes de sincronizar.
CREATE INDEX IF NOT EXISTS idx_incidents_pending_sync
    ON incidents (created_at)
    WHERE jira_sync = FALSE;
