# Testing Strategy

## Purpose

This document defines the testing strategy for the Parking Violation Portal.

The objective is to ensure that:

* Business rules are correctly implemented
* Rule versioning behaves correctly
* Historical fines remain immutable
* Payment flows work as expected
* Core assignment requirements are verified

Testing focuses on business-critical functionality rather than exhaustive coverage.

---

# Testing Levels

The project includes three levels of testing:

## Unit Tests

Purpose:

Verify business logic in isolation.

Target:

* Fine Engine
* Rule Management
* Payment Processing
* Validation Logic

---

## Integration Tests

Purpose:

Verify interactions between modules and database.

Target:

* Violation creation flow
* Invoice generation
* Rule publishing
* Payment processing

---

## End-to-End Tests

Purpose:

Verify complete user workflows.

Target:

* Officer workflows
* Member workflows

---

# Test Coverage Priorities

Priority 1 (Critical)

* Fine calculation
* Rule versioning
* Historical consistency
* Payment processing

Priority 2

* API endpoints
* Validation
* Permissions

Priority 3

* RabbitMQ consumers
* Notification worker

---

# Unit Tests

## Fine Engine

File:

```text id="u6jlwm"
internal/fines/service_test.go
```

---

### Test Case

Daytime Fine

Input:

```text id="zztfxk"
Violation Type:
no_parking_zone

Time:
10:00

Prior Unpaid:
0
```

Expected:

```text id="jlwmji"
150000
```

---

### Test Case

Night Fine

Input:

```text id="snubfj"
Violation Type:
no_parking_zone

Time:
23:00

Prior Unpaid:
0
```

Expected:

```text id="63ajyh"
225000
```

Calculation:

```text id="s4l1j7"
150000 × 1.5 × 1.0
```

---

### Test Case

Repeat Offender

Input:

```text id="k2x5of"
Violation Type:
no_parking_zone

Time:
10:00

Prior Unpaid:
2
```

Expected:

```text id="gr5a8j"
300000
```

Calculation:

```text id="q8ggqz"
150000 × 1.0 × 2.0
```

---

# Rule Versioning Tests

File:

```text id="n5xjlwm"
internal/rules/service_test.go
```

---

### Test Case

Publish New Rule Version

Given:

```text id="e5jzoc"
Version 1 active
```

When:

```text id="l5gmy0"
Version 2 published
```

Then:

```text id="0xsl9o"
Version 1 inactive
Version 2 active
```

---

### Test Case

Historical Fine Preservation

Given:

```text id="x1u7mz"
Violation A

Rule Version 1

Fine = 150000
```

When:

```text id="if7n1y"
Rule Version 2 changes fine
```

Then:

```text id="ylh5b0"
Violation A remains 150000
```

---

# Violation Module Tests

File:

```text id="s4z3c7"
internal/violations/service_test.go
```

---

### Test Case

Create Violation

Expected:

* violation created
* rule version stored
* snapshot stored
* invoice created

---

### Test Case

Create Violation Without Photo

Expected:

```text id="fxw0ok"
validation error
```

---

### Test Case

Invalid Violation Type

Expected:

```text id="9dk7wf"
validation error
```

---

# Payment Module Tests

File:

```text id="rj7uqm"
internal/payments/service_test.go
```

---

### Test Case

Payment Success

Input:

```json id="kg1fpc"
{
  "scenario": "success"
}
```

Expected:

```text id="zokh7j"
invoice status = PAID
payment status = PAID
```

---

### Test Case

Payment Failed

Input:

```json id="3qv6w8"
{
  "scenario": "failed"
}
```

Expected:

```text id="my6khs"
invoice status = FAILED
payment status = FAILED
```

---

### Test Case

Already Paid Invoice

Expected:

```text id="wh2o3d"
cannot process payment
```

---

# Upload Tests

File:

```text id="l9w6pu"
internal/uploads/service_test.go
```

---

### Test Case

Upload JPG

Expected:

```text id="jqzjv4"
success
```

---

### Test Case

Upload PNG

Expected:

```text id="1gk2p4"
success
```

---

### Test Case

Upload Unsupported File

Input:

```text id="iqb78r"
document.pdf
```

Expected:

```text id="5t6xv5"
validation error
```

---

### Test Case

Upload Large File

Input:

```text id="5ub4ha"
10MB image
```

Expected:

```text id="szyivx"
file size error
```

---

# Integration Tests

File:

```text id="1z7r8f"
tests/integration/
```

---

## Flow 1

Officer Creates Violation

Expected:

* violation created
* invoice created
* snapshot stored
* event published

---

## Flow 2

Publish New Rule

Expected:

* previous version inactive
* new version active

---

## Flow 3

Member Pays Fine

Expected:

* payment recorded
* invoice updated
* event published

---

## Flow 4

History Query

Expected:

Response contains:

* violation
* fine amount
* rule version
* payment status

---

# API Tests

Endpoints:

```text id="1o4jye"
POST /violations
GET /violations

POST /rules
POST /rules/{id}/publish

POST /payments

GET /history

POST /uploads/violations
```

Verify:

* status code
* response schema
* validation errors

---

# End-to-End Test Scenarios

## Scenario 1

Officer Issues Violation

Steps:

1. Login as officer
2. Upload photo
3. Create violation

Verify:

* violation exists
* invoice exists

---

## Scenario 2

Officer Publishes New Rule

Steps:

1. Create rule version
2. Publish version

Verify:

* active version updated

---

## Scenario 3

Member Pays Invoice

Steps:

1. Login as member
2. Select invoice
3. Choose success
4. Pay

Verify:

* invoice paid
* payment recorded

---

## Scenario 4

Historical Consistency

Steps:

1. Create violation under V1
2. Publish V2
3. View history

Verify:

```text id="svrzcy"
original rule version preserved
original fine preserved
```

---

# RabbitMQ Tests

## Event Publishing

Verify:

* ViolationCreated published
* InvoiceCreated published
* PaymentSucceeded published
* PaymentFailed published
* RulePublished published

---

## Notification Worker

Verify:

* event consumed
* message logged
* no duplicate processing

---

# Test Data

Officer:

```text id="jrg0wk"
officer@example.com
```

Member:

```text id="4w21mt"
member@example.com
```

Default Rule Version:

```text id="wkx5m5"
Version 1
```

License Plate:

```text id="wbhdhy"
B1234XYZ
```

---

# Success Criteria

The implementation is considered complete when all five assignment flows pass:

1. Officer submits violation
2. Fine calculated correctly
3. Rule versioning works
4. Member pays fine
5. History shows applied rule version

These five flows are the primary acceptance criteria for the assignment.
