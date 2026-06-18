# AI Rules

> These rules apply whenever you (the AI) generate or modify code in this
> repository. They are a subset of `MASTER_PROMPTS.md` focused on rules a
> model can apply mechanically.

---

# Authority order (most → least)

1. `BUSINESS_RULES.md`
2. `DATABASE_MAPPING.md`
3. `ERROR_CATALOG.md`
4. `API_CONTRACTS.md`
5. `DOMAIN_MODELS.md`
6. `MODULE_WORKFLOW.md`
7. `SERVICE_BOUNDARIES.md`
8. `MODULES.md`
9. `FOLDER_STRUCTURE.md`
10. `ARCHITECTURE_DECISION.md` (including ADR-014: Gin framework)
11. `CODE_TEMPLATES.md` (canonical Gin/Go patterns)
12. everything else under `.ai/`

ADR-008 supersedes ADR-001's service list. **ADR-014 supersedes any other
backend framework choice** — all HTTP services use **Gin**.
`BUSINESS_RULES.md` supersedes the assignment PDF on the time-window decision
(see the warning at the top of that file).

If a request asks you to break one of these rules, **refuse and explain**.

---

# Hard rules

- **Always** follow `BUSINESS_RULES.md`
- **Never** invent fields not defined in `DATABASE_MAPPING.md`
- **Never** bypass module boundaries (see `SERVICE_BOUNDARIES.md` and
  `MODULES.md` for "must not" lists)
- **Always** go through the service layer
  - backend: `Handler → Service → Repository → DB`
  - frontend: `Page → Hook → service.ts → API client`
- **Never** put business logic in handlers/controllers
- **Never** access the database directly from handlers
- **Never** recalculate historical violations — always read from
  `violation.calculation_snapshot`
- **Always** use the standardized error envelope from `ERROR_CATALOG.md`
- **Always** force `member_id = req.user.id` for MEMBER role on list endpoints
- **Always** use **Gin** for HTTP routing (no Echo, Fiber, Chi, net/http)
- **Always** use **pgx/v5 + pgxpool** for PostgreSQL
- **Always** use **shopspring/decimal** for money (never `float64`)
- **Always** use **bcrypt** for password hashing (never plaintext, never MD5/SHA)
- **Always** return `UNAUTHORIZED` (not `NOT_FOUND` vs `FORBIDDEN`) for both
  missing-email and wrong-password — don't leak which case occurred
- **Never** log or echo back the password hash or plaintext password
- **Always** use the patterns in `CODE_TEMPLATES.md` — if a file doesn't
  match, the file is wrong, not the pattern

---

# Conventions

All timestamps: **UTC**, ISO 8601.

All IDs: **UUID** (v4).

All API responses:
- success: `{ success: true, data: ..., message?: string }`
- error:   `{ success: false, error: { code, message, details? } }`
  (see `ERROR_CATALOG.md` for the list of `code` values)

All money: **decimal.Decimal** — IDR, no subunits. Never `float64`.

All enums (status, role, scenario, etc.) are **uppercase** strings.

All time-multiplier decisions use the **half-open** intervals from
`BUSINESS_RULES.md` (06:00–21:59 day, 22:00–05:59 night). Do not implement
the literal PDF intervals.

---

# Gin-specific conventions (from ADR-014 and CODE_TEMPLATES.md)

- **One `*gin.Engine` per service**, created in `cmd/api/main.go`
- **Middleware order** (outermost first): `Recovery` → `RequestID` → `CORS` →
  `Logger` (zap) → `Auth` (gateway only) → `ErrorTranslator`
- **Route groups** by version (`/api/v1`) and by role (`officer`, `member`)
- **Bind with `c.ShouldBindJSON(&req)`** — never `Bind`
- **Handlers panic on typed `*errs.AppError`** — never call
  `c.JSON(4xx, gin.H{...})` directly. The `ErrorTranslator` middleware
  formats the response.
- **Use `c.Request.Context()`** for downstream calls
- **Static `/uploads/*`** served via `r.Static("/uploads", "./storage")` in
  the violation service
- **One `Register(*gin.RouterGroup)` method per module's handler** so the
  route table is co-located with the handler

---

# File-creation rules

- One module = one Go package under `internal/<module>/`
- One feature = one folder under `frontend/src/modules/<feature>/`
- New shared types go in `backend/pkg/`, not duplicated
- New error codes go in `ERROR_CATALOG.md` first, then `pkg/errs`, then the
  handlers — not the other way around
- Migrations are append-only, never edited after merge
- Never delete a test
- `cmd/api/main.go` wires the service: config → logger → db → gin engine →
  middleware → register modules → graceful shutdown
