-- =============================================================================
-- 0010 - add password_hash to users
-- =============================================================================
-- Adds password-based authentication. Replaces the "email-only mock" from
-- ADR-006 with real bcrypt-hashed credentials. The column is nullable for
-- backward compatibility but every active user must have one (enforced in
-- the auth service).
-- =============================================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);

-- Seed passwords are set by the seeder (cmd/seed) for the 3 demo users.
-- Default password for the demo is "password123" (bcrypt-hashed).
