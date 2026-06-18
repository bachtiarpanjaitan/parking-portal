# Service Boundaries

> **Service list** (per `ARCHITECTURE_DECISION.md` ADR-008 and ADR-009):
> **API Gateway, Violation Service, Payment Service, Notification Worker**.
> If you find an older version of this file that lists only 3 services,
> it has been superseded by ADR-008.

---

# API Gateway

**Owns:** nothing (stateless proxy + auth).

**Responsibilities:**
- HTTPS termination (out of scope for slice; HTTP only)
- JWT validation on every request except `POST /auth/login`
- `user_id`, `role` injection into request context
- Routing to backend services by URL prefix
- Standardized error envelope (see `ERROR_CATALOG.md`)
- Static file serving for `/uploads/*` (or delegate to violation service)

**Must not:**
- Call the database directly
- Perform fine calculations
- Hold business state

**Routes:**
- `/api/v1/auth/*` → handled by gateway (calls violation service internally)
- `/api/v1/uploads/*`, `/api/v1/violations/*`, `/api/v1/invoices/*`,
  `/api/v1/rules/*`, `/api/v1/members/*`, `/api/v1/history/*` → Violation Service
- `/api/v1/payments/*` → Payment Service
- `/uploads/*` → static files (storage volume)

---

# Violation Service

**Owns:**
- `users` (read-only; writes go through the auth helper used by gateway login)
- `fine_rule_versions`
- `fine_rule_details`
- `violations`
- `invoices`

**Responsibilities:**
- Rule management (CRUD + publish)
- Fine calculation
- Violation creation
- Invoice creation
- Photo upload (writes to `storage/violations/`, see `PHOTO_STORAGE.md`)
- History aggregation
- Member lookup for officers
- Role-based access enforcement for its endpoints

**Can publish events:**
- `ViolationCreated`
- `InvoiceCreated`
- `RulePublished`

**Must not:**
- Process payments
- Own or read the `payments` table (read-only access for joining is allowed
  in the history view)

---

# Payment Service

**Owns:**
- `payments`

**Responsibilities:**
- Mock payment provider (`PaymentService.charge(invoice_id, amount, scenario)`)
- Payment processing
- Payment recording
- Invoice status update **within its own transaction** (so a `payments`
  write and the corresponding `invoices` status update are atomic)

**Can publish events:**
- `PaymentSucceeded`
- `PaymentFailed`

**Must not:**
- Modify violations
- Modify fine rules
- Modify any other service's tables directly — it can update `invoices.status`
  because that is a payment-driven transition; this is the one allowed
  cross-service write and is documented in `MODULE_WORKFLOW.md` Flow 3.

---

# Notification Worker

**Owns:**
- `notifications` (optional)
- `processed_events`

**Responsibilities:**
- Consume all events from RabbitMQ exchange `parking.events`
- Log them
- Optionally persist to `notifications`
- Idempotency via `processed_events`

**Must not:**
- Modify any business data
- Publish events

---

# Database ownership summary

| Table                  | Owner (write)         | Read access        |
| ---------------------- | --------------------- | ------------------ |
| users                  | Gateway (login only)  | All services       |
| fine_rule_versions     | Violation Service     | All services       |
| fine_rule_details      | Violation Service     | All services       |
| violations             | Violation Service     | All services       |
| invoices               | Violation Service (create) + Payment Service (status update only) | All services |
| payments               | Payment Service       | Violation Service (history join only) |
| notifications          | Notification Worker   | All services       |
| processed_events       | Notification Worker   | Notification Worker only |

> Violation Service has **read-only** access to `payments` for the history
> view only. It cannot insert or update payments.
