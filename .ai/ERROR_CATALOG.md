# Error Catalog

## Purpose

This document defines the standard error handling strategy for the Parking Violation Portal.

Goals:

* Consistent API responses
* Predictable HTTP status codes
* Standardized error codes
* Easier frontend integration
* Easier debugging and logging

All services must follow this catalog.

---

# Error Response Format

All errors must use the following structure.

```json id="xjlwm1"
{
  "success": false,
  "error": {
    "code": "VIOLATION_NOT_FOUND",
    "message": "Violation not found"
  }
}
```

Optional validation details:

```json id="zwx6r8"
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": {
      "license_plate": [
        "license plate is required"
      ]
    }
  }
}
```

---

# HTTP Status Codes

| Status | Meaning                 |
| ------ | ----------------------- |
| 200    | Success                 |
| 201    | Resource Created        |
| 400    | Validation Error        |
| 401    | Unauthorized            |
| 403    | Forbidden               |
| 404    | Resource Not Found      |
| 409    | Conflict                |
| 422    | Business Rule Violation |
| 500    | Internal Server Error   |

---

# General Errors

## VALIDATION_ERROR

HTTP:

```text id="4vjlwm"
400 Bad Request
```

Example:

```json id="z7sk8m"
{
  "code": "VALIDATION_ERROR",
  "message": "Validation failed"
}
```

---

## UNAUTHORIZED

HTTP:

```text
401 Unauthorized
```

Example (also covers wrong-password):

```json
{
  "code": "UNAUTHORIZED",
  "message": "invalid email or password"
}
```

> All login failures (email not found, wrong password, missing hash) return
> this single response to avoid leaking whether the email exists.

---

## FORBIDDEN

HTTP:

```text id="jjlwm3"
403 Forbidden
```

Example:

```json id="fjlwm4"
{
  "code": "FORBIDDEN",
  "message": "Access denied"
}
```

---

## RESOURCE_NOT_FOUND

HTTP:

```text id="jlwm5s"
404 Not Found
```

Example:

```json id="zjlwm6"
{
  "code": "RESOURCE_NOT_FOUND",
  "message": "Resource not found"
}
```

---

## INTERNAL_SERVER_ERROR

HTTP:

```text id="jlwm7x"
500 Internal Server Error
```

Example:

```json id="jlwm8a"
{
  "code": "INTERNAL_SERVER_ERROR",
  "message": "Unexpected server error"
}
```

---

# Authentication Errors

## INVALID_TOKEN

HTTP:

```text id="jlwm9d"
401 Unauthorized
```

Example:

```json id="jlwm10"
{
  "code": "INVALID_TOKEN",
  "message": "Token is invalid"
}
```

---

## TOKEN_EXPIRED

HTTP:

```text id="jlwm11"
401 Unauthorized
```

Example:

```json id="jlwm12"
{
  "code": "TOKEN_EXPIRED",
  "message": "Token has expired"
}
```

---

# Rule Management Errors

## RULE_VERSION_NOT_FOUND

HTTP:

```text id="jlwm13"
404 Not Found
```

Example:

```json id="jlwm14"
{
  "code": "RULE_VERSION_NOT_FOUND",
  "message": "Rule version not found"
}
```

---

## RULE_ALREADY_ACTIVE

HTTP:

```text id="jlwm15"
409 Conflict
```

Example:

```json id="jlwm16"
{
  "code": "RULE_ALREADY_ACTIVE",
  "message": "Rule version is already active"
}
```

---

## NO_ACTIVE_RULE

HTTP:

```text id="jlwm17"
422 Unprocessable Entity
```

Example:

```json id="jlwm18"
{
  "code": "NO_ACTIVE_RULE",
  "message": "No active rule version found"
}
```

---

# Violation Errors

## VIOLATION_NOT_FOUND

HTTP:

```text id="jlwm19"
404 Not Found
```

Example:

```json id="jlwm20"
{
  "code": "VIOLATION_NOT_FOUND",
  "message": "Violation not found"
}
```

---

## INVALID_VIOLATION_TYPE

HTTP:

```text id="jlwm21"
400 Bad Request
```

Example:

```json id="jlwm22"
{
  "code": "INVALID_VIOLATION_TYPE",
  "message": "Invalid violation type"
}
```

---

## LICENSE_PLATE_REQUIRED

HTTP:

```text id="jlwm23"
400 Bad Request
```

Example:

```json id="jlwm24"
{
  "code": "LICENSE_PLATE_REQUIRED",
  "message": "License plate is required"
}
```

---

