# Folder Structure

> **Layout assumption:** this assignment uses a **monorepo with per-service Go modules**.
> Each backend service has its own `go.mod`. The frontend is a single Vite + React app.
> See `ARCHITECTURE_DECISION.md` (ADR-008, ADR-009) for the service list.

```
parking_violation_portal/
├── backend/
│   ├── gateway/                # API Gateway service (see ADR-009)
│   │   ├── cmd/
│   │   │   └── main.go
│   │   ├── internal/
│   │   │   ├── auth/           # JWT validation
│   │   │   ├── router/         # route table → backend services
│   │   │   ├── proxy/          # HTTP forwarder
│   │   │   └── middleware/     # auth, request-id, error envelope
│   │   ├── go.mod
│   │   └── Dockerfile
│   │
│   ├── violation-service/      # Owns: violations, invoices, rules, fine_rule_details
│   │   ├── cmd/
│   │   │   └── main.go
│   │   ├── internal/
│   │   │   ├── auth/           # login handler (shared with gateway)
│   │   │   ├── users/          # member lookup
│   │   │   ├── rules/          # rule versions + details
│   │   │   ├── violations/     # violation CRUD + fine engine
│   │   │   ├── invoices/       # invoice CRUD
│   │   │   ├── uploads/        # photo upload handler (see PHOTO_STORAGE.md)
│   │   │   ├── history/        # aggregated history view
│   │   │   ├── database/       # postgres connection
│   │   │   ├── events/         # RabbitMQ publisher
│   │   │   ├── shared/         # DTOs, errors, helpers shared within service
│   │   │   └── middleware/     # role check (OFFICER vs MEMBER)
│   │   ├── migrations/         # SQL migrations
│   │   ├── go.mod
│   │   └── Dockerfile
│   │
│   ├── payment-service/        # Owns: payments
│   │   ├── cmd/
│   │   │   └── main.go
│   │   ├── internal/
│   │   │   ├── payments/       # payment handlers, mock provider
│   │   │   ├── events/         # RabbitMQ publisher
│   │   │   ├── database/
│   │   │   └── shared/
│   │   ├── go.mod
│   │   └── Dockerfile
│   │
│   ├── notification-worker/    # Consumer-only worker (see NOTIFICATIONS.md)
│   │   ├── cmd/
│   │   │   └── main.go
│   │   ├── internal/
│   │   │   ├── consumer/       # RabbitMQ consumer
│   │   │   ├── notifications/  # log + optional DB write
│   │   │   └── shared/
│   │   ├── go.mod
│   │   └── Dockerfile
│   │
│   ├── pkg/                    # Cross-service shared Go types (DTOs, enums)
│   │   ├── events/             # event envelope + payload types
│   │   ├── errors/             # error code constants
│   │   └── money/              # decimal helpers
│   │
│   ├── docker-compose.yml      # all services + postgres + rabbitmq
│   └── Makefile                # convenience commands
│
├── frontend/
│   ├── src/
│   │   ├── app/                # routing root
│   │   │   ├── router.tsx
│   │   │   └── providers.tsx   # query client, auth, toaster
│   │   │
│   │   ├── pages/              # route components (thin)
│   │   │
│   │   ├── modules/            # feature modules
│   │   │   ├── auth/
│   │   │   │   ├── pages/
│   │   │   │   ├── components/
│   │   │   │   ├── hooks/
│   │   │   │   └── service.ts
│   │   │   ├── violations/
│   │   │   ├── rules/
│   │   │   ├── invoices/
│   │   │   ├── payments/
│   │   │   ├── uploads/
│   │   │   ├── members/
│   │   │   └── history/
│   │   │
│   │   ├── components/         # shared UI (DataTable, Form, Button, Card)
│   │   ├── services/           # cross-feature service layer
│   │   ├── hooks/              # cross-feature hooks
│   │   ├── layouts/            # OfficerLayout, MemberLayout
│   │   ├── lib/                # axios client, formatters, query keys
│   │   ├── types/              # shared TypeScript types / DTOs
│   │   ├── stores/             # zustand stores (auth, ui)
│   │   └── mocks/              # MSW handlers for tests
│   │
│   ├── public/
│   ├── package.json
│   ├── tsconfig.json
│   ├── tailwind.config.js
│   └── vite.config.ts
│
├── storage/                    # photo uploads (mounted volume, see PHOTO_STORAGE.md)
│   └── violations/
│
├── docs/                       # DESIGN.md assets
│   ├── erd.png
│   ├── erd.drawio
│   ├── data-flow.png
│   └── data-flow.drawio
│
├── DESIGN.md
├── README.md
├── .env.example
├── .gitignore
└── .ai/                        # this folder
```

---

# Module mapping

| Concern                | Folder                                |
| ---------------------- | ------------------------------------- |
| Login (mock)           | `backend/violation-service/internal/auth` (called by gateway) |
| Rule management        | `backend/violation-service/internal/rules` |
| Fine engine            | `backend/violation-service/internal/violations` |
| Violation CRUD         | `backend/violation-service/internal/violations` |
| Invoice CRUD           | `backend/violation-service/internal/invoices` |
| Photo upload           | `backend/violation-service/internal/uploads` |
| History aggregation    | `backend/violation-service/internal/history` |
| Payment processing     | `backend/payment-service/internal/payments` |
| Event publishing       | `backend/*/internal/events`            |
| Event consumption      | `backend/notification-worker/internal/consumer` |
| Routing / JWT          | `backend/gateway/internal/{router,auth,proxy}` |
| Shared types           | `backend/pkg/`                        |
