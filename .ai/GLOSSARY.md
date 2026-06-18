# Glossary

> Standard definitions for terms used across the `.ai` docs and the codebase.
> When a term is used in another doc, the meaning here is authoritative.

---

# Actor terms

- **Officer** — User with role `OFFICER`. Can create violations, manage fine
  rules, view all data, and look up members.
- **Member** — User with role `MEMBER`. Can view and pay **only their own**
  violations and invoices. Cannot read rules.

---

# Violation terms

- **Violation** — A record created by an officer documenting a parking
  infraction. Immutable after creation. Has a frozen `rule_version_id`,
  `fine_amount`, and `calculation_snapshot`.
- **`violation_timestamp`** — The actual time the violation occurred (UTC).
  This is **not** the same as `created_at` (when the officer submitted the
  record). The fine's time-multiplier is derived from `violation_timestamp`.
- **`photo_url`** — Local path to the violation photo, produced by
  `POST /uploads/violations` (see `PHOTO_STORAGE.md`).
- **License plate** — Free-text identifier (e.g. `B1234XYZ`). A single
  plate can be associated with many violations and potentially multiple
  members over time. The officer confirms which `member_id` is the current
  holder of the plate when creating a violation.

---

# Rule terms

- **Rule version** (`FineRuleVersion`) — A named, point-in-time snapshot of
  the fine calculation ruleset. Has `version_number`, `is_active`,
  `published_at`, `created_by`.
- **Active rule version** — The single rule version where `is_active = true`.
  All new violations use this version.
- **Draft rule version** — A version where `is_active = false`. Not yet used
  for any violation.
- **Rule detail** (`FineRuleDetail`) — One row per `(rule_version_id,
  violation_type)`, holding `base_amount`, `day_multiplier`, `night_multiplier`,
  `repeat_0`, `repeat_1`, `repeat_2_plus`.

---

# Fine calculation terms

- **Base amount** — IDR value for a given violation type, taken from the
  active `FineRuleDetail`.
- **Time multiplier** — `1.0` for day (06:00–21:59) or `1.5` for night
  (22:00–05:59), **half-open intervals**, evaluated against the
  `violation_timestamp`. The decision to use half-open intervals is
  documented at the top of `BUSINESS_RULES.md`.
- **Repeat multiplier** — `1.0` / `1.5` / `2.0` based on the number of
  **unpaid** invoices (`PENDING` or `FAILED`) for the same `license_plate`
  whose `violation_timestamp` falls within **90 days** of the new violation.
  Maps to the `repeat_0` / `repeat_1` / `repeat_2_plus` columns.
- **Unpaid** — Invoice status is `PENDING` or `FAILED` (i.e. not `PAID`).
  `FAILED` is still payable, so it still counts toward the repeat multiplier.
- **Final fine** — `base_amount × time_multiplier × repeat_multiplier`,
  rounded to 2 decimal places.

---

# Calculation snapshot terms

- **`calculation_snapshot`** — Immutable JSONB field on `violations`
  containing the inputs and outputs of the fine calculation at creation
  time. Used by the history view to display the breakdown and to guarantee
  historical consistency when rules change.
- **`time_window`** — String label (`DAY` or `NIGHT`) recorded in the
  snapshot for human readability.

---

# Invoice terms

- **Invoice** — Bill generated from a violation, one-to-one.
- **Invoice status** — `PENDING` | `PAID` | `FAILED` | `CANCELLED`.
  - `PENDING` — Created, not yet paid
  - `PAID` — Terminal, paid successfully
  - `FAILED` — Last payment attempt failed, still payable (member can retry)
  - `CANCELLED` — Admin-cancelled (not used in MVP)

---

# Payment terms

- **Payment** — One attempt to pay an invoice. Each attempt creates a new row
  in `payments` (in `PENDING` state) and gets a Midtrans Snap token.
- **Payment status** — `PENDING` (awaiting Midtrans confirmation),
  `PAID` (settled), `FAILED` (rejected/expired/cancelled by Midtrans).
