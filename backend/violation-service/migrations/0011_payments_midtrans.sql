-- =============================================================================
-- 0011 - payments: add Midtrans columns + change status enum
-- =============================================================================
-- Replaces the old "scenario" mock with the real Midtrans Snap flow.
-- The payment_id UUID is unchanged; existing rows will get NULLs for the
-- new columns (which is fine — they represent pre-Midtrans mock payments).
-- =============================================================================

-- Drop old scenario column (mock provider no longer used).
ALTER TABLE payments DROP COLUMN IF EXISTS scenario;

-- Add Midtrans columns
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS payment_method                  VARCHAR(20)    NULL,
    ADD COLUMN IF NOT EXISTS midtrans_order_id               VARCHAR(100)   UNIQUE,
    ADD COLUMN IF NOT EXISTS midtrans_snap_token            TEXT           NULL,
    ADD COLUMN IF NOT EXISTS midtrans_transaction_id        VARCHAR(100)   NULL,
    ADD COLUMN IF NOT EXISTS midtrans_transaction_status    VARCHAR(50)    NULL,
    ADD COLUMN IF NOT EXISTS midraw_response                JSONB          NULL;

-- Replace the status CHECK constraint with a wider enum that includes
-- PENDING (Snap transaction created, not yet settled), CANCELLED, EXPIRED.
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('PENDING', 'PAID', 'FAILED', 'CANCELLED', 'EXPIRED'));

-- New CHECK for payment_method (only when set)
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_payment_method_check;
ALTER TABLE payments ADD CONSTRAINT payments_payment_method_check
    CHECK (payment_method IS NULL OR payment_method IN
        ('gopay', 'qris', 'shopeepay', 'dana', 'ovo', 'bca_va', 'bni_va',
         'credit_card', 'bank_transfer', 'echannel', 'other'));

CREATE INDEX IF NOT EXISTS idx_payments_midtrans_order_id
    ON payments (midtrans_order_id);
CREATE INDEX IF NOT EXISTS idx_payments_status_created
    ON payments (status, created_at DESC);
