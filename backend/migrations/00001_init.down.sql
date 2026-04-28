DROP INDEX IF EXISTS idx_incidents_pending_sync;
DROP TABLE IF EXISTS incidents;
-- No droppeamos la extensión pgcrypto: otras apps en la misma DB pueden usarla.
