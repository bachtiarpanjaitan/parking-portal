-- =============================================================================
-- 0001 - schema_migrations
-- =============================================================================
-- Tracks which migrations have been applied. Inserted before any other
-- migration so the migrator can detect "fresh DB" vs "already migrated".
-- =============================================================================

CREATE TABLE IF NOT EXISTS schema_migrations (
    version     VARCHAR(32) PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
