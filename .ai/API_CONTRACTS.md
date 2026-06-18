# API Contracts

**Base URL:** `/api/v1`

**Auth:** all endpoints except `POST /auth/login` require `Authorization: Bearer <jwt>`.
The JWT carries `{ sub: user_id, role: OFFICER|MEMBER, exp }`.

**Success response shape:**
```json
{ "success": true, "data": { ... }, "message": "optional" }
```

**Error response shape** (see `ERROR_CATALOG.md`):
```json
{
  "success": false,
  "error": {
    "code": "INVOICE_NOT_FOUND",
    "message": "Invoice not found",
    "details": { "field": ["..."] }
  }
}
```

**Status enums (consistent across all endpoints):**
- Invoice: `PENDING` | `PAID` | `FAILED` | `CANCELLED`
- Payment: `PAID` | `FAILED`
- Rule version: `is_active: true|false`

**Role-based access** (enforced in middleware; see `MODULES.md`):
- `OFFICER` → can read/write violations, rules; read all data
- `MEMBER` → can read **only their own** violations/invoices/payments; cannot read rules

---

# Authentication

## POST /auth/login

> Mocked auth — pass an email of an existing user in `users` table. No password.

**Request:**
```json
{ "email": "officer@example.com" }
```

**Response 200:**
```json
{
  "success": true,
  "data": {
    "token": "jwt_token",
    "user": { "id": "uuid", "name": "Officer", "role": "OFFICER" }
  }
}
```

**Errors:** `RESOURCE_NOT_FOUND` (unknown email), `VALIDATION_ERROR`.

---

# Uploads (see PHOTO_STORAGE.md)

## POST /uploads/violations

> **OFFICER only.** Multipart form upload of a violation photo.

**Request:** `multipart/form-data` with field `file` (image, max 5 MB,
allowed: jpg, jpeg, png, webp).

**Response 201:**
```json
{
  "success": true,
  "data": {
    "file_name": "550e8400-e29b-41d4-a716-446655440000.jpg",
    "photo_url": "/uploads/violations/550e8400-e29b-41d4-a716-446655440000.jpg"
  }
}
```

**Errors:** `FILE_REQUIRED`, `FILE_TOO_LARGE`, `INVALID_FILE_TYPE`,
`FILE_UPLOAD_FAILED`, `FORBIDDEN`.

The returned `photo_url` is then passed to `POST /violations`.

---

# Rule Management (OFFICER only, except GET for member's own history)

## GET /rules

List all rule versions (newest first).

**Response 200:**
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "version_number": 1,
      "is_active": true,
      "published_at": "2026-01-01T00:00:00Z",
      "created_by": "uuid"
    }
  ]
}
```

---

## GET /rules/{id}

Get one rule version with all its `FineRuleDetail` rows.

**Response 200:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "version_number": 1,
    "is_active": true,
    "published_at": "2026-01-01T00:00:00Z",
    "details": [
      {
        "id": "uuid",
        "violation_type": "expired_meter",
        "base_amount": 50000,
        "day_multiplier": 1.0,
        "night_multiplier": 1.5,
        "repeat_0": 1.0,
        "repeat_1": 1.5,
        "repeat_2_plus": 2.0
      }
    ]
  }
}
```

**Errors:** `RULE_VERSION_NOT_FOUND`.

---

## GET /rules/active

Get the currently active rule version (used internally by service layer
and exposed for transparency).

**Response 200:** same shape as `GET /rules/{id}`.

**Errors:** `NO_ACTIVE_RULE` (if no version is published yet — should not happen after seed).

---

## POST /rules

Create a new rule version in **draft** state. It is **not active** until published.

**Request:**
```json
{
  "rules": [
    {
      "violation_type": "expired_meter",
      "base_amount": 60000,
      "day_multiplier": 1.0,
      "night_multiplier": 1.5,
      "repeat_0": 1.0,
      "repeat_1": 1.5,
      "repeat_2_plus": 2.0
    }
  ]
}
```

