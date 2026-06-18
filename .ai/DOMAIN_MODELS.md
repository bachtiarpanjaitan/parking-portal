# Domain Models

> **Source of truth** for the application's core entities.
> Fields mirror `DATABASE_MAPPING.md`. Behavior is implemented in the service layer.
> All IDs are UUIDs, all timestamps are UTC.

---

# User

**Purpose:** System actor.

**Roles:** `OFFICER`, `MEMBER`

**Properties:**
- `id`
- `name`
- `email`
- `role`
- `created_at`, `updated_at`

**Password handling:**
- The user-facing `User` DTO **does not** carry a password hash.
- The `UserWithPassword` DTO (internal, used by the auth service only) adds
  `PasswordHash string` and is **never** sent over the API.
- Passwords are stored as bcrypt hashes in `users.password_hash` (see ADR-006).

**Behavior:** none (read-only entity for this assignment)

---

# FineRuleVersion

**Purpose:** Represents a published ruleset.

**Properties:**
- `id` (uuid)
- `version_number` (int, unique)
- `is_active` (bool)
- `published_at` (timestamp)
- `created_by` (uuid, FK → User) — officer who published
- `created_at`, `updated_at`

**Behavior:**
- `Publish()` — set `is_active = true`, deactivate all others, set `published_at = now()`
- `Deactivate()` — set `is_active = false`

Invariants:
- Exactly one version is active at any time.

---

# FineRuleDetail

**Purpose:** Stores configurable calculation values for one (version, violation_type) pair.

**Properties:**
- `id` (uuid)
- `rule_version_id` (uuid, FK)
- `violation_type` (`expired_meter` | `no_parking_zone` | `blocking_hydrant` | `disabled_spot`)
- `base_amount` (decimal, IDR)
- `day_multiplier` (decimal) — multiplier applied during 06:00–21:59 local
- `night_multiplier` (decimal) — multiplier applied during 22:00–05:59 local
- `repeat_0` (decimal) — multiplier when 0 prior unpaid violations in 90 days
- `repeat_1` (decimal) — multiplier when 1 prior unpaid violation in 90 days
- `repeat_2_plus` (decimal) — multiplier when 2+ prior unpaid violations in 90 days
- `created_at`, `updated_at`

> **Note:** The 3 separate repeat fields (`repeat_0`, `repeat_1`, `repeat_2_plus`)
> are stored as discrete columns in the DB (see DATABASE_MAPPING.md). The service
> layer exposes a unified `GetRepeatMultiplier(unpaidCount int) decimal` that picks
> the right column based on the count.

**Behavior:**
- `GetBaseAmount() decimal`
- `GetTimeMultiplier(localTime time.Time) decimal`
- `GetRepeatMultiplier(unpaidCount int) decimal`

---

# Violation

**Purpose:** Represents a parking violation recorded by an officer.

**Properties:**
- `id` (uuid)
- `member_id` (uuid, FK → User) — the member whose plate is involved
- `rule_version_id` (uuid, FK → FineRuleVersion) — frozen at creation
- `license_plate` (string)
- `violation_type` (enum)
- `location` (string)
- **`violation_timestamp`** (timestamp, UTC) — the actual time of the violation
  (distinct from `created_at` which is when officer submitted the record)
- `photo_url` (string) — local path from `PHOTO_STORAGE.md`
- `fine_amount` (decimal) — calculated and frozen at creation
- `calculation_snapshot` (CalculationSnapshot) — immutable breakdown
- `created_at`, `updated_at`

**Behavior:**
- `CalculateFine(rule FineRuleDetail, unpaidCount int) decimal` — pure function
- `CreateInvoice() Invoice` — creates invoice with `amount = fine_amount`

Invariants:
- `rule_version_id`, `fine_amount`, and `calculation_snapshot` are **immutable**
  after creation. They never change, even when a new rule version is published.

---

# CalculationSnapshot

**Purpose:** Immutable breakdown of how `fine_amount` was computed.

**Properties (JSONB):**
```json
{
  "rule_version_id": "uuid",
  "rule_version_number": 1,
  "violation_type": "no_parking_zone",
  "base_amount": 150000,
  "time_multiplier": 1.5,
  "time_window": "NIGHT",
  "repeat_multiplier": 1.0,
  "prior_unpaid_count": 0,
  "calculated_fine": 225000,
  "calculated_at": "2026-01-01T10:00:00Z"
}
```

**Behavior:** none — fully immutable.

> Frontend can render this breakdown in the history view for transparency.

---

# Invoice

**Purpose:** Bill generated from a violation.

**Properties:**
- `id` (uuid)
- `violation_id` (uuid, FK, unique — one-to-one with Violation)
- `amount` (decimal) — immutable, equal to `violation.fine_amount`
- `status` (`PENDING` | `PAID` | `FAILED` | `CANCELLED`)
- `created_at`, `updated_at`

**Behavior:**
- `MarkPaid()` — status `PENDING` → `PAID`
- `MarkFailed()` — status `PENDING` → `FAILED` (stays payable, member can retry)
- `MarkCancelled()` — for admin use, not in MVP

State transitions:
```
PENDING ──pay success──> PAID   (terminal)
PENDING ──pay failed───> FAILED
FAILED  ──pay success──> PAID   (after retry)
PAID  ────────────────> (terminal)
```

---

# Payment

**Purpose:** One attempt to pay an invoice via the (mock) payment provider.

**Properties:**
- `id` (uuid)
- `invoice_id` (uuid, FK → Invoice)
- `amount` (decimal) — must equal `invoice.amount` (validated before insert)
- `transaction_id` (string) — from mock provider
- `status` (`PAID` | `FAILED`)
- `scenario` (`success` | `failed`) — test-only input echoed back
- `created_at`, `updated_at`

**Behavior:**
- `Process(scenario string) (status, transaction_id)` — delegates to `PaymentService.charge()`

Note: each payment attempt creates a new row. Invoice status is updated to match
the **most recent** successful payment.

---

# Notification (optional, see NOTIFICATIONS.md)

**Properties:**
- `id`, `user_id` (nullable), `event_type`, `title`, `message`, `created_at`

---

# ProcessedEvent (optional, see NOTIFICATIONS.md)

**Properties:**
- `event_id` (PK), `processed_at`

Used by Notification Worker for idempotency.
