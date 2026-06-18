-- =============================================================================
-- 0002 - users
-- =============================================================================
-- See .ai/DATABASE_MAPPING.md → users
-- Mocked auth: email-only, no password (ADR-006).
-- =============================================================================

CREATE TABLE IF NOT EXISTS users (
    id          UUID         PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    email       VARCHAR(255) NOT NULL UNIQUE,
    role        VARCHAR(20)  NOT NULL CHECK (role IN ('OFFICER', 'MEMBER')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_users_role  ON users (role);
