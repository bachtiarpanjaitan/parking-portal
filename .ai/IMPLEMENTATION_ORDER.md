# Implementation Order

> Phases below assume the per-service layout in `FOLDER_STRUCTURE.md`
> (gateway, violation-service, payment-service, notification-worker)
> and the role-based access matrix in `MODULES.md`.

Complete tasks in this exact order. Each phase has a single deliverable
that can be demoed end-to-end.

---

# Phase 1 — Project Setup

**Tasks:**
- Initialize monorepo (this folder structure)
- Initialize Go modules: `gateway/`, `violation-service/`, `payment-service/`, `notification-worker/`, `pkg/`
- Initialize `docker-compose.yml` with: `postgres`, `rabbitmq`, the 4 services, and a volume for `storage/`
- Initialize Vite + React + TypeScript + TailwindCSS frontend
- Set up `Makefile` with `make up`, `make down`, `make logs`, `make seed`, `make test`
- Copy `.env.example` to `.env` for both backend and frontend (see `ENVVAR_CONFIG.md`)
- Verify all 4 services start and the frontend loads at `http://localhost:5173`

**Deliverable:** `docker-compose up` brings up the full stack; the gateway returns 200 on a ping endpoint.

---

# Phase 2 — Database & Shared Infra

**Tasks:**
- Create migrations for all tables in `DATABASE_MAPPING.md` (run in violation-service, shared by all backend services via one Postgres instance)
- Create seeders per `SEED_DATA.md`
- Create shared Go types in `backend/pkg/` (events envelope, error codes, money helpers)
- Create database connection helper (`internal/database/postgres.go`)

**Tables:**
- `users`
- `fine_rule_versions`
- `fine_rule_details`
- `violations` (must include `violation_timestamp`)
- `invoices`
- `payments` (must include `amount`)
- `notifications` (optional, for worker)
- `processed_events` (optional, for worker)

**Deliverable:** `make migrate && make seed` runs cleanly; tables exist; 4 demo violations are present.

---

# Phase 3 — Authentication

**Tasks:**
- `POST /auth/login` in the gateway (email-only, see ADR-006)
- JWT generation with `sub` (user id) and `role` claims
- Gateway middleware: validate JWT on all routes except `/auth/login`
- Gateway middleware: extract `user_id` and `role` into request context
- Service-side middleware: role check (OFFICER / MEMBER) per `MODULES.md`

**Deliverable:** `POST /auth/login` returns a valid JWT; calling any protected route without the token returns `UNAUTHORIZED`; calling a MEMBER-only route with an OFFICER token returns `FORBIDDEN`.

---

# Phase 4 — Photo Uploads

**Tasks:**
- `POST /api/v1/uploads/violations` in violation-service
- File validation: MIME, extension, size (per `PHOTO_STORAGE.md`)
- Save to `storage/violations/<uuid>.<ext>`
- Serve `/uploads/*` statically (gateway or violation-service)
- Mount the `storage/` volume in `docker-compose.yml`

**Deliverable:** an officer can upload a JPG via the API and receives a `photo_url`; the file is reachable at `http://localhost:8080/uploads/violations/<filename>`.

---

# Phase 5 — Rule Management

**Tasks:**
- `GET /rules`, `GET /rules/{id}`, `GET /rules/active`
- `POST /rules` (create draft version with 4 details)
- `POST /rules/{id}/publish` (atomic activate/deactivate)
- Enforce unique `(rule_version_id, violation_type)`
- Partial unique index for `is_active = true` (only one active version)
- Unit tests for publish flow + historical preservation (see `TESTING_STRATEGY.md`)

**Deliverable:** an officer can list, create, and publish rule versions; publishing a new version leaves the old violations' snapshot untouched.

---

# Phase 6 — Fine Engine (pure functions)

