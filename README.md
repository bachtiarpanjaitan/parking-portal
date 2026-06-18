# Parking Violation Portal

A modular Parking Violation Portal built with **Go (Gin)**, **React + TypeScript (Vite)**, **PostgreSQL**, **RabbitMQ**, and the real **Midtrans Snap** payment gateway.

The system lets officers issue parking violations, calculate fines using **versioned rule sets** (so historical fines never change), and lets members pay fines via the Midtrans Snap UI (GoPay / QRIS). All five flows from the assignment are supported end-to-end.

> **For reviewers:** see [`DESIGN.md`](./DESIGN.md) for the data flow diagram + ERD, and the [`.ai/`](./.ai/) folder for the full design documentation.

---

# Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Makefile Shortcuts](#makefile-shortcuts) ⭐
- [Quick Start](#quick-start) ⭐
  - [Option A: Docker Compose (easiest)](#option-a-docker-compose-easiest)
  - [Option B: Local dev (no Docker)](#option-b-local-dev-no-docker) ⭐
- [Test Accounts](#test-accounts)
- [Payment Testing (Midtrans)](#payment-testing-midtrans)
- [API Quick Reference](#api-quick-reference)
- [Project Layout](#project-layout)
- [Assumptions](#assumptions)
- [Trade-Offs](#trade-offs)

---

# Features

## Officer
- Login (email + bcrypt — see `ADR-006`)
- Create parking violations with **photo upload** (see `PHOTO_STORAGE.md`)
- View all violations, filter by member / plate / date
- Manage **fine rule versions** (draft → publish, atomic activate)
- View transaction history with the **rule version applied** at each violation

## Member
- Login
- View **own** violations (server forces `member_id = req.user.id` — see `MODULES.md`)
- View **own** invoices and pay them
- Pay with **Midtrans Snap** (GoPay / QRIS — sandbox mode)
- View **own** payment history

---

# Architecture

4 backend services + 1 worker + 2 infra (per `ARCHITECTURE_DECISION.md` ADR-008, ADR-009):

```
React Frontend  (http://localhost:3000)
        |
        | HTTP (JWT)
        v
   API Gateway  (http://localhost:8080)   <-- single entrypoint (ADR-009)
        |
        +---- HTTP ----+----- HTTP -----+
        |              |               |
        v              v               v
 Violation Svc    Payment Svc    (static /uploads/*)
 (8081)           (8082)            (proxied to violation)
        |              |
        +------+-------+
               |
               v
          PostgreSQL  (shared instance, ownership per SERVICE_BOUNDARIES.md)
               |
               ^  (publish events, best-effort after commit — ADR-011)
               |
          RabbitMQ  (parking.events topic exchange)
               |
               v
   Notification Worker  (consumer-only, logs events — ADR-008)
```

| Service              | Port | Owns                                             | Purpose |
| -------------------- | :--: | ------------------------------------------------ | ------- |
| API Gateway          | 8080 | nothing (stateless)                              | JWT validation, URL prefix routing, error envelope |
| Violation Service    | 8081 | `users`, `fine_rule_versions`, `fine_rule_details`, `violations`, `invoices`, photo upload | All flows except payment |
| Payment Service      | 8082 | `payments` (+ atomic update of `invoices.status`) | **Midtrans Snap** integration |
| Notification Worker  |  —   | `notifications` (optional), `processed_events`   | Consumes RabbitMQ events, logs them |
| PostgreSQL           | 5432 | all tables                                       | Shared by all services (per `SERVICE_BOUNDARIES.md`) |
| RabbitMQ             | 5672 | —                                                | Async event bus |
| Frontend (Vite)      | 3000 | —                                                | React SPA (dev) or nginx (prod) |

---

# Tech Stack

## Backend
- **Go 1.24+**
- **Gin** (HTTP framework)
- **pgx v5** (PostgreSQL driver, connection pool)
- **amqp091-go** (RabbitMQ client)
- **golang-jwt v5** (JWT, HS256)
- **decimal** (`shopspring/decimal` for money)
- **validator.v10** (request validation)
- **zap** (structured logging)
- **bcrypt** (password hashing)

## Frontend
- **React 18+**
- **TypeScript** (strict mode)
- **Vite 5** (dev server + build)
- **TailwindCSS 3** (utility-first CSS)
- **TanStack Query 5** (data fetching — no raw `fetch` in components)
- **react-hook-form** + **zod** (form validation)
- **zustand 5** (auth/UI state)
- **axios** (HTTP client + interceptors for JWT + 401 handling)
- **react-router-dom 6** (routing)
- **Midtrans Snap.js** (loaded from CDN, `window.snap.pay(token)`)

## Infrastructure
- Docker Compose v2
- PostgreSQL 17
- RabbitMQ 3 (with management plugin)
- Midtrans Sandbox (free, real network calls)

---

# Prerequisites

Install:
- **Docker 24+** & **Docker Compose v2** (for Option A)
- **Go 1.24+** (for Option B / local dev)
- **Node.js 22+** (or **Bun** / **pnpm** / **yarn** — any work)
- **Midtrans Sandbox account** — server key (`SB-Mid-server-...`) already in `.env.example`

For the Midtrans Snap UI in the browser, no extra setup — it loads from `app.sandbox.midtrans.com` CDN.

---

# Makefile Shortcuts

> Every common workflow is a one-liner via the `Makefile` at the repo root.
> Run `make help` to print the full list, or `make ports` to see the per-service ports.

| Target                | What it does                                                                |
| --------------------- | --------------------------------------------------------------------------- |
| `make help`           | Print all targets with their descriptions.                                  |
| `make ports`          | Print the configured per-service ports (8080/8081/8082).                    |
| `make up`             | `docker compose up -d --build` — start **everything** (infra + 4 services + frontend). |
| `make down`           | `docker compose down` — stop everything.                                    |
| `make logs`           | Tail logs from all compose services.                                        |
| `make ps`             | List running compose services.                                              |
| `make build`          | Build all docker images.                                                    |
| `make rebuild SVC=X`  | Rebuild a single service image (e.g. `SVC=violation-service`).              |
| `make migrate`        | Run SQL migrations against the violation-service DB (via compose).          |
| `make seed`           | Seed demo users + violations + invoices (idempotent).                       |
| `make fresh`          | **Destructive** — drop volumes, re-init DB, re-seed, restart everything.    |
| `make test`           | Run all Go unit tests (`cd backend && go test ./...`).                      |
| `make fmt`            | `gofmt -w` across the backend.                                              |
| `make tidy`           | `go mod tidy` in the backend.                                               |
| `make clean`          | `docker compose down -v` + remove build artifacts.                          |
| `make run-violation`  | Run the **violation service** on `:8081` (loads `../.env`).                 |
| `make run-payment`    | Run the **payment service** on `:8082` (loads `../.env`).                   |
| `make run-gateway`    | Run the **API gateway** on `:8080` (loads `../.env`).                       |
| `make run-worker`     | Run the **notification worker** locally (no HTTP port — RabbitMQ consumer). |

### Why `make run-*` Just Works

- **Ports are baked into the Makefile** itself (8080 / 8081 / 8082 / no-port), so the
  `.env` doesn't need an `APP_PORT` line and there's no risk of two services
  accidentally colliding on the same port. Run `make ports` any time to see the
  current assignments.
- The Go services **auto-load the project-root `.env`** on startup via
  `backend/pkg/dotenv` (a tiny, dependency-free loader that walks up from
  the current working directory). So no need to prefix every command with
  `JWT_SECRET=... DB_USER=...` or to `cp .env backend/.env`.
- Existing shell env vars always win over the file (so Docker / CI keep working).

If you ever need to override a single value, just inline it on the command line:

```bash
# Override just the port for one run
make run-violation VIOLATION_PORT=9081

# Or override a database setting
DB_USER=myuser make run-violation
```

> **Heads up:** the `run-*` targets assume you want to point at local infra
> (`localhost:5432`, `localhost:5672`). If you're running everything in
> Docker, prefer `make up` instead.

---

# Quick Start

## Option A: Docker Compose (easiest)

> One command brings up Postgres + RabbitMQ + all 4 backend services + the frontend nginx. Migrations and seed run automatically via the entrypoint scripts.

```bash
# 1. Copy env file
cp .env.example .env

# 2. (optional) edit .env — set MIDTRANS_SERVER_KEY to your real sandbox key
#    The default key in .env.example is a public demo key (works for dev).

# 3. Build + start everything
make up
# (equivalent to: docker compose up -d --build)

# 4. Open the app
open http://localhost:3000          # Frontend (Vite dev)
# or
open http://localhost:15672          # RabbitMQ UI  (guest / guest)
```

Migrations and seed run automatically on container start. No extra steps.

---

## Option B: Local dev (no Docker)

> Run the Go services directly, point them at a local Postgres + RabbitMQ. Useful when you want fast iteration without rebuilding containers.

### Step 0 — Start the infra (one terminal)

```bash
# Option B1: via Docker (infra only, faster)
docker compose up -d postgres rabbitmq

# Option B2: via brew (no Docker at all)
brew services start postgresql
brew services start rabbitmq
createdb parking_portal
```

Verify:
- Postgres on `localhost:5432` (user `bachtiarpanjaitan` or `postgres`)
- RabbitMQ on `localhost:5672`

### Step 1 — `.env` + migrations + seed

```bash
cp .env.example .env

# Apply all 11 migrations
make migrate

# Seed 3 users + 4 violations + 4 invoices
make seed
```

> Both `make migrate` and `make seed` run inside the `violation-service`
> container, so they pick up the same `.env` automatically.

### Step 2 — Start the 4 backend services (separate terminals)

The Go services auto-load `../.env` (see [Makefile Shortcuts](#makefile-shortcuts)),
so these are one-liners — no env-var prefixing, no port juggling:

```bash
# Terminal A — Violation Service (port 8081)
make run-violation

# Terminal B — Payment Service (port 8082)
make run-payment

# Terminal C — API Gateway (port 8080)  <-- THE FRONTEND TALKS TO THIS
make run-gateway

# Terminal D — Notification Worker (no HTTP, RabbitMQ consumer)
make run-worker
```

Wait for the 3 HTTP services to log "listening" before continuing.

> **Need to override a port for one run?** Use the `*_PORT` make variables:
> `make run-violation VIOLATION_PORT=9081`
>
> **Need to override a database setting?** Inline it as usual:
> `DB_USER=myuser make run-violation`

### Step 3 — Start the frontend

```bash
# Terminal E — Vite dev (port 3000)
cd frontend
npm install     # first time only
npm run dev     # or: bun run dev / pnpm dev
```

The Vite config proxies `/api/*` and `/uploads/*` to `http://localhost:8080` (the gateway).

### Step 4 — Open the app

```bash
open http://localhost:3000
```

### Quick health check

```bash
# Gateway
curl http://localhost:8080/healthz
# → {"service":"api-gateway","status":"ok","upstream":{...}}

# Login as officer
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"officer@example.com","password":"password123"}'
# → {"success":true,"data":{"token":"...","user":{...}}}

# Hit history (with the token)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/history
```

If any of these fail with `ECONNREFUSED` or `connection refused`, you skipped
**Step 2** — go back and start the backend services first.

---

# Test Accounts (from `.ai/SEED_DATA.md`)

| Role    | Email                 | Password      | Notes |
| ------- | --------------------- | ------------- | ----- |
| Officer | `officer@example.com` | `password123` | Can create violations, manage rules, view all data |
| Member  | `member@example.com`  | `password123` | Sees 4 seeded violations (1 PAID, 1 PENDING, 1 FAILED, 1 ready to pay) |
| Member  | `member2@example.com` | `password123` | No violations, for variety in officer screens |

All passwords are hashed with bcrypt (`DefaultCost`). Login failure (wrong
email OR wrong password) returns the same `401 UNAUTHORIZED` to avoid leaking
which case occurred (see ADR-006).

---

# Payment Testing (Midtrans)

The payment integration uses the **real Midtrans Snap sandbox**. When a member
clicks "Pay with Midtrans" in the Invoices page, the frontend calls
`POST /api/v1/payments/snap-token` → backend calls Midtrans → returns
`snap_token` → frontend opens `window.snap.pay(token, callbacks)`.

**Payment methods** enabled: `qris, gopay` (configurable via
`MIDTRANS_ENABLED_METHODS=qris,gopay,shopeepay,dana,...`).

### Sandbox test flow

1. Open `http://localhost:3000` → log in as `member@example.com`
2. Sidebar → **My Invoices** → click **Pay with Midtrans** on a PENDING invoice
3. Midtrans Snap UI opens in a popup/modal
4. Choose **GoPay** or **QRIS** (sandbox mode — no real money)
5. For GoPay, Midtrans shows a **simulator phone number** in sandbox — use any
   number like `081234567890` and the test will auto-approve.
6. For QRIS, Midtrans shows a QR code you can scan with the Midtrans sandbox
   app (or the auto-approve page).
7. On success, you return to the Invoices page with the status now **PAID**.

The payment record is stored in the `payments` table with the full Midtrans
response (transaction_id, payment_method, transaction_status) in
`midraw_response` (JSONB) for debugging. See `ARCHITECTURE_DECISION.md` ADR-012.

**To use a different Midtrans key**, edit `.env`:

```env
MIDTRANS_SERVER_KEY=SB-Mid-server-XXXXXX   # your sandbox key
MIDTRANS_ENV=sandbox                       # or production
MIDTRANS_ENABLED_METHODS=qris,gopay,shopeepay
```

---

# API Quick Reference

Base URL: `http://localhost:8080/api/v1`

Full contract: `API_CONTRACTS.md`. The frontend never calls the backend
services directly — it goes through the gateway.

| Method | Path                          | Role         | Purpose |
| ------ | ----------------------------- | ------------ | ------- |
| POST   | `/auth/login`                 | any          | Email + password login, returns JWT |
| POST   | `/uploads/violations`         | OFFICER      | Upload photo, returns `photo_url` |
| POST   | `/violations`                 | OFFICER      | Create violation (auto-calculates fine) |
| GET    | `/violations`                 | any (own)    | List violations |
| GET    | `/violations/{id}`            | any (own)    | Get violation detail |
| GET    | `/invoices`                   | any (own)    | List invoices |
| GET    | `/invoices/{id}`              | any (own)    | Get invoice + latest payment |
| POST   | `/payments/snap-token`        | MEMBER       | Create Midtrans Snap token (returns `snap_token` + `redirect_url`) |
| POST   | `/payments/notification`      | public       | Midtrans webhook callback (no auth) |
| GET    | `/history`                    | any (own)    | Aggregated history with rule version |
| GET    | `/rules`                      | OFFICER      | List rule versions |
| GET    | `/rules/{id}`                 | OFFICER      | Get version + all details |
| GET    | `/rules/active`               | OFFICER      | Get currently active version |
| POST   | `/rules`                      | OFFICER      | Create draft version |
| POST   | `/rules/{id}/publish`         | OFFICER      | Publish draft (atomic activate) |
| GET    | `/members`                    | OFFICER      | Member lookup for officer screens |
| GET    | `/payments`                   | any          | List payments |
| GET    | `/payments/{id}`              | any (own)    | Get payment detail |

All `/api/v1/*` routes (except `/auth/login` and `/payments/notification`) require a `Authorization: Bearer <jwt>` header.

---

# Project Layout

```
parking_violation_portal/
├── backend/
│   ├── gateway/                  # API Gateway (port 8080) - ADR-009
│   ├── violation-service/        # Rules, violations, invoices, uploads (port 8081)
│   ├── payment-service/          # Midtrans Snap integration (port 8082)
│   ├── notification-worker/      # RabbitMQ consumer
│   ├── pkg/                      # Cross-service Go types (auth, events, dotenv, etc)
│   └── pkg/dotenv/               # Tiny .env auto-loader (no external deps)
├── frontend/                     # Vite + React + TS
│   ├── src/
│   │   ├── lib/api.ts            # axios + JWT + 401 handler + friendly errors
│   │   ├── store/auth.ts         # zustand auth store
│   │   ├── components/Layout.tsx # role-aware sidebar
│   │   └── pages/                # LoginPage, DashboardPage, ViolationsPage, etc
│   └── README.md                 # frontend-specific notes
├── storage/                      # Photo uploads (mounted volume)
├── docs/                         # DESIGN.md assets (ERD, data flow)
├── docker-compose.yml
├── Makefile                      # Convenience targets (make up, make run-*, make ports, ...)
├── .env.example                  # copy to .env
├── DESIGN.md
├── README.md                     # this file
└── .ai/                          # Design documentation (17 files)
```

---

# Assumptions

1. One violation generates **exactly one** invoice (no installments, no partial payments).
2. The payment integration is **real Midtrans Snap** (sandbox key) — not a mock.
   The Snap UI opens in the browser and the test card / QRIS simulator
   completes the flow end-to-end. See `ARCHITECTURE_DECISION.md` ADR-012.
3. **Login uses password + bcrypt** (see ADR-006). All failure cases (email
   not found, wrong password) return the same `401 UNAUTHORIZED` response so
   the API doesn't leak which case occurred. A production system would add
   password reset, refresh tokens, MFA.
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
11. **Payment methods are configurable** via `MIDTRANS_ENABLED_METHODS`
    (comma-separated). Default: `qris,gopay`.

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

## Real Midtrans (not mock)
The original assignment asked for a mock payment provider. We went a step
further and integrated the **real Midtrans Snap sandbox** so reviewers can
see a real Snap UI open, choose GoPay/QRIS, and complete the flow. The cost
is one environment variable (`MIDTRANS_SERVER_KEY`). To revert to a mock,
set the key to `MOCK_anything` and the client returns a fake token (see
`midtrans/client.go`). See ADR-012.

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
- ✅ Makefile convenience targets (`make help` to list, `make ports` for the port map)
- ✅ `.ai/` design documentation (17 files, self-consistent)
- ✅ Seed data + migrations
- ✅ Unit tests for fine engine, rule versioning, and dotenv loader
