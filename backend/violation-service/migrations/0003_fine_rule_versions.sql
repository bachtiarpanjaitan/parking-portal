-- =============================================================================
-- 0003 - fine_rule_versions
-- =============================================================================
-- See .ai/DATABASE_MAPPING.md → fine_rule_versions
-- Enforces "at most one active version" via partial unique index.
-- =============================================================================

CREATE TABLE IF NOT EXISTS fine_rule_versions (
    id              UUID         PRIMARY KEY,
    version_number  INTEGER      NOT NULL UNIQUE,
    is_active       BOOLEAN      NOT NULL DEFAULT false,
    published_at    TIMESTAMPTZ  NOT NULL,
    created_by      UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- At most one active version.
CREATE UNIQUE INDEX IF NOT EXISTS idx_fine_rule_versions_active
    ON fine_rule_versions (is_active) WHERE is_active = true;
