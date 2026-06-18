# Environment Variables Configuration

## Purpose

This document defines all environment variables used by the Parking Violation Portal.

Goals:

* Consistent configuration across services
* Easy local development
* Docker compatibility
* Clear deployment requirements

All services must load configuration from environment variables.

---

# Environment Files

Backend:

```text id="1vhtlm"
backend/.env
```

Frontend:

```text id="7c4um0"
frontend/.env
```

Example files:

```text id="0x1f6w"
backend/.env.example
frontend/.env.example
```

Secrets must never be committed to source control.

---

# Shared Variables

## APP_ENV

Environment name.

Values:

```text id="g79z5j"
development
staging
production
```

Example:

```env id="k09clg"
APP_ENV=development
```

---

## APP_NAME

Application name.

Example:

```env id="6grm5s"
APP_NAME=Parking Violation Portal
```

---

## APP_PORT

Application listening port.

Example:

```env id="a5jlwm"
APP_PORT=8080
```

---

# PostgreSQL Configuration

## DB_HOST

Example:

```env id="2lxjzt"
DB_HOST=postgres
```

---

## DB_PORT

Example:

```env id="yod5s5"
DB_PORT=5432
```

---

## DB_NAME

Example:

```env id="5ftkh4"
DB_NAME=parking_portal
```

---

## DB_USER

Example:

```env id="l0hvq9"
DB_USER=postgres
```

---

## DB_PASSWORD

Example:

```env id="kjlwm1"
DB_PASSWORD=postgres
```

---

## DATABASE_URL

Optional consolidated connection string.

Example:

```env id="jlwm2"
DATABASE_URL=postgres://postgres:postgres@postgres:5432/parking_portal?sslmode=disable
```

If provided, services should prefer DATABASE_URL.

---

# JWT Configuration

## JWT_SECRET

Secret key used to sign JWT tokens.

Example:

```env id="jlwm3"
JWT_SECRET=change-me-in-production
```

Required:

* minimum 32 characters
* must be different in production

---

## JWT_EXPIRATION_HOURS

Token expiration time.

Example:

```env id="jlwm4"
JWT_EXPIRATION_HOURS=24
```

---

# RabbitMQ Configuration

## RABBITMQ_URL

Connection string.

Example:

```env id="jlwm5"
RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/
```

Used by:

* Violation Service
* Payment Service
* Notification Worker

---

## RABBITMQ_EXCHANGE

Example:

```env id="jlwm6"
RABBITMQ_EXCHANGE=parking.events
```

---

## RABBITMQ_NOTIFICATION_QUEUE

Example:

```env id="jlwm7"
RABBITMQ_NOTIFICATION_QUEUE=notification.queue
```

---

# File Upload Configuration

## STORAGE_PATH

Local upload directory.

Example:

```env id="jlwm8"
STORAGE_PATH=./storage
```

---

## MAX_UPLOAD_SIZE_MB

Maximum upload size.

Example:

```env id="jlwm9"
MAX_UPLOAD_SIZE_MB=5
```

---

## PUBLIC_UPLOAD_URL

Public upload base path.

Example:

```env id="jlwm10"
PUBLIC_UPLOAD_URL=/uploads
```

Example generated URL:

```text id="jlwm11"
http://localhost:8080/uploads/violations/file.jpg
```

---

# API Gateway Configuration

## VIOLATION_SERVICE_URL

Example:

```env id="jlwm12"
VIOLATION_SERVICE_URL=http://violation-service:8081
```

---

## PAYMENT_SERVICE_URL

Example:

```env id="jlwm13"
PAYMENT_SERVICE_URL=http://payment-service:8082
```

---

# Notification Worker Configuration

## WORKER_CONCURRENCY

Number of consumers.

Example:

```env id="jlwm14"
WORKER_CONCURRENCY=1
```

Assignment default:

```text id="jlwm15"
1
```

---

## WORKER_RETRY_COUNT

Example:

```env id="jlwm16"
WORKER_RETRY_COUNT=3
```

---

# Frontend Variables

## VITE_API_URL

Base API URL.

Example:

```env id="jlwm17"
VITE_API_URL=http://localhost:8080/api/v1
```

---

## VITE_APP_NAME

Example:

```env id="jlwm18"
VITE_APP_NAME=Parking Violation Portal
```

---

# Development Example

Backend:

```env id="jlwm19"
APP_ENV=development

APP_PORT=8080

DB_HOST=postgres
DB_PORT=5432
DB_NAME=parking_portal
DB_USER=postgres
DB_PASSWORD=postgres

JWT_SECRET=super-secret-key-for-development

RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/

RABBITMQ_EXCHANGE=parking.events
RABBITMQ_NOTIFICATION_QUEUE=notification.queue

STORAGE_PATH=./storage
MAX_UPLOAD_SIZE_MB=5
PUBLIC_UPLOAD_URL=/uploads

VIOLATION_SERVICE_URL=http://violation-service:8081
PAYMENT_SERVICE_URL=http://payment-service:8082
```

---

Frontend:

```env id="jlwm20"
VITE_API_URL=http://localhost:8080/api/v1
VITE_APP_NAME=Parking Violation Portal
```

---

# Docker Compose Mapping

PostgreSQL:

```yaml id="jlwm21"
environment:
  DB_HOST: postgres
  DB_PORT: 5432
  DB_NAME: parking_portal
  DB_USER: postgres
  DB_PASSWORD: postgres
```

---

RabbitMQ:

```yaml id="jlwm22"
environment:
  RABBITMQ_URL: amqp://guest:guest@rabbitmq:5672/
```

---

Gateway:

```yaml id="jlwm23"
environment:
  VIOLATION_SERVICE_URL: http://violation-service:8081
  PAYMENT_SERVICE_URL: http://payment-service:8082
```

---

# Production Notes

For production deployment:

Required changes:

```text id="jlwm24"
JWT_SECRET
DB_PASSWORD
RABBITMQ_URL
```

Must not use development values.

---

# Variables Not Used

The following infrastructure is intentionally not implemented:

```text id="jlwm25"
AWS S3
MinIO
Redis
Kubernetes
```

Therefore no environment variables are defined for:

```text id="jlwm26"
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
S3_BUCKET
MINIO_ENDPOINT
REDIS_URL
```

These may be introduced in future versions if the architecture evolves.

---

# Validation Rules

Application startup must fail if any required variable is missing.

Required:

```text id="jlwm27"
APP_ENV

DB_HOST
DB_PORT
DB_NAME
DB_USER
DB_PASSWORD

JWT_SECRET

RABBITMQ_URL

STORAGE_PATH

VIOLATION_SERVICE_URL
PAYMENT_SERVICE_URL
```

The application should not start with incomplete configuration.
