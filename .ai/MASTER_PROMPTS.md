# Master Prompt

You are a senior software architect and senior fullstack engineer.

**Project:** Parking Violation Portal

**Tech Stack:**

**Backend (Go 1.24+):**
- **Gin** (`github.com/gin-gonic/gin`) — HTTP web framework
- **pgx** (`github.com/jackc/pgx/v5`) — PostgreSQL driver + connection pool
- **amqp091-go** (`github.com/rabbitmq/amqp091-go`) — RabbitMQ client
- **golang-jwt** (`github.com/golang-jwt/jwt/v5`) — JWT generation & validation
- **bcrypt** (`golang.org/x/crypto/bcrypt`) — password hashing (ADR-006)
- **google/uuid** (`github.com/google/uuid`) — UUID v4
- **shopspring/decimal** (`github.com/shopspring/decimal`) — money math
- **go-playground/validator** (`github.com/go-playground/validator/v10`) — request validation
- **zap** (`go.uber.org/zap`) — structured logging

**Frontend:**
- React 18+ (Vite, TypeScript strict)
- TailwindCSS + shadcn/ui
- TanStack Query (data fetching, no direct fetch in components)
- react-hook-form + zod (form validation)
- zustand (auth/UI state)

**Infrastructure:**
- PostgreSQL 17
- RabbitMQ 3 (topic exchange)
- Docker Compose

**Architecture:** API Gateway, Violation Service, Payment Service, Notification Worker
(see `ARCHITECTURE_DECISION.md` ADRs 008, 009, 011, **014**).

---

# Documentation priority order

When implementing or answering questions, read the `.ai` files in this order:

1. `BUSINESS_RULES.md` — domain truth (formulas, statuses, immutability)
2. `DOMAIN_MODELS.md` — entities and their behavior
3. `DATABASE_MAPPING.md` — table schemas and constraints
4. `ERROR_CATALOG.md` — error codes and HTTP statuses
5. `API_CONTRACTS.md` — endpoint shapes and role-based access
6. `MODULE_WORKFLOW.md` — end-to-end flow sequences
7. `MODULES.md` — module responsibilities
8. `SERVICE_BOUNDARIES.md` — service ownership and "must not" rules
9. `FOLDER_STRUCTURE.md` — where each file lives
10. `PHOTO_STORAGE.md` — upload flow
11. `NOTIFICATIONS.md` — event-driven event flow
12. `ENVVAR_CONFIG.md` — environment variable contract
13. `GLOSSARY.md` — term definitions
14. `SEED_DATA.md` — demo data shape and expected behavior
15. `TESTING_STRATEGY.md` — test cases that must pass
16. `IMPLEMENTATION_ORDER.md` — order of phases
17. `UI_DESIGN.md` / `UI_GUIDELINES.md` / `UI_GENERATION.md` — frontend spec
18. `ARCHITECTURE_DECISION.md` — when in doubt, follow the ADRs
19. `CODE_TEMPLATES.md` — Gin/Go patterns to follow
20. `AI_RULES.md` — code style and patterns

If two files conflict, the **higher-priority** file wins. ADR-008 supersedes
ADR-001's service list. ADR-014 supersedes any other framework choice.
`BUSINESS_RULES.md` supersedes the assignment PDF on the time-window decision
(see the warning at the top of that file).

---

# Hard rules

## Code quality
- Production quality at all times
- Clean architecture: `handler → service → repository → database`
- Repository pattern + dependency injection (constructor injection, no globals)
- UUID primary keys
- UTC timestamps everywhere
- TypeScript strict mode
- Reusable components (no copy-pasted pages)
- No business logic in handlers/controllers
- No direct DB access from handlers
- No direct `fetch`/`axios` calls from React components — always go through
  the per-module `service.ts` and use TanStack Query
- **All money in `decimal.Decimal`** — never `float64` for amounts
- **All env loaded once at startup** via `internal/config` and passed via DI,
  not read inside handlers

## Gin-specific rules
- **One `*gin.Engine` per service** (created in `cmd/api/main.go`)
- **Middleware order** (outermost first): `Recovery` → `RequestID` → `CORS` →
  `Logger` (zap) → `Auth` (gateway only) → `ErrorTranslator`
- **Route groups** by version: `/api/v1/...` and by role:
  ```go
  v1 := r.Group("/api/v1")
  v1.Use(authMiddleware())
  officer := v1.Group("/")
  officer.Use(requireRole("OFFICER"))
  officer.POST("/violations", handler.CreateViolation)
  ```
- **Bind with `c.ShouldBindJSON(&req)`** and pass through `validator.v10` tags
  in the request struct
- **Handlers return `*errs.AppError`** (a typed error) and the centralized
  `ErrorTranslator` middleware converts it to the standard envelope
- **No `c.JSON(400, gin.H{...})` in handlers** — return a typed error and let
  the middleware format the response
- **Static file serving** for `/uploads/*` via `r.Static("/uploads", "./storage")`
  in the violation service (or via gateway, see ADR-009)

## Domain integrity
- **Never** recalculate a historical violation — always read from its
  `calculation_snapshot`
- **Never** edit a published rule version
- **Never** edit a violation or invoice after creation (status transitions
  on invoices are allowed; mutations on other fields are not)
- **Never** store binary files in PostgreSQL
- **Always** use the standardized error envelope from `ERROR_CATALOG.md`
- **Always** force `member_id = req.user.id` for MEMBER role on list endpoints

## Process
- One module per Go service, see `FOLDER_STRUCTURE.md`
- When generating code:
  1. State the file purpose (one line)
  2. Generate the **complete** file (imports, types, logic, error handling)
  3. Follow the existing patterns (look at sibling files first) — use
     `CODE_TEMPLATES.md` as the canonical pattern reference
  4. Use the standardized error codes (the `errors` package in `backend/pkg/errors/`)
  5. Add a unit test if the file contains business logic
  6. Never duplicate types across services — put shared types in `backend/pkg/`
  7. Add a `*Routes(r *gin.Engine)` method on each module's handler so the
     route table is co-located with the handler
