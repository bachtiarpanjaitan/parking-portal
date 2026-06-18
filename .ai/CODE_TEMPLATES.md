# Code Templates

> **Canonical Go patterns** for this project. All Go services use **Gin** as
> the HTTP framework (see `ARCHITECTURE_DECISION.md` ADR-014). Frontend uses
> Vite + React + TanStack Query.
>
> If a generated file doesn't match these patterns, it is **wrong** — fix the
> file, not the pattern.

---

# Backend Flow

```
HTTP request
    ↓
Gin middleware  (RequestID → Logger → Auth → ErrorTranslator)
    ↓
Handler         (bind + validate + call service + return)
    ↓
Service         (business logic)
    ↓
Repository      (DB / RabbitMQ)
    ↓
PostgreSQL / RabbitMQ
```

The `ErrorTranslator` middleware catches `*errs.AppError` returned from any
layer and writes the standardized envelope from `ERROR_CATALOG.md`. Handlers
**must not** call `c.JSON(4xx, ...)` directly.

---

# Go module layout

```go
// internal/<module>/
//   handler.go        // gin handlers + Routes(r *gin.RouterGroup)
//   service.go        // business logic
//   repository.go     // pgx queries
//   model.go          // domain types for this module
//   dto.go            // request/response DTOs
//   handler_test.go   // gin handler tests with httptest
//   service_test.go   // service unit tests
```

---

# Gin engine setup (cmd/api/main.go)

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "go.uber.org/zap"

    "<module>/internal/config"
    "<module>/internal/database"
    "<module>/internal/violations"
    "<module>/internal/invoices"
    "<module>/internal/rules"
    "<module>/internal/middleware"
    "pkg/errs"
    "pkg/httpx"
    "pkg/logger"
)

func main() {
    cfg, err := config.Load()
    if err != nil { log.Fatal(err) }

    log, _ := logger.New(cfg.AppEnv)
    defer log.Sync()

    db, err := database.NewPostgres(cfg.DatabaseURL, log)
    if err != nil { log.Fatal("db connect", zap.Error(err)) }
    defer db.Close()

    r := gin.New()
    r.Use(
        middleware.Recovery(log),
        middleware.RequestID(),
        middleware.CORS(),
        middleware.Logger(log),
        middleware.ErrorTranslator(errs.Map),  // converts *errs.AppError → envelope
    )

    r.GET("/healthz", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })

    v1 := r.Group("/api/v1")

    // Mount modules
    violations.Register(v1, violations.Deps{
        Service: violations.NewService(db, log),
        Log:     log,
    })
    invoices.Register(v1, invoices.Deps{...})
    rules.Register(v1, rules.Deps{...})

    srv := &http.Server{Addr: ":" + cfg.AppPort, Handler: r}
    go func() { srv.ListenAndServe() }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    srv.Shutdown(ctx)
}
```

---

# Repository interface

```go
package violations

import (
    "context"

    "github.com/google/uuid"
)

type Repository interface {
    Create(ctx context.Context, v *Violation) error
    FindByID(ctx context.Context, id uuid.UUID) (*Violation, error)
    List(ctx context.Context, filter ListFilter) ([]Violation, int, error)
}
```

Conventions:
- One interface per module, defined where it is **consumed** (in `service.go`),
  not where it is implemented
- Return the row count separately for paginated list methods
- All methods take `context.Context` as the first parameter
- All IDs are `uuid.UUID`, never `string`

---

# Service

```go
package violations

type Service interface {
    Create(ctx context.Context, req CreateRequest) (*Violation, error)
    List(ctx context.Context, f ListFilter) ([]Violation, int, error)
    GetByID(ctx context.Context, id uuid.UUID) (*Violation, error)
}

type service struct {
    repo Repository
    rules *rules.Service      // fine engine lives in rules module
    inv   *invoices.Service
    log   *zap.Logger
}

func NewService(repo Repository, rules *rules.Service, inv *invoices.Service, log *zap.Logger) Service {
    return &service{repo: repo, rules: rules, inv: inv, log: log}
}
```

Business logic only. The service returns `*errs.AppError` for known failure
cases (validation, not-found, conflict, etc.) and a plain `error` for
unexpected failures. The handler does not see the difference — the
`ErrorTranslator` middleware does.

---

# Gin handler

```go
package violations

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "go.uber.org/zap"

    "pkg/errs"
    "pkg/httpx"
)

