-- =============================================================================
-- 0006 - invoices
-- =============================================================================
-- See .ai/DATABASE_MAPPING.md → invoices
-- One invoice per violation (UNIQUE on violation_id).
-- Status transitions: PENDING → PAID, PENDING → FAILED, FAILED → PAID.
-- PAID is terminal.
-- =============================================================================

CREATE TABLE IF NOT EXISTS invoices (
    id           UUID          PRIMARY KEY,
    violation_id UUID          NOT NULL UNIQUE REFERENCES violations(id) ON DELETE CASCADE,
    amount       NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    status       VARCHAR(20)   NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PAID', 'FAILED', 'CANCELLED')),
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_invoices_status
    ON invoices (status);

CREATE INDEX IF NOT EXISTS idx_invoices_violation_id
    ON invoices (violation_id);
