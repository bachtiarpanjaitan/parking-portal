# Modules

> Per-module responsibilities and per-role access matrix.
> See `SERVICE_BOUNDARIES.md` for ownership of tables and "must not" rules.

---

# Authentication Module

**Responsibilities:**
- `POST /auth/login` — password + bcrypt login (ADR-006)
- Look up user by email, compare bcrypt hash
- JWT generation on success
- `role` extraction from JWT
- Gateway-side JWT validation middleware
- **All failure cases collapse to `UNAUTHORIZED` "invalid email or password"**

**Roles in the system:** `OFFICER`, `MEMBER`

---

# Rule Management Module (OFFICER only)

**Responsibilities:**
- `POST /rules` — create draft rule version
- `GET /rules` — list versions
- `GET /rules/{id}` — get one version with all `FineRuleDetail` rows
- `GET /rules/active` — get the currently active version
- `POST /rules/{id}/publish` — atomically deactivate previous, activate this
- `version_number` auto-increment (max + 1)
- Enforce unique `(rule_version_id, violation_type)` on `fine_rule_details`

**Must not:**
- Edit a published rule
- Have two active versions at the same time

---

# Fine Engine Module

**Responsibilities:**
- `time_multiplier` from `violation_timestamp` (half-open intervals, see `BUSINESS_RULES.md`)
- `repeat_multiplier` from counting prior unpaid invoices (PENDING|FAILED)
  for the same `license_plate` in the last 90 days
- Final fine = `base_amount × time_multiplier × repeat_multiplier`
- Build the `CalculationSnapshot` JSON

**Must not:**
- Be called on historical violations (those use the stored snapshot)

This is a **pure function** module with no DB access of its own — the
violation module passes in the inputs and persists the result.

---

# Violation Module (OFFICER writes, both roles read)

**Responsibilities:**
- `POST /violations` — create a violation, **auto-generate invoice**,
  freeze the rule version, store the snapshot
- `GET /violations` — list (member forced to own `member_id`)
- `GET /violations/{id}` — detail
- Compute the prior-unpaid count for the repeat multiplier
- Publish `ViolationCreated` and `InvoiceCreated` events

**Must not:**
- Allow a member to write
- Mutate a violation after creation (immutable)

---

# Invoice Module

**Responsibilities:**
- Generated automatically by the violation module
- `GET /invoices` — list (member forced to own)
- `GET /invoices/{id}` — detail with latest payment
- Expose status transitions only — never allow amount changes

**Must not:**
- Allow amount changes after creation
- Allow re-payment of a `PAID` invoice

---

# Payment Module (MEMBER writes)

**Responsibilities:**
- `POST /payments` — process payment via the mock provider
  (`PaymentService.charge(invoice_id, amount, scenario)`)
- Validate that the invoice is `PENDING` or `FAILED`
- Validate that the requester is the invoice's member
- Insert a new `payments` row
- Update the invoice status (`PAID` or `FAILED`)
- Publish `PaymentSucceeded` or `PaymentFailed`

**Must not:**
- Modify a `PAID` invoice
- Modify any violation, rule, or non-payment table
- Call any external service (mock is in-process)

---

# History Module

**Responsibilities:**
- `GET /history` — aggregated view: violation + invoice + latest payment
  + rule version + full `calculation_snapshot`
- Filter and pagination
- Member role forced to own `member_id`

---

# Uploads Module (OFFICER only, see PHOTO_STORAGE.md)

**Responsibilities:**
- `POST /uploads/violations` — multipart upload
- Validate MIME, extension, size
- Generate UUID filename
- Save to `storage/violations/`
- Serve `/uploads/*` as static files (gateway or violation service)

**Must not:**
- Store binaries in PostgreSQL
- Use the original filename

---

# Members Module (OFFICER read)

**Responsibilities:**
- `GET /members` — list members for officer's lookup when creating violations
- `GET /members/{id}` — detail

**Must not:** allow MEMBER to call.

---

# Notification Worker (see NOTIFICATIONS.md)

**Responsibilities:**
- Consume all events from RabbitMQ
- Log them
- Optionally insert into `notifications` table
- Use `processed_events` for idempotency

**Must not:** modify any business table.

---

# Role-based access matrix

| Endpoint                          | OFFICER | MEMBER (own only) |
| --------------------------------- | :-----: | :---------------: |
| POST /auth/login                  |   ✅    |        ✅         |
| POST /uploads/violations          |   ✅    |        ❌         |
| POST /violations                  |   ✅    |        ❌         |
| GET /violations                   |   ✅    |        ✅         |
| GET /violations/{id}              |   ✅    |        ✅         |
| GET /invoices                     |   ✅    |        ✅         |
| GET /invoices/{id}                |   ✅    |        ✅         |
| POST /payments                    |   ❌    |        ✅         |
| GET /history                      |   ✅    |        ✅         |
| POST /rules                       |   ✅    |        ❌         |
| GET /rules                        |   ✅    |        ❌         |
| GET /rules/{id}                   |   ✅    |        ❌         |
| GET /rules/active                 |   ✅    |        ❌         |
| POST /rules/{id}/publish          |   ✅    |        ❌         |
| GET /members                      |   ✅    |        ❌         |

> "own only" means the service layer ignores any `member_id` query param
> and forces it to `req.user.id`. Attempting to pass another user's id
> returns `FORBIDDEN`.