type Handler struct {
    svc Service
    log *zap.Logger
}

func NewHandler(svc Service, log *zap.Logger) *Handler {
    return &Handler{svc: svc, log: log}
}

// Register attaches this module's routes to the given group.
func (h *Handler) Register(rg *gin.RouterGroup, auth gin.HandlerFunc) {
    g := rg.Group("/violations")
    g.Use(auth)  // JWT validation from gateway (or local in dev)

    g.POST("", h.Create)
    g.GET("", h.List)
    g.GET("/:id", h.GetByID)
}

func (h *Handler) Create(c *gin.Context) {
    var req CreateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        panic(errs.New(errs.CodeValidation, "invalid request body", err))
    }
    // role check
    role, _ := c.Get("role")
    if role != "OFFICER" {
        panic(errs.New(errs.CodeForbidden, "officer role required"))
    }

    v, err := h.svc.Create(c.Request.Context(), req)
    if err != nil { panic(err) }  // ErrorTranslator handles the rest

    c.JSON(http.StatusCreated, httpx.OK(v))
}
```

Conventions:
- One `Register` method per handler that wires all routes for the module
- Use `c.ShouldBindJSON` (not `Bind`) so a failed bind can be converted
  to our error type
- **Panic on errors** — the recovery middleware + error translator catches
  the typed error and writes the response. This keeps handler code straight-line.
- Use `c.Request.Context()` (Gin's request context) for downstream calls
- `httpx.OK(data)` wraps the success envelope `{ success: true, data }`

---

# Request DTO with validator

```go
type CreateRequest struct {
    MemberID           uuid.UUID `json:"member_id" validate:"required"`
    LicensePlate       string    `json:"license_plate" validate:"required,min=1,max=20"`
    ViolationType      string    `json:"violation_type" validate:"required,oneof=expired_meter no_parking_zone blocking_hydrant disabled_spot"`
    Location           string    `json:"location" validate:"required,max=255"`
    ViolationTimestamp time.Time `json:"violation_timestamp" validate:"required"`
    PhotoURL           string    `json:"photo_url" validate:"required,url|startswith=/uploads"`
}
```

Validation runs in the service layer (so the same rules apply to non-HTTP
callers like the seeder), and any `validator.ValidationErrors` is converted
to `errs.CodeValidation` with `details` populated.

---

# Standardized error envelope

The `pkg/errs` package defines:

```go
package errs

type Code string

const (
    CodeValidation          Code = "VALIDATION_ERROR"
    CodeUnauthorized        Code = "UNAUTHORIZED"
    CodeInvalidToken        Code = "INVALID_TOKEN"
    CodeTokenExpired        Code = "TOKEN_EXPIRED"
    CodeForbidden           Code = "FORBIDDEN"
    CodeNotFound            Code = "RESOURCE_NOT_FOUND"
    CodeViolationNotFound   Code = "VIOLATION_NOT_FOUND"
    CodeInvoiceNotFound     Code = "INVOICE_NOT_FOUND"
    CodeInvoiceAlreadyPaid  Code = "INVOICE_ALREADY_PAID"
    CodeRuleNotFound        Code = "RULE_VERSION_NOT_FOUND"
    CodeRuleAlreadyActive   Code = "RULE_ALREADY_ACTIVE"
    CodeNoActiveRule        Code = "NO_ACTIVE_RULE"
    CodePaymentFailed       Code = "PAYMENT_FAILED"
    CodeInvalidScenario     Code = "INVALID_PAYMENT_SCENARIO"
    CodeFileRequired        Code = "FILE_REQUIRED"
    CodeFileTooLarge        Code = "FILE_TOO_LARGE"
    CodeInvalidFileType     Code = "INVALID_FILE_TYPE"
    CodeFileUploadFailed    Code = "FILE_UPLOAD_FAILED"
    CodeInternal            Code = "INTERNAL_SERVER_ERROR"
)

// AppError is a typed error that the ErrorTranslator middleware can
// inspect to produce a standardized JSON response.
type AppError struct {
    HTTPStatus int            `json:"-"`
    Code       Code           `json:"code"`
    Message    string         `json:"message"`
    Details    map[string]any `json:"details,omitempty"`
}

