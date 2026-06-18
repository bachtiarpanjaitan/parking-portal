-- =============================================================================
-- 0007 - payments
-- =============================================================================
-- See .ai/DATABASE_MAPPING.md → payments
-- Each payment attempt creates a new row. Invoice status is updated to match
-- the most recent successful payment.
-- =============================================================================

CREATE TABLE IF NOT EXISTS payments (
    id              UUID          PRIMARY KEY,
    invoice_id      UUID          NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    amount          NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    transaction_id  VARCHAR(100)  NOT NULL,
    status          VARCHAR(20)   NOT NULL
        CHECK (status IN ('PAID', 'FAILED')),
    scenario        VARCHAR(20)   NOT NULL
        CHECK (scenario IN ('success', 'failed')),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payments_invoice_id
    ON payments (invoice_id);

CREATE INDEX IF NOT EXISTS idx_payments_created_at
    ON payments (created_at DESC);