**Tasks:**
- Implement `time_multiplier` using the **half-open intervals** from `BUSINESS_RULES.md` (06:00–21:59 day, 22:00–05:59 night)
- Implement `repeat_multiplier` from `repeat_0` / `repeat_1` / `repeat_2_plus`
- `Calculate(rule, violationTimestamp, priorUnpaidCount) → (amount, snapshot)` pure function
- Unit tests covering day, night, repeat 0/1/2+ (see `TESTING_STRATEGY.md`)

**Deliverable:** all fine-engine unit tests pass.

---

# Phase 7 — Violation Module

**Tasks:**
- `POST /violations` — open transaction:
  1. Load active rule + `FineRuleDetail`
  2. Count prior unpaid invoices (PENDING|FAILED) for the same `license_plate` in last 90 days
  3. Call fine engine
  4. Insert violation with frozen `rule_version_id` + `calculation_snapshot`
  5. Insert invoice (status PENDING)
  6. Commit
  7. Publish `ViolationCreated`, `InvoiceCreated` (best-effort, see ADR-011)
- `GET /violations` with `member_id` filter (forced to own for MEMBER)
- `GET /violations/{id}`
- `GET /members` for officer lookup
- Member-role middleware: force `member_id = req.user.id`

**Deliverable:** an officer can create a violation via the API; the response shows `fine_amount` and the full `calculation_snapshot`; the invoice is created with the same amount.

---

# Phase 8 — Payment Module

**Tasks:**
- Mock `PaymentService.charge(invoice_id, amount, scenario)` (in-process, see ADR-012)
- `POST /payments` — open transaction:
  1. Validate invoice exists, status is PENDING or FAILED, requester is the invoice's member
  2. Call `charge()` → `{ status, transaction_id }`
  3. Insert payment row (with `amount` from invoice)
  4. Update invoice status to PAID or FAILED
  5. Commit
  6. Publish `PaymentSucceeded` or `PaymentFailed`
- Unit tests for both scenarios (see `TESTING_STRATEGY.md`)

**Deliverable:** a member can pay a PENDING invoice; choosing `success` makes the invoice PAID, choosing `failed` makes it FAILED and the member can retry.

---

# Phase 9 — History

**Tasks:**
- `GET /history` with member filter (forced to own for MEMBER)
- Join violations ⨝ invoices ⨝ latest payment ⨝ fine_rule_versions
- Return the full `calculation_snapshot` for transparency

**Deliverable:** the history endpoint returns the 4 seeded demo rows with correct status, fine, and rule_version_number.

---

# Phase 10 — Notification Worker

**Tasks:**
- RabbitMQ consumer subscribed to `parking.events` exchange
- Log every event
- Optionally insert into `notifications` (or log-only for the slice)
- Idempotency via `processed_events`
- Reconnect logic on broker restart

**Deliverable:** publishing a `ViolationCreated` from violation-service results in a log line in the worker container.

---

# Phase 11 — Frontend

**Tasks:**
- Login page (email only) → stores JWT in `localStorage` or cookie
- OfficerLayout / MemberLayout with role-based sidebar (see `UI_DESIGN.md`)
- Pages: Dashboard, Violations (list + create), Rules (list + create + publish), Invoices, Payments, History
- Reusable `DataTable`, `Form`, `Button`, `Card` (see `UI_GUIDELINES.md`)
- TanStack Query for all data; module `service.ts` is the only place that calls `axios`
- Scenario selector on the payment page (`success` / `failed`)

**Deliverable:** the full UI works end-to-end with the seeded data; an officer can publish a new rule and a member can pay with both scenarios.

---

# Phase 12 — Documentation & Submission

**Tasks:**
- `README.md` — how to run locally (docker-compose), env setup, test accounts, assumptions
- `DESIGN.md` — data flow + ERD (draw.io, attached as images)
- Final pass on `.ai/` consistency
- `make test` is green
- All 5 assignment flows pass the e2e scenarios in `TESTING_STRATEGY.md`

**Deliverable:** assignment submission is ready for review.
