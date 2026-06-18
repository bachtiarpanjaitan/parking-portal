# Module Workflow

> All workflows assume the request has already been **authenticated** by the
> API Gateway (JWT validated, `user_id` and `role` injected into the context).
> Role checks below are enforced in service-layer middleware on each backend service.
> See `MODULES.md` for the per-role permission matrix.

Legend:
- `(S)` = synchronous HTTP
- `(A)` = asynchronous (RabbitMQ event, after DB commit)
- `[OFFICER]` / `[MEMBER]` = role required for the user-facing step

---

# Flow 1 — Officer Creates Violation

```
[OFFICER] Web App
    | (S) POST /api/v1/uploads/violations  (multipart, photo)
    v
API Gateway  ───────────────►  Violation Service /uploads
                                    |
                                    v
                              Store file to storage/violations/<uuid>.<ext>
                                    |
                                    v
                              Return { photo_url }

[OFFICER] Web App
    | (S) POST /api/v1/violations
    |     { member_id, license_plate, violation_type,
    |       location, violation_timestamp, photo_url }
    v
API Gateway  ───────────────►  Violation Service
                                    |
                                    v
                              BEGIN TRANSACTION
                                    |
                                    v
                              Load ACTIVE rule version (DB)
                                    |
                                    v
                              Load FineRuleDetail for violation_type
                                    |
                                    v
                              Count prior UNPAID violations
                              (PENDING|FAILED invoices) for same license_plate
                              in last 90 days
                                    |
                                    v
                              Fine Engine
                              fine = base × time_mult × repeat_mult
                                    |
                                    v
                              INSERT violation
                              (frozen rule_version_id, calculation_snapshot)
                                    |
                                    v
                              INSERT invoice  (amount = fine, status = PENDING)
                                    |
                              COMMIT
                                    |
                                    v
                              Publish ViolationCreated (A)
                              Publish InvoiceCreated   (A)
                                    |
                                    v
                              Return { violation_id, invoice_id, fine_amount, snapshot }
```

**Failure paths:**
- Photo upload: `FILE_REQUIRED`, `FILE_TOO_LARGE`, `INVALID_FILE_TYPE`
- Violation: `VALIDATION_ERROR`, `LICENSE_PLATE_REQUIRED`, `PHOTO_REQUIRED`,
  `INVALID_VIOLATION_TYPE`, `NO_ACTIVE_RULE`, `FORBIDDEN` (not OFFICER)
- If RabbitMQ publish fails after commit → log error, do **not** roll back
  (event is best-effort, see ADR-011)

---

# Flow 2 — Officer Publishes New Rule Version

```
[OFFICER] Web App
    | (S) POST /api/v1/rules
    |     { rules: [ ...4 violation types... ] }
    v
Violation Service
    |
    v
INSERT fine_rule_versions  (is_active=false, version_number = max+1)
INSERT fine_rule_details  (4 rows, one per violation_type)
    |
    v
Return { id, version_number, is_active: false }

[OFFICER] Web App
    | (S) POST /api/v1/rules/{id}/publish
    v
Violation Service
    |
    v
BEGIN TRANSACTION
    UPDATE fine_rule_versions SET is_active=false WHERE is_active=true
    UPDATE fine_rule_versions SET is_active=true, published_at=now() WHERE id={id}
COMMIT
    |
    v
Publish RulePublished (A)
Return { id, version_number, is_active: true, published_at }
```

**Failure paths:**
- `RULE_VERSION_NOT_FOUND`
- `RULE_ALREADY_ACTIVE` (already published, no-op)
- `VALIDATION_ERROR` (missing types, bad amounts, non-positive multipliers)
- `FORBIDDEN` (not OFFICER)

**Historical safety:** existing violations are untouched — their `rule_version_id`
and `calculation_snapshot` are immutable. This is the core invariant of the
assignment (see ADR-004, BUSINESS_RULES.md).

---

