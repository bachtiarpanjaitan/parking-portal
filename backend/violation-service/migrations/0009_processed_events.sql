-- =============================================================================
-- 0009 - processed_events
-- =============================================================================
-- Idempotency table for the Notification Worker. Stores event_id of every
-- event that has been successfully processed so we don't double-log on
-- re-delivery.
-- =============================================================================

CREATE TABLE IF NOT EXISTS processed_events (
    event_id     UUID        PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