## PHOTO_REQUIRED

HTTP:

```text id="jlwm25"
400 Bad Request
```

Example:

```json id="jlwm26"
{
  "code": "PHOTO_REQUIRED",
  "message": "Violation photo is required"
}
```

---

# Invoice Errors

## INVOICE_NOT_FOUND

HTTP:

```text id="jlwm27"
404 Not Found
```

Example:

```json id="jlwm28"
{
  "code": "INVOICE_NOT_FOUND",
  "message": "Invoice not found"
}
```

---

## INVOICE_ALREADY_PAID

HTTP:

```text id="jlwm29"
409 Conflict
```

Example:

```json id="jlwm30"
{
  "code": "INVOICE_ALREADY_PAID",
  "message": "Invoice already paid"
}
```

---

## INVALID_INVOICE_STATUS

HTTP:

```text id="jlwm31"
422 Unprocessable Entity
```

Example:

```json id="jlwm32"
{
  "code": "INVALID_INVOICE_STATUS",
  "message": "Invoice status does not allow this operation"
}
```

---

# Payment Errors

## PAYMENT_FAILED

HTTP:

```text id="jlwm33"
422 Unprocessable Entity
```

Example:

```json id="jlwm34"
{
  "code": "PAYMENT_FAILED",
  "message": "Payment provider returned failed status"
}
```

---

## PAYMENT_NOT_FOUND

HTTP:

```text id="jlwm35"
404 Not Found
```

Example:

```json id="jlwm36"
{
  "code": "PAYMENT_NOT_FOUND",
  "message": "Payment record not found"
}
```

---

## INVALID_PAYMENT_SCENARIO

HTTP:

```text id="jlwm37"
400 Bad Request
```

Example:

```json id="jlwm38"
{
  "code": "INVALID_PAYMENT_SCENARIO",
  "message": "Payment scenario must be success or failed"
}
```

---

# Upload Errors

## FILE_REQUIRED

HTTP:

```text id="jlwm39"
400 Bad Request
```

Example:

```json id="jlwm40"
{
  "code": "FILE_REQUIRED",
  "message": "File is required"
}
```

---

## FILE_TOO_LARGE

HTTP:

```text id="jlwm41"
400 Bad Request
```

Example:

```json id="jlwm42"
{
  "code": "FILE_TOO_LARGE",
  "message": "File exceeds maximum allowed size"
}
```

---

## INVALID_FILE_TYPE

HTTP:

```text id="jlwm43"
400 Bad Request
```

Example:

```json id="jlwm44"
{
  "code": "INVALID_FILE_TYPE",
  "message": "Unsupported file type"
}
```

---

## FILE_UPLOAD_FAILED

HTTP:

```text id="jlwm45"
500 Internal Server Error
```

Example:

```json id="jlwm46"
{
  "code": "FILE_UPLOAD_FAILED",
  "message": "Unable to save uploaded file"
}
```

---

# Notification Errors

## EVENT_PUBLISH_FAILED

HTTP:

```text id="jlwm47"
500 Internal Server Error
```

Example:

```json id="jlwm48"
{
  "code": "EVENT_PUBLISH_FAILED",
  "message": "Failed to publish event"
}
```

---

## EVENT_PROCESSING_FAILED

HTTP:

```text id="jlwm49"
500 Internal Server Error
```

Example:

```json id="jlwm50"
{
  "code": "EVENT_PROCESSING_FAILED",
  "message": "Failed to process event"
}
```

---

# Frontend Handling Rules

Frontend must:

* Display error.message to users
* Use error.code for conditional behavior
* Never parse raw backend error strings

Example:

```typescript id="jlwm51"
switch (error.code) {
  case "INVOICE_ALREADY_PAID":
    showAlreadyPaidDialog();
    break;

  case "TOKEN_EXPIRED":
    redirectToLogin();
    break;

  default:
    showGenericError();
}
```

---

# Logging Rules

Server logs must include:

```text id="jlwm52"
request_id
user_id
error_code
error_message
timestamp
```

Example:

```json id="jlwm53"
{
  "request_id": "uuid",
  "user_id": "uuid",
  "error_code": "INVOICE_ALREADY_PAID",
  "message": "Invoice already paid"
}
```

---

# Implementation Rule

Services must never return raw errors directly to clients.

Bad:

```go id="jlwm54"
return err
```

Good:

```go id="jlwm55"
return errors.New(
    "INVOICE_ALREADY_PAID"
)
```

Controllers or middleware are responsible for translating internal errors into standardized API responses defined in this document.