- **Midtrans Snap** — Midtrans's single-integration payment UI. The frontend
  embeds Snap JS and calls `window.snap.pay(snap_token)` to open it. See
  ADR-012.
- **Snap token** — A short-lived token returned by Midtrans `/snap/v1/transactions`,
  used by the frontend to open the Snap payment page.
- **Order ID** — Midtrans's unique identifier for a transaction. We generate
  it server-side (e.g. `ORDER-<uuid>`) and store it in `midtrans_order_id`.
- **Midtrans transaction status** — `capture` / `settlement` / `pending` /
  `deny` / `cancel` / `expire` / `refund`. Mapped to our `PAID` / `PENDING` /
  `FAILED` enum in the service layer.
- **Webhook (notification)** — Midtrans POSTs to our `MIDTRANS_NOTIFICATION_URL`
  when a transaction status changes. We verify with Midtrans before updating
  the DB (the webhook body is a hint, not the source of truth).
- **payment_method** — GoPay, QRIS, etc. Set after the webhook confirms the
  actual method the member chose.
- **MIDTRANS_ENABLED_METHODS** — Env var (comma-separated) listing the payment
  methods the Snap UI may offer. Set in `.env.example` to `qris,gopay`.

---

# Authentication terms

- **JWT** — JSON Web Token issued on login. Carries `sub` (user id) and
  `role`. Validated by the API Gateway on every request.
- **bcrypt** — Password hashing algorithm used for `users.password_hash`
  (golang.org/x/crypto/bcrypt, `DefaultCost`). See ADR-006.
- **Password auth** — Login takes `email` + `password`. The service looks up
  the user by email, compares the bcrypt hash, and returns a JWT on success.
  All failure cases (email not found, wrong password, missing hash) return
  the same `UNAUTHORIZED` message to avoid leaking which case occurred.
- **Demo password** — `password123` — the default password for the 3 seeded
  users. NEVER use in production. See `SEED_DATA.md`.

---

# Service terms

- **API Gateway** — Single HTTP entrypoint at port `8080`. Validates JWT,
  routes by URL prefix, applies the error envelope. See ADR-009.
- **Violation Service** — Backend service owning violations, invoices, rules,
  fine-rule-details, and the photo upload handler.
- **Payment Service** — Backend service owning the `payments` table and the
  mock payment provider.
- **Notification Worker** — Consumer-only worker that subscribes to all
  events from RabbitMQ and logs/persists them. See `NOTIFICATIONS.md` and ADR-008.
- **Best-effort event** — RabbitMQ publishes happen **after** the DB commit.
  If the publish fails, the request still succeeds and the failure is logged.
  See ADR-011.

---

# Status enums (canonical, uppercase)

| Entity         | Allowed values                          |
| -------------- | --------------------------------------- |
| User role      | `OFFICER`, `MEMBER`                     |
| Invoice status | `PENDING`, `PAID`, `FAILED`, `CANCELLED`|
| Payment status | `PAID`, `FAILED`                        |
| Payment scenario | `success`, `failed`                   |
| Violation type | `expired_meter`, `no_parking_zone`, `blocking_hydrant`, `disabled_spot` |
| Time window    | `DAY`, `NIGHT`                          |

---

# Error code terms

- **Error code** — Machine-readable string in the standardized error envelope.
  See `ERROR_CATALOG.md` for the full list. Examples: `INVOICE_NOT_FOUND`,
  `INVOICE_ALREADY_PAID`, `FORBIDDEN`, `VALIDATION_ERROR`.
- **Error envelope** — `{ success: false, error: { code, message, details? } }`
  used for all 4xx/5xx responses.

---

# File / format terms

- **DTO** — Data Transfer Object. A typed struct used to cross a service or
  process boundary. See `backend/pkg/`.
- **Snapshot** — Frozen record of how something was computed or what its
  inputs were at a given moment. The `calculation_snapshot` is the
  assignment's central example.
- **Half-open interval** — `[start, end)` — `start` is included, `end` is
  excluded. Used for the time-window decision in `BUSINESS_RULES.md`.