# Flow 3 — Member Pays Invoice

```
[MEMBER] Web App
    | (S) POST /api/v1/payments
    |     { invoice_id, scenario: "success" | "failed" }
    v
API Gateway  ───────────────►  Payment Service
                                    |
                                    v
                              Load invoice (call to Violation Service or
                              read replica — for slice, call Violation
                              Service HTTP)
                                    |
                                    v
                              Validate:
                              - invoice exists
                              - invoice.status IN (PENDING, FAILED)
                              - invoice.member_id == req.user.id
                                    |
                                    v
                              Call internal mock:
                              PaymentService.charge(invoice_id, amount, scenario)
                              → { status: "paid"|"failed", transaction_id }
                                    |
                                    v
                              BEGIN TRANSACTION
                                INSERT payment
                                  (amount, transaction_id, status, scenario)
                                IF status == "paid":
                                  UPDATE invoice SET status = 'PAID'
                                ELSE:
                                  UPDATE invoice SET status = 'FAILED'
                              COMMIT
                                    |
                                    v
                              Publish PaymentSucceeded OR PaymentFailed (A)
                                    |
                                    v
                              Return { payment_id, status, invoice: { status, amount } }
```

**Failure paths:**
- `INVOICE_NOT_FOUND`
- `INVOICE_ALREADY_PAID` (cannot re-pay a PAID invoice)
- `INVALID_PAYMENT_SCENARIO` (must be "success" or "failed")
- `FORBIDDEN` (not the invoice's member, or not MEMBER)
- Mock provider "failed" outcome → HTTP **200**, body `status: "FAILED"`,
  invoice becomes `FAILED` (still payable, member can retry). This matches the
  assignment's "expose a way to choose the scenario" requirement.

---

# Flow 4 — View History

```
[OFFICER or MEMBER] Web App
    | (S) GET /api/v1/history
    |     ?member_id=...&from=...&to=...&page=...&page_size=...
    v
API Gateway  ───────────────►  Violation Service /history
                                    |
                                    v
                              For MEMBER role: force member_id = req.user.id
                                    |
                                    v
                              SELECT joined view:
                              violations ⨝ invoices ⨝ payments(latest) ⨝ fine_rule_versions
                                    |
                                    v
                              For each row, return:
                              violation fields + invoice_status + payment_status
                              + rule_version_number + calculation_snapshot
                                    |
                                    v
                              Return paginated list
```

**Failure paths:**
- `VALIDATION_ERROR` (bad date format, bad page)
- `FORBIDDEN` (MEMBER tried to query another member_id — forced to own)

---

# Flow 5 — Officer Updates Rule (without affecting past violations)

Same as Flow 2. The "without affecting past violations" property is **not** a
separate workflow — it is a property of how the system stores violations:
each violation carries a frozen `rule_version_id` and `calculation_snapshot`.
After a new rule is published, no read endpoint ever calls the current rule
to recompute historical fines. See `BUSINESS_RULES.md` → "History Rules"
and `TESTING_STRATEGY.md` → "Historical Fine Preservation".

---

# Cross-cutting: Event Publication (best-effort)

After every successful DB commit, the producing service attempts to publish
events to RabbitMQ:

| Event              | Published by        | Consumed by              |
| ------------------ | ------------------- | ------------------------ |
| `ViolationCreated` | Violation Service   | Notification Worker      |
| `InvoiceCreated`   | Violation Service   | Notification Worker      |
| `RulePublished`    | Violation Service   | Notification Worker      |
| `PaymentSucceeded` | Payment Service     | Notification Worker      |
| `PaymentFailed`    | Payment Service     | Notification Worker      |

If publish fails (broker down, network), the producing service logs
`EVENT_PUBLISH_FAILED` and continues — the user's request still succeeds.
Notification Worker uses a `processed_events` table for idempotency so
re-deliveries do not double-log. See `NOTIFICATIONS.md` for details.
