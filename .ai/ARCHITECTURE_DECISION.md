# Architecture Decision Records

---

# ADR-001

**Title:** Use Modular Services Instead of Monolith

**Status:** Accepted

**Reason:** Assignment explicitly requires modular services.

**Services:**
- API Gateway
- Violation Service
- Payment Service
- Notification Worker

---

# ADR-002

**Title:** Use PostgreSQL

**Status:** Accepted

**Reason:** System requires transactions, relational data, rule versioning, and historical records. PostgreSQL covers all of these with strong consistency.

---

# ADR-003

**Title:** Use RabbitMQ

**Status:** Accepted

**Reason:** Support asynchronous event propagation between services.

**Events:** `ViolationCreated`, `InvoiceCreated`, `PaymentSucceeded`, `PaymentFailed`, `RulePublished`.

RabbitMQ is **not** used for core business transactions (synchronous DB writes are
the source of truth). It is used only for downstream event propagation
(notifications, audit, future integrations).

---

# ADR-004

**Title:** Store Calculation Snapshot

**Status:** Accepted

**Reason:** Rule changes must not affect historical violations.

Each violation stores:
- `rule_version_id`
- `calculation_snapshot` (JSONB breakdown)

This guarantees historical consistency.

---

# ADR-005

**Title:** One Invoice Per Violation

**Status:** Accepted

**Reason:** Assignment does not require installments or partial payments. Simplifies design.

---

# ADR-006

**Title:** Password-Based Authentication (bcrypt)

**Status:** Accepted (revised from "mock email-only" → "password + bcrypt")

**Reason:** Real password authentication is needed for the assignment to
demonstrate a complete login flow. The login endpoint accepts `email` and
`password`; the password is stored as a bcrypt hash (`golang.org/x/crypto/bcrypt`,
`DefaultCost`). JWT is signed on success. All failure cases collapse to
`UNAUTHORIZED` ("invalid email or password") to avoid leaking whether the
email exists.

**Password rules (basic):**
- min 8 characters (recommended; current validator is `min=1` for the demo)
- bcrypt hash stored in `users.password_hash` (VARCHAR(255))
- comparison via `bcrypt.CompareHashAndPassword`
- default seed password is `password123` (see `SEED_DATA.md`)
- demo accounts:
  - `officer@example.com` / `password123` (OFFICER)
  - `member@example.com`  / `password123` (MEMBER)
  - `member2@example.com` / `password123` (MEMBER)

---

# ADR-007

**Title:** Use Repository Pattern

**Status:** Accepted

**Reason:** Improves testability, maintainability, and separation of concerns.

**Architecture:** `Handler → Service → Repository → Database`

---

# ADR-008

**Title:** Add Notification Worker as a Separate Service

**Status:** Accepted

**Reason:** The original ADR-001 listed only 3 services. After expanding the design to include event-driven notification logging, the Notification Worker is introduced as a 4th service. It is a **consumer-only** worker that subscribes to all events from RabbitMQ, logs them, and optionally writes to a `notifications` table. It never modifies business data.

