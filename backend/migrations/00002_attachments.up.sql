-- Tabla de adjuntos de incidentes. La fila se conserva como audit trail
-- aunque el binario en disco se borre por retención (ver retention_worker):
-- en ese caso purged_at queda no-NULL.
CREATE TABLE IF NOT EXISTS incident_attachments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id         UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,

    -- Filename original sanitizado (solo caracteres seguros, máx 100). NO se
    -- usa para escribir en disco; sirve para mostrar contexto en UI/JIRA.
    filename_original   TEXT NOT NULL,
    -- Filename real en disco: "<uuid>.<ext>". El UUID evita path traversal y
    -- colisiones entre subidas.
    filename_stored     TEXT NOT NULL,

    -- MIME del archivo TRAS el re-encode (lo que realmente está en disco).
    mime_type           TEXT NOT NULL,
    size_bytes          BIGINT NOT NULL,
    -- SHA256 del archivo guardado, para audit y dedupe potencial.
    sha256              TEXT NOT NULL,

    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Tracking de sync con JIRA. NULL = pendiente. Lo usa la tanda 3b.
    uploaded_to_jira_at TIMESTAMP WITH TIME ZONE,
    -- Tracking de retención. NULL = el archivo aún vive en disco.
    purged_at           TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_attachments_incident
    ON incident_attachments (incident_id);

-- Índice parcial para el worker de JIRA (tanda 3b): solo indexa filas
-- pendientes de subir y aún no purgadas.
CREATE INDEX IF NOT EXISTS idx_attachments_pending_jira
    ON incident_attachments (incident_id)
    WHERE uploaded_to_jira_at IS NULL AND purged_at IS NULL;