func (e *AppError) Error() string { return string(e.Code) + ": " + e.Message }

func New(code Code, msg string, details ...map[string]any) *AppError { /* ... */ }
```

---

# Postgres repository (pgx)

```go
package violations

import (
    "context"
    "errors"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"

    "pkg/errs"
)

type pgRepository struct {
    db *pgxpool.Pool
}

func NewPGRepository(db *pgxpool.Pool) Repository {
    return &pgRepository{db: db}
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Violation, error) {
    const q = `
        SELECT id, member_id, rule_version_id, license_plate, violation_type,
               location, violation_timestamp, photo_url, fine_amount,
               calculation_snapshot, created_at, updated_at
        FROM violations WHERE id = $1
    `
    var v Violation
    err := r.db.QueryRow(ctx, q, id).Scan(
        &v.ID, &v.MemberID, &v.RuleVersionID, &v.LicensePlate,
        &v.ViolationType, &v.Location, &v.ViolationTimestamp,
        &v.PhotoURL, &v.FineAmount, &v.CalculationSnapshot,
        &v.CreatedAt, &v.UpdatedAt,
    )
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, errs.New(errs.CodeViolationNotFound, "violation not found")
    }
    if err != nil { return nil, err }
    return &v, nil
}
```

Conventions:
- Use `pgxpool.Pool` injected via constructor
- Translate `pgx.ErrNoRows` to the matching `AppError` code in the repo
- Never let raw SQL errors leak past the service layer

---

# RabbitMQ publisher

```go
package events

type Publisher interface {
    Publish(ctx context.Context, routingKey string, event Event) error
}

type amqpPublisher struct {
    ch *amqp091.Channel
    exchange string
}

func (p *amqpPublisher) Publish(ctx context.Context, key string, e Event) error {
    body, _ := json.Marshal(e)
    return p.ch.PublishWithContext(ctx,
        p.exchange,
        key,
        false, false,
        amqp091.Publishing{
            ContentType:  "application/json",
            DeliveryMode: amqp091.Persistent,
            Body:         body,
        },
    )
}
```

Conventions:
- Publishes happen **after** the DB commit, **best-effort** (see ADR-011)
- If publish fails, log + return a `WARN` to the caller; do **not** roll back
- Use `Persistent` delivery mode for at-least-once

---

# Frontend module

```
src/modules/<feature>/
├── pages/                # route components (thin)
│   ├── ListPage.tsx
│   └── CreatePage.tsx
├── components/           # feature-only UI
├── hooks/                # useXxx() — TanStack Query wrappers
│   ├── useList.ts
│   └── useCreate.ts
├── service.ts            # the ONLY place that calls axios
└── types.ts              # request/response types
```

Flow:

```ts
// service.ts — the only axios caller for this feature
import { api } from "@/lib/api";
import type { CreatePayload, Violation } from "./types";

export const violationsService = {
  list: (params?: ListParams) =>
    api.get<{ success: true; data: { items: Violation[]; total: number } }>(
      "/violations",
      { params }
    ).then(r => r.data.data),

  create: (payload: CreatePayload) =>
    api.post<{ success: true; data: Violation }>("/violations", payload)
      .then(r => r.data.data),
};

// hooks/useList.ts
import { useQuery } from "@tanstack/react-query";
import { violationsService } from "../service";

export const useViolations = (params?: ListParams) =>
  useQuery({
    queryKey: ["violations", params],
    queryFn: () => violationsService.list(params),
  });

// pages/ListPage.tsx
export default function ViolationListPage() {
  const { data, isLoading, error } = useViolations();
  if (isLoading) return <Spinner />;
  if (error) return <ErrorState error={error} />;
  return <DataTable columns={cols} data={data!.items} />;
}
```

Conventions:
- **Never** call `axios` or `fetch` directly from a component, page, or hook —
  always go through `service.ts`
- **Never** use `useEffect` to fetch data — use TanStack Query
- Pages are thin (just layout + data hook + component composition)
- Reusable UI lives in `src/components/`, not in `src/modules/<feature>/components/`
  (unless it is feature-specific and not reusable)
