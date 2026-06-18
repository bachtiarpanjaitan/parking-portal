# Notifications Module

## Purpose

The Notification Worker demonstrates asynchronous processing using RabbitMQ.

It is intentionally lightweight and exists to show:

* Event-driven architecture
* Message broker integration
* Service decoupling
* Asynchronous processing

Notification delivery itself is not a core business requirement of the assignment.

---

# Architecture

```text
Violation Service
       |
       | Publish Event
       v
    RabbitMQ
       |
       v
Notification Worker
       |
       v
Notification Store / Logs
```

The Notification Worker is a consumer only.

It never modifies core business entities.

---

# Responsibilities

The Notification Worker is responsible for:

* Consuming domain events
* Generating notification messages
* Recording notifications
* Logging notification activity

The worker must not:

* Calculate fines
* Modify violations
* Modify invoices
* Modify payments
* Modify fine rules

---

# Event Sources

## Violation Service

Publishes:

* ViolationCreated
* InvoiceCreated
* RulePublished

---

## Payment Service

Publishes:

* PaymentSucceeded
* PaymentFailed

---

# RabbitMQ Topology

Exchange:

```text
parking.events
```

Type:

```text
topic
```

Queue:

```text
notification.queue
```

Routing Keys:

```text
violation.created
invoice.created
payment.succeeded
payment.failed
rule.published
```

---

# Event Contract

All events must follow the same envelope structure.

```json
{
  "event_id": "uuid",
  "event_type": "ViolationCreated",
  "occurred_at": "2026-01-01T10:00:00Z",
  "payload": {}
}
```

---

# Event Definitions

## ViolationCreated

```json
{
  "violation_id": "uuid",
  "license_plate": "B1234XYZ",
  "fine_amount": 150000,
  "member_id": "uuid"
}
```

Notification Example:

```text
A new parking violation has been issued.
```

---

## InvoiceCreated

```json
{
  "invoice_id": "uuid",
  "violation_id": "uuid",
  "amount": 150000
}
```

Notification Example:

```text
A new invoice has been generated.
```

---

## PaymentSucceeded

```json
{
  "invoice_id": "uuid",
  "transaction_id": "trx_123",
  "amount": 150000
}
```

Notification Example:

```text
Payment completed successfully.
```

---

## PaymentFailed

```json
{
  "invoice_id": "uuid",
  "transaction_id": "trx_123"
}
```

Notification Example:

```text
Payment attempt failed.
```

---

## RulePublished

```json
{
  "rule_version_id": "uuid",
  "version_number": 2
}
```

Notification Example:

```text
A new fine rule version has been published.
```

---

# Notification Storage

For assignment scope, notifications may be stored in PostgreSQL.

Table:

```sql
CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    user_id UUID NULL,
    event_type VARCHAR(100),
    title VARCHAR(255),
    message TEXT,
    created_at TIMESTAMP NOT NULL
);
```

This table is optional.

A logging-only implementation is acceptable.

---

# Processing Flow

Example:

```text
Officer Creates Violation
            |
            v
Violation Saved
            |
            v
Publish ViolationCreated
            |
            v
RabbitMQ
            |
            v
Notification Worker
            |
            v
Notification Created
```

---

# Error Handling

If processing fails:

* Message should not be acknowledged
* RabbitMQ should requeue the message

Worker must log:

* event id
* event type
* processing error

---

# Idempotency

The worker should process an event only once.

Recommended approach:

```text
processed_events
```

table.

Schema:

```sql
CREATE TABLE processed_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMP NOT NULL
);
```

If the event already exists:

* Ignore event
* Acknowledge message

---

# Assignment Scope

Required:

* RabbitMQ consumer
* Event handling
* Logging

Optional:

* Notification database table
* User notification center
* Real-time websocket notifications

For this assignment, logging notifications is sufficient.

```
```
