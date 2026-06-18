# Parking Violation Portal

A modular Parking Violation Portal built with **Go**, **React + TypeScript**, **PostgreSQL**, and **RabbitMQ**.

The system allows officers to issue parking violations, calculate fines using **versioned rule sets** (so historical fines never change), and allows members to pay fines via a **mocked payment provider**. All five flows from the assignment are supported end-to-end.

> **For reviewers:** see [`DESIGN.md`](./DESIGN.md) for the data flow diagram + ERD, and the [`.ai/`](./.ai/) folder for the full design documentation.

---

# Features

## Officer
- Login (mocked, email-only — see ADR-006)
- Create parking violations with **photo upload** (see `PHOTO_STORAGE.md`)
- View all violations and filter by member / plate / date
- Manage **fine rule versions** (draft → publish, atomic activate)
- View transaction history with the **rule version applied** at each violation

## Member
- Login
- View **own** violations (server forces `member_id = req.user.id` — see `MODULES.md`)
- View **own** invoices and pay them
- Choose payment scenario (`success` / `failed`) to exercise the mock provider
- View **own** payment history

---

# Architecture

4 services + 2 infra (per `ARCHITECTURE_DECISION.md` ADR-008, ADR-009):

```
React Frontend  (http://localhost:3000)
        |
        | HTTP (JWT)
        v
   API Gateway  (http://localhost:8080)   <- single entrypoint
        |
        +---- HTTP ----+----- HTTP -----+
        |              |               |
        v              v               v
 Violation Svc    Payment Svc    (static /uploads/*)
 (8081)           (8082)
        |              |
        +------+-------+
               |
               v
          PostgreSQL  (shared instance, ownership per SERVICE_BOUNDARIES.md)
               |
               ^  (publish events, best-effort after commit)
               |
          RabbitMQ  (parking.events topic exchange)
               |
               v
   Notification Worker  (consumer-only, logs events)
```

| Service              | Port | Owns                                             | Purpose |
| -------------------- | :--: | ------------------------------------------------ | ------- |
| API Gateway          | 8080 | nothing (stateless)                              | JWT validation, URL prefix routing, error envelope |
| Violation Service    | 8081 | `users` (read), `fine_rule_versions`, `fine_rule_details`, `violations`, `invoices`, photo upload | All flows except payment |
| Payment Service      | 8082 | `payments` (+ atomic update of `invoices.status`) | Mock payment provider, payment recording |
| Notification Worker  |  —   | `notifications` (optional), `processed_events`   | Consumes RabbitMQ events, logs them |
| PostgreSQL           | 5432 | all tables                                       | Shared by all services (per `SERVICE_BOUNDARIES.md`) |
| RabbitMQ             | 5672 | —                                                | Async event bus |
| Frontend (nginx)     | 3000 | —                                                | Vite-built static SPA |

---

# Tech Stack

## Backend
- **Go 1.24+**
- **Gin** (HTTP framework)
- **pgx** (PostgreSQL driver)
- **amqp091-go** (RabbitMQ client)
- **golang-jwt** (JWT)

## Frontend
- **React 18+**
- **TypeScript** (strict)
- **Vite**
- **TailwindCSS** + **shadcn/ui**
- **TanStack Query** (no direct `fetch`/`axios` from components)
- **react-hook-form** + **zod** (form validation)
- **zustand** (auth/UI state)

## Infrastructure
- Docker Compose
- PostgreSQL 17
- RabbitMQ 3 (with management plugin)

---

# Prerequisites

Install:
- Docker 24+
- Docker Compose v2
- (Optional, for local Go dev) Go 1.24+
- (Optional, for local frontend dev) Node.js 22+

---

# Quick Start (full stack with Docker)

```bash
# 1. Copy env file
cp .env.example .env

# 2. Build and start all services
docker compose up -d --build

# 3. Wait for services to be healthy, then run migrations + seed
docker compose exec violation-service ./migrate
docker compose exec violation-service ./seed

# 4. Open the app
# Frontend:  http://localhost:3000
# Gateway:   http://localhost:8080
# RabbitMQ:  http://localhost:15672  (guest / guest)
```

That's it. No external services required.

---

# Running Locally (without Docker, for development)

## 1. Start infrastructure only

```bash
docker compose up -d postgres rabbitmq
```

## 2. Backend

