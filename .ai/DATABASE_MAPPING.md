# Database Mapping

> **Naming convention:** `snake_case`, UUID primary keys, UTC timestamps.
> All tables have `created_at` and `updated_at` (set by application layer).
> Status fields are stored as `varchar` but constrained by application enum
> (see GLOSSARY.md for allowed values).

---

## users

| Field | Type | Notes |
|---|---|---|
| id | uuid | PK |
| name | varchar(255) | not null |
| email | varchar(255) | unique, not null |
| role | varchar(20) | enum: `OFFICER`, `MEMBER` |
| password_hash | varchar(255) | bcrypt hash; nullable for backward compat |
| created_at | timestamp | not null, default `now()` |
| updated_at | timestamp | not null |

Auth: **password-based with bcrypt** (see ADR-006). The hash is never sent
to clients. The seeder sets `password123` for the 3 demo users.

---

## fine_rule_versions

| Field | Type | Notes |
|---|---|---|
| id | uuid | PK |
| version_number | integer | not null, unique |
| is_active | boolean | not null, default `false` |
| published_at | timestamp | not null |
| created_by | uuid | FK → users.id (officer who published) |
| created_at | timestamp | not null |
| updated_at | timestamp | not null |

Constraint: at most one row with `is_active = true` (enforced in service layer
+ partial unique index in DB).

---

## fine_rule_details

| Field | Type | Notes |
|---|---|---|
| id | uuid | PK |
| rule_version_id | uuid | FK → fine_rule_versions.id |
| violation_type | varchar(50) | enum: `expired_meter`, `no_parking_zone`, `blocking_hydrant`, `disabled_spot` |
| base_amount | numeric(12,2) | not null, in IDR |
| day_multiplier | numeric(3,2) | not null, default `1.0` |
| night_multiplier | numeric(3,2) | not null, default `1.5` |
| repeat_0 | numeric(3,2) | not null, default `1.0` (0 prior unpaid) |
| repeat_1 | numeric(3,2) | not null, default `1.5` (1 prior unpaid) |
| repeat_2_plus | numeric(3,2) | not null, default `2.0` (2+ prior unpaid) |
| created_at | timestamp | not null |
| updated_at | timestamp | not null |

Constraint: `UNIQUE (rule_version_id, violation_type)` — one rule per violation type
per version.

---

## violations

| Field | Type | Notes |
|---|---|---|
| id | uuid | PK |
| member_id | uuid | FK → users.id (the member whose plate is involved) |
| rule_version_id | uuid | FK → fine_rule_versions.id (the version active at creation time) |
| license_plate | varchar(20) | not null, indexed |
| violation_type | varchar(50) | not null, enum (see above) |
| location | varchar(255) | not null |
| **violation_timestamp** | **timestamp** | **not null, UTC** (used for time multiplier calc) |
| photo_url | text | not null (local path, see PHOTO_STORAGE.md) |
| fine_amount | numeric(12,2) | not null, calculated at creation |
| calculation_snapshot | jsonb | not null, immutable (see DOMAIN_MODELS.md) |
| created_at | timestamp | not null |
| updated_at | timestamp | not null |

⚠️ `violation_timestamp` is the time the violation **occurred**, distinct from
`created_at` (when officer submitted the record). Required for fine calculation.

---

## invoices

| Field | Type | Notes |
|---|---|---|
| id | uuid | PK |
| violation_id | uuid | FK → violations.id, unique (one-to-one) |
| amount | numeric(12,2) | not null, equal to `violations.fine_amount`, immutable |
| status | varchar(20) | not null, enum: `PENDING`, `PAID`, `FAILED`, `CANCELLED` |
| created_at | timestamp | not null |
| updated_at | timestamp | not null |

Status transitions:
- `PENDING` → `PAID` (on payment success)
- `PENDING` → `FAILED` (on payment failure; invoice stays open, member can retry)
- `PAID` is terminal

---

## payments

| Field | Type | Notes |
|---|---|---|
| id | uuid | PK |
| invoice_id | uuid | FK → invoices.id |
| amount | numeric(12,2) | **not null** — must equal `invoices.amount` (validated in service) |
| transaction_id | varchar(100) | not null, from mock provider |
| status | varchar(20) | not null, enum: `PAID`, `FAILED` |
| scenario | varchar(20) | not null, enum: `success`, `failed` (test-only input) |
| created_at | timestamp | not null |
| updated_at | timestamp | not null |

Note: `amount` is stored on every payment attempt for audit/replay purposes,
even though the mock provider does not echo it.

---

## notifications (optional, see NOTIFICATIONS.md)

| Field | Type | Notes |
|---|---|---|
| id | uuid | PK |
| user_id | uuid | nullable, FK → users.id |
| event_type | varchar(100) | not null |
| title | varchar(255) | not null |
| message | text | not null |
| created_at | timestamp | not null |

---

## processed_events (optional, see NOTIFICATIONS.md)

| Field | Type | Notes |
|---|---|---|
| event_id | uuid | PK |
| processed_at | timestamp | not null |

Used by Notification Worker for idempotency.

---

## Indexes (recommended)

```sql
CREATE UNIQUE INDEX idx_users_email ON users (email);
CREATE INDEX idx_violations_license_plate ON violations (license_plate);
CREATE INDEX idx_violations_member_id ON violations (member_id);
CREATE INDEX idx_violations_violation_timestamp ON violations (violation_timestamp);
CREATE UNIQUE INDEX idx_fine_rule_details_unique
  ON fine_rule_details (rule_version_id, violation_type);
CREATE UNIQUE INDEX idx_fine_rule_versions_active
  ON fine_rule_versions (is_active) WHERE is_active = true;
CREATE INDEX idx_invoices_violation_id ON invoices (violation_id);
CREATE INDEX idx_invoices_status ON invoices (status);
CREATE INDEX idx_payments_invoice_id ON payments (invoice_id);
```
