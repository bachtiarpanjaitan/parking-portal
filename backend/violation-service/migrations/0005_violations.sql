-- =============================================================================
-- 0005 - violations
-- =============================================================================
-- See .ai/DATABASE_MAPPING.md → violations
-- violation_timestamp is the time the violation OCCURRED (not created_at).
-- fine_amount + rule_version_id + calculation_snapshot are IMMUTABLE.
-- =============================================================================

CREATE TABLE IF NOT EXISTS violations (
    id                  UUID         PRIMARY KEY,
    member_id           UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    rule_version_id     UUID         NOT NULL REFERENCES fine_rule_versions(id) ON DELETE RESTRICT,
    license_plate       VARCHAR(20)  NOT NULL,
    violation_type      VARCHAR(50)  NOT NULL
        CHECK (violation_type IN ('expired_meter', 'no_parking_zone', 'blocking_hydrant', 'disabled_spot')),
    location            VARCHAR(255) NOT NULL,
    violation_timestamp TIMESTAMPTZ  NOT NULL,
    photo_url           TEXT         NOT NULL,
    fine_amount         NUMERIC(12,2) NOT NULL CHECK (fine_amount > 0),
    calculation_snapshot JSONB        NOT NULL,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_violations_license_plate
    ON violations (license_plate);

CREATE INDEX IF NOT EXISTS idx_violations_member_id
    ON violations (member_id);

CREATE INDEX IF NOT EXISTS idx_violations_violation_timestamp
    ON violations (violation_timestamp);

-- Composite index for the repeat-multiplier query:
-- "count unpaid violations for plate in last 90 days"
CREATE INDEX IF NOT EXISTS idx_violations_plate_ts
    ON violations (license_plate, violation_timestamp);

-- Range index for history pagination
CREATE INDEX IF NOT EXISTS idx_violations_created_at
    ON violations (created_at DESC);
