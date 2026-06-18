-- =============================================================================
-- 0004 - fine_rule_details
-- =============================================================================
-- See .ai/DATABASE_MAPPING.md → fine_rule_details
-- One row per (rule_version_id, violation_type). UNIQUE constraint prevents
-- duplicate rules in a single version.
-- =============================================================================

CREATE TABLE IF NOT EXISTS fine_rule_details (
    id              UUID          PRIMARY KEY,
    rule_version_id UUID          NOT NULL REFERENCES fine_rule_versions(id) ON DELETE CASCADE,
    violation_type  VARCHAR(50)   NOT NULL
        CHECK (violation_type IN ('expired_meter', 'no_parking_zone', 'blocking_hydrant', 'disabled_spot')),
    base_amount     NUMERIC(12,2) NOT NULL CHECK (base_amount > 0),
    day_multiplier  NUMERIC(3,2)  NOT NULL DEFAULT 1.0 CHECK (day_multiplier > 0),
    night_multiplier NUMERIC(3,2) NOT NULL DEFAULT 1.5 CHECK (night_multiplier > 0),
    repeat_0        NUMERIC(3,2)  NOT NULL DEFAULT 1.0 CHECK (repeat_0 > 0),
    repeat_1        NUMERIC(3,2)  NOT NULL DEFAULT 1.5 CHECK (repeat_1 > 0),
    repeat_2_plus   NUMERIC(3,2)  NOT NULL DEFAULT 2.0 CHECK (repeat_2_plus > 0),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (rule_version_id, violation_type)
);

CREATE INDEX IF NOT EXISTS idx_fine_rule_details_version
    ON fine_rule_details (rule_version_id);