**Updated services list** (replaces ADR-001's list):
- API Gateway (HTTP entrypoint, routing, auth, see ADR-009)
- Violation Service (owns violations, invoices, rules, fine-rule-details)
- Payment Service (owns payments, mock provider)
- Notification Worker (consumer-only, owns notifications, processed_events)

---

# ADR-009

**Title:** API Gateway as a Single HTTP Entrypoint

**Status:** Accepted

**Reason:** The assignment requires a single entrypoint between frontend and backend. The API Gateway is a lightweight Go HTTP service that:
- validates JWT
- extracts `user_id` and `role`
- routes requests to the appropriate backend service by URL prefix:
  - `/api/v1/auth/*` → handled by the gateway directly (login)
  - `/api/v1/uploads/*`, `/api/v1/violations/*`, `/api/v1/invoices/*`, `/api/v1/rules/*`, `/api/v1/members/*` → forward to Violation Service
  - `/api/v1/payments/*` → forward to Payment Service
  - `/api/v1/history/*` → can be served by either service; routed to Violation Service
  - `/uploads/*` → static file serving (see PHOTO_STORAGE.md)
- applies the standardized error envelope from `ERROR_CATALOG.md`

The gateway does **not** call the database directly and does **not** perform fine
calculations. It is the only service exposed to the frontend (port `8080`).

---

# ADR-010

**Title:** Local Filesystem for Photo Storage

**Status:** Accepted

**Reason:** The assignment scope is small (5 flows, slice). Object storage (S3, MinIO)
adds infrastructure that is not justified by the assignment. The `POST /uploads/violations`
endpoint writes to a local `storage/violations/` directory, which is mounted as a
Docker volume in `docker-compose.yml`. See `PHOTO_STORAGE.md` for the full design.

Future migration to S3/MinIO is planned but out of scope.

---

# ADR-011

**Title:** Synchronous HTTP for Core Writes, Asynchronous Events for Side Effects

**Status:** Accepted

**Reason:** The 5 assignment flows require predictable, transactional behavior
(violation + invoice creation, payment + invoice status update). These are all
done **synchronously** over HTTP within a single DB transaction. RabbitMQ events
are **published after the DB commit** as side effects for downstream consumers
(notifications, future analytics). If RabbitMQ is unavailable, the core flow
still succeeds — events are best-effort. See `MODULE_WORKFLOW.md` for the
sequence diagram.

---

# ADR-012

**Title:** Mocked Payment Provider, In-Process

**Status:** Accepted

**Reason:** The assignment explicitly says "Mock response from payment provider in
internal service." The `PaymentService.charge(invoice_id, amount, scenario)` function
is implemented as a Go function inside the Payment Service. It returns
`{ status, transaction_id }` based on the `scenario` input. There is no external
HTTP call. This keeps the slice fully runnable without external dependencies.

---

# ADR-013

**Title:** Inactive Infrastructure

**Status:** Accepted

**Reason:** The following technologies are **intentionally not** part of this
implementation:
- Redis (no caching needed at this scale; rule lookups are cheap SQL queries)
- MinIO / S3 (see ADR-010)
- Kubernetes / orchestration (Docker Compose is sufficient)
- gRPC (REST over HTTP is enough for the slice)

This is to keep the surface area small and focused on the 5 assignment flows.

---

# ADR-014

**Title:** Use Gin (Go) as the Backend HTTP Framework

**Status:** Accepted

**Reason:** All four backend services (gateway, violation, payment, worker is
not HTTP) use **Gin** (`github.com/gin-gonic/gin`) as the HTTP framework.

Gin was chosen because:
- Mature, battle-tested, used by most Go shops for REST APIs
- Excellent middleware ecosystem (CORS, recovery, logging, request-id)
- Native support for `*http.Request` context propagation (we use
  `c.Request.Context()` for pgx/RabbitMQ calls)
- Low overhead, fast router
- Plays well with `validator.v10` for request validation
- Plays well with the typed-error + middleware-translator pattern from
  `CODE_TEMPLATES.md`

**Stack confirmed:**
- Router: `github.com/gin-gonic/gin`
- Postgres: `github.com/jackc/pgx/v5` + `pgxpool`
- AMQP: `github.com/rabbitmq/amqp091-go`
- JWT: `github.com/golang-jwt/jwt/v5`
- UUID: `github.com/google/uuid`
- Money: `github.com/shopspring/decimal`
- Validation: `github.com/go-playground/validator/v10`
- Logging: `go.uber.org/zap`

**Conventions** (full examples in `CODE_TEMPLATES.md`):
- One `*gin.Engine` per service, created in `cmd/api/main.go`
- Standard middleware chain: `Recovery → RequestID → CORS → Logger → Auth → ErrorTranslator`
- Route groups by version and role
- Handlers panic on typed `*errs.AppError`; middleware writes the response
- No raw `c.JSON(4xx, gin.H{...})` in handlers

**Not used:**
- Echo, Fiber, Chi, net/http (would force re-implementing middleware we get for free in Gin)