**Response 201:**
```json
{
  "success": true,
  "data": { "id": "uuid", "version_number": 2, "is_active": false }
}
```

**Errors:** `VALIDATION_ERROR`, `INVALID_VIOLATION_TYPE`, `FORBIDDEN`.

---

## POST /rules/{id}/publish

Publish a draft rule. This atomically:
1. Sets the current active version to `is_active = false`
2. Sets `{id}` to `is_active = true` and `published_at = now()`

**Response 200:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "version_number": 2,
    "is_active": true,
    "published_at": "2026-06-18T10:00:00Z"
  },
  "message": "Rule published"
}
```

**Errors:** `RULE_VERSION_NOT_FOUND`, `RULE_ALREADY_ACTIVE`, `FORBIDDEN`.

> Existing violations are NOT touched (their snapshot is frozen). This is the
> core "rule versioning" guarantee from the assignment.

---

# Violations

## POST /violations (OFFICER only)

Create a violation. Server auto-calculates the fine using the active rule.

**Request:**
```json
{
  "member_id": "uuid",
  "license_plate": "B1234XYZ",
  "violation_type": "no_parking_zone",
  "location": "Jakarta",
  "violation_timestamp": "2026-01-01T10:00:00Z",
  "photo_url": "/uploads/violations/abc.jpg"
}
```

**Response 201:**
```json
{
  "success": true,
  "data": {
    "violation_id": "uuid",
    "rule_version_id": "uuid",
    "rule_version_number": 1,
    "invoice_id": "uuid",
    "fine_amount": 150000,
    "calculation_snapshot": {
      "rule_version_id": "uuid",
      "rule_version_number": 1,
      "violation_type": "no_parking_zone",
      "base_amount": 150000,
      "time_multiplier": 1.0,
      "time_window": "DAY",
      "repeat_multiplier": 1.0,
      "prior_unpaid_count": 0,
      "calculated_fine": 150000,
      "calculated_at": "2026-01-01T10:00:00Z"
    }
  }
}
```

**Errors:** `VALIDATION_ERROR`, `LICENSE_PLATE_REQUIRED`, `PHOTO_REQUIRED`,
`INVALID_VIOLATION_TYPE`, `NO_ACTIVE_RULE`, `FORBIDDEN`.

---

## GET /violations

List violations.

**Query params (all optional):**
- `member_id` (uuid) — filter by member; **for `MEMBER` role, server forces this
  to `req.user.id` regardless of what the client sends** (security)
- `license_plate` (string) — exact match
- `from`, `to` (ISO 8601) — filter by `violation_timestamp`
- `page` (int, default 1), `page_size` (int, default 20, max 100)
- `sort` (one of `violation_timestamp`, `created_at`, `fine_amount`)
- `order` (`asc` | `desc`, default `desc`)

**Response 200:**
```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "uuid",
        "member_id": "uuid",
        "rule_version_id": "uuid",
        "rule_version_number": 1,
        "license_plate": "B1234XYZ",
        "violation_type": "no_parking_zone",
        "location": "Jakarta",
        "violation_timestamp": "2026-01-01T10:00:00Z",
        "photo_url": "/uploads/violations/abc.jpg",
        "fine_amount": 150000,
        "invoice_id": "uuid",
        "invoice_status": "PENDING",
        "created_at": "2026-01-01T10:00:00Z"
      }
    ],
    "page": 1,
    "page_size": 20,
    "total": 1
  }
}
```

**Errors:** `VALIDATION_ERROR`, `FORBIDDEN` (only if MEMBER tries to query another member).

---

## GET /violations/{id}

Get one violation with full detail.

**Response 200:** same shape as list item, plus `calculation_snapshot`.

**Errors:** `VIOLATION_NOT_FOUND`, `FORBIDDEN`.

---

# Invoices

## GET /invoices

List invoices.

**Query params:**
- `member_id` (uuid) — **forced to `req.user.id` for MEMBER**
- `status` (`PENDING` | `PAID` | `FAILED` | `CANCELLED`)
- `from`, `to`, `page`, `page_size`, `sort`, `order` (same conventions as violations)

**Response 200:**
```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "uuid",
        "violation_id": "uuid",
        "member_id": "uuid",
        "amount": 150000,
        "status": "PENDING",
        "created_at": "2026-01-01T10:00:00Z",
        "updated_at": "2026-01-01T10:00:00Z"
      }
    ],
    "page": 1,
    "page_size": 20,
    "total": 1
  }
}
```

**Errors:** `VALIDATION_ERROR`, `FORBIDDEN`.

---

## GET /invoices/{id}

Get one invoice with the latest payment info.

**Response 200:** invoice shape + `latest_payment: { id, status, transaction_id, scenario, created_at }`.

**Errors:** `INVOICE_NOT_FOUND`, `FORBIDDEN`.

---

# Payments

## POST /payments

> MEMBER can only pay **their own** invoices. Server checks `invoice.member_id == req.user.id`.

**Request:**
```json
{
  "invoice_id": "uuid",
  "scenario": "success"
}
```

`scenario` is `success` or `failed` (test-only, passed to the mock provider).

**Response 200 (success path):**
```json
{
  "success": true,
  "data": {
    "payment_id": "uuid",
    "transaction_id": "trx_123",
    "status": "PAID",
    "invoice": {
      "id": "uuid",
      "status": "PAID",
      "amount": 150000
    }
  }
}
```

**Response 200 (failed path) — still HTTP 200, the failure is in the body:**
```json
{
  "success": true,
  "data": {
    "payment_id": "uuid",
    "transaction_id": "trx_456",
    "status": "FAILED",
    "invoice": {
      "id": "uuid",
      "status": "FAILED",
      "amount": 150000
    }
  },
  "message": "Payment failed"
}
```

> Note: the assignment does not require a 4xx for provider-declined payments.
> The provider's failure is reflected in the body's `status: "FAILED"`. The
> HTTP status is 200 because the request itself was processed.

**Errors:** `INVOICE_NOT_FOUND`, `INVOICE_ALREADY_PAID`, `INVALID_PAYMENT_SCENARIO`,
`FORBIDDEN`, `PAYMENT_FAILED` (only for unexpected provider error, not the
mock `failed` scenario).

---

# History

## GET /history

Aggregated view of violation + invoice + payment + rule version, per the
assignment's flow 5.

**Query params:**
- `member_id` — **forced to `req.user.id` for MEMBER**
- `from`, `to` — by `violation_timestamp`
- `page`, `page_size`, `sort`, `order`

**Response 200:**
```json
{
  "success": true,
  "data": {
    "items": [
      {
        "violation_id": "uuid",
        "license_plate": "B1234XYZ",
        "violation_type": "no_parking_zone",
        "location": "Jakarta",
        "violation_timestamp": "2026-01-01T10:00:00Z",
        "fine_amount": 150000,
        "rule_version_id": "uuid",
        "rule_version_number": 1,
        "invoice_id": "uuid",
        "invoice_status": "PAID",
        "payment_status": "PAID",
        "calculation_snapshot": { ... }
      }
    ],
    "page": 1,
    "page_size": 20,
    "total": 1
  }
}
```

**Errors:** `VALIDATION_ERROR`, `FORBIDDEN`.

---

# Members (OFFICER lookup)

## GET /members

List users with role `MEMBER` (used by officer to pick `member_id` for a new violation).

**Query params:** `q` (search name/email), `page`, `page_size`.

**Response 200:**
```json
{
  "success": true,
  "data": {
    "items": [
      { "id": "uuid", "name": "Member User", "email": "member@example.com", "role": "MEMBER" }
    ],
    "page": 1,
    "page_size": 20,
    "total": 1
  }
}
```

**Errors:** `FORBIDDEN` (MEMBER cannot use this).

---

# Error format (full reference)

See `ERROR_CATALOG.md` for the full list of error codes and HTTP statuses.

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": {
      "license_plate": ["license_plate is required"]
    }
  }
}
```