Each Go service has its own module under `backend/<service>/`.

```bash
# violation-service
cd backend/violation-service
cp ../../.env.example .env
go mod tidy
go run cmd/migrate/main.go    # run migrations
go run cmd/seed/main.go       # seed demo data
go run cmd/api/main.go        # start on :8081

# payment-service
cd backend/payment-service
go mod tidy
go run cmd/api/main.go        # start on :8082

# gateway
cd backend/gateway
go mod tidy
go run cmd/api/main.go        # start on :8080

# notification-worker
cd backend/notification-worker
go mod tidy
go run cmd/worker/main.go
```

## 3. Frontend

```bash
cd frontend
npm install
cp ../.env.example .env       # or just VITE_API_URL=http://localhost:8080/api/v1
npm run dev                   # start on :5173 (Vite dev server)
```

Frontend dev URL: `http://localhost:5173` (proxies API to `http://localhost:8080`)

---

# Test Accounts (from `.ai/SEED_DATA.md`)

| Role    | Email                | Notes |
| ------- | -------------------- | ----- |
| Officer | `officer@example.com`  | Can create violations, manage rules, view all data |
| Member  | `member@example.com`   | Sees 4 seeded violations (1 PAID, 1 PENDING, 1 FAILED, 1 ready to pay) |
| Member  | `member2@example.com`  | No violations, for variety in officer screens |

Login is **email-only** (no password) — the gateway returns a JWT immediately.

---

# Payment Testing

The payment provider is **mocked in-process** (no external call). See `ARCHITECTURE_DECISION.md` ADR-012.

When paying an invoice, the UI lets you choose a scenario:

| Scenario    | Mock response | Invoice status after | Retriable? |
| ----------- | ------------- | -------------------- | ---------- |
| `success`   | `paid`        | `PAID` (terminal)    | No         |
| `failed`    | `failed`      | `FAILED`             | **Yes** — member can retry |

The HTTP response is **200** for both scenarios; the body's `status` field reflects
the outcome. The `INVOICE_ALREADY_PAID` error is returned only if a `PAID` invoice
is paid again. See `API_CONTRACTS.md` and `ERROR_CATALOG.md`.

---

# Rule Versioning — Why History is Immutable

When an officer creates a violation, the service:
1. Loads the **currently active** rule version.
2. Calculates the fine using that version's `FineRuleDetail`.
3. **Persists** the violation with:
   - `rule_version_id` (FK, frozen)
   - `fine_amount` (frozen)
   - `calculation_snapshot` (JSONB, frozen — contains base, multipliers, snapshot of inputs)

When an officer later publishes **Rule Version 2** (e.g. higher base amounts):
- The new version is activated.
- **No existing violation is touched.** The history view reads `calculation_snapshot`
  and `rule_version_id` directly — it never recomputes.

This is the **core invariant** of the assignment. See `BUSINESS_RULES.md` and
`TESTING_STRATEGY.md` "Historical Fine Preservation".

---

# API Quick Reference

Base URL: `http://localhost:8080/api/v1`

Full contract: `API_CONTRACTS.md`.

| Method | Path                          | Role         | Purpose |
| ------ | ----------------------------- | ------------ | ------- |
| POST   | `/auth/login`                 | any          | Mocked login, returns JWT |
| POST   | `/uploads/violations`         | OFFICER      | Upload photo, returns `photo_url` |
| POST   | `/violations`                 | OFFICER      | Create violation (auto-calculates fine) |
| GET    | `/violations`                 | any (own)    | List violations |
| GET    | `/violations/{id}`            | any (own)    | Get violation detail |
| GET    | `/invoices`                   | any (own)    | List invoices |
| GET    | `/invoices/{id}`              | any (own)    | Get invoice + latest payment |
| POST   | `/payments`                   | MEMBER       | Pay an invoice (choose scenario) |
| GET    | `/history`                    | any (own)    | Aggregated history with rule version |
| GET    | `/rules`                      | OFFICER      | List rule versions |
| GET    | `/rules/{id}`                 | OFFICER      | Get version + all details |
| GET    | `/rules/active`               | OFFICER      | Get currently active version |
| POST   | `/rules`                      | OFFICER      | Create draft version |
| POST   | `/rules/{id}/publish`         | OFFICER      | Publish draft (atomic activate) |
| GET    | `/members`                    | OFFICER      | Member lookup for officer screens |

