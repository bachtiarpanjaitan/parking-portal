-- =============================================================================
-- 0008 - notifications (optional, see .ai/NOTIFICATIONS.md)
-- =============================================================================
-- Written by the Notification Worker. Optional - the worker can also be
-- configured to log only.
-- =============================================================================

CREATE TABLE IF NOT EXISTS notifications (
    id          UUID         PRIMARY KEY,
    user_id     UUID         NULL REFERENCES users(id) ON DELETE SET NULL,
    event_type  VARCHAR(100) NOT NULL,
    title       VARCHAR(255) NOT NULL,
    message     TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id
    ON notifications (user_id);

CREATE INDEX IF NOT EXISTS idx_notifications_created_at
    ON notifications (created_at DESC);