---

# Project Layout

See `.ai/FOLDER_STRUCTURE.md` for the full layout. Quick view:

```
parking_violation_portal/
├── backend/
│   ├── gateway/                # API Gateway (port 8080)
│   ├── violation-service/      # Rules, violations, invoices, uploads (port 8081)
│   ├── payment-service/        # Mock payment provider (port 8082)
│   ├── notification-worker/    # RabbitMQ consumer
│   └── pkg/                    # Cross-service Go types
├── frontend/                   # Vite + React + TS
├── storage/                    # Photo uploads (mounted volume)
├── docs/                       # DESIGN.md assets (ERD, data flow)
├── docker-compose.yml
├── .env.example
├── DESIGN.md
├── README.md
└── .ai/                        # Design documentation (17 files)
```

---

# Assumptions

1. One violation generates **exactly one** invoice (no installments, no partial payments).
2. The payment provider is **mocked in-process** — no real network call. The
   `scenario` query param drives the outcome for testing.
3. **Login is email-only** (no password) — see ADR-006. A production system
   would add passwords, refresh tokens, MFA.
4. The **time window** decision uses half-open intervals (06:00–21:59 = day,
   22:00–05:59 = night) — see the warning at the top of `BUSINESS_RULES.md`.
   This resolves the ambiguity in the assignment PDF.
5. **License plate is not unique to a member** — the officer confirms which
   `member_id` is associated with the plate when creating a violation. The
   plate is the unit used for the repeat-offender calculation.
6. **Member can retry a `FAILED` invoice** — FAILED is not terminal. A new
   `payments` row is created on each attempt.
7. **One rule version is active at any time** — enforced by partial unique
   index on `is_active = true` and by the atomic publish flow.
8. **Historical violations are immutable** — their `rule_version_id` and
   `calculation_snapshot` are frozen at creation.
9. **Photos are stored on the local filesystem** (not S3/MinIO) — see ADR-010.
   A `storage/violations/` volume is mounted on the violation service.
10. **RabbitMQ events are best-effort** — they fire after the DB commit, and
    if the broker is down the request still succeeds. See ADR-011.

---

# Trade-Offs

## Modular services with shared database
The assignment requires modular services. We use **logical** separation
(separate Go services, separate deployments) with a **shared PostgreSQL
instance** for operational simplicity. For a larger system, each service
would own its own database. See `SERVICE_BOUNDARIES.md` "Database ownership summary".

## Synchronous writes + asynchronous events
Core writes (violation+invoice, payment+status update) are **synchronous HTTP**
within a single DB transaction. RabbitMQ is used only for **side effects**
(notification logging). This makes the 5 flows predictable and testable.
See ADR-011.

## Local photo storage
S3/MinIO would be more scalable, but the assignment scope is small and the
slice needs to be runnable with `docker compose up` and nothing else. The
`storage/violations/` volume persists uploads across container restarts.
See ADR-010.

## In-process mock payment provider
The assignment explicitly says "Mock response from payment provider in
internal service." The `PaymentService.charge()` is a Go function, not an
HTTP call. This keeps the slice self-contained.

---

# What I would do with more time

- **Per-service databases** + outbox pattern for reliable event publishing
- **Real S3/MinIO** + signed URLs for photos
- **Refresh tokens** + MFA for production-ready auth
- **OpenTelemetry** tracing across gateway → services → DB
- **Service-to-service auth** (the gateway currently uses JWT for clients only;
  backend-to-backend traffic is unauthenticated inside the Docker network)
- **Comprehensive integration tests** (the unit tests cover the fine engine and
  rule versioning; the slice would benefit from testcontainers-based e2e tests)
- **CI/CD** (GitHub Actions, push to ECR, deploy to ECS/K8s)
- **Rate limiting** at the gateway
- **A real notification center** (the worker currently logs only)

---

# Deliverables

- ✅ `README.md` (this file)
- ✅ `DESIGN.md` — data flow + ERD (draw.io, attached as images in `docs/`)
- ✅ Working source code (Go + React + TypeScript)
- ✅ Docker Compose configuration
- ✅ `.ai/` design documentation (17 files, self-consistent)
- ✅ Seed data + migrations
- ✅ Unit tests for fine engine and rule versioning
