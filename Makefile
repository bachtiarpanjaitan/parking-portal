# =============================================================================
# Parking Violation Portal - convenience commands
# =============================================================================
# Usage:
#   make help            - list targets
#   make up              - start all services
#   make down            - stop all services
#   make logs            - tail logs
#   make migrate         - run SQL migrations
#   make seed            - seed demo data
#   make test            - run all Go unit tests
#   make run-violation   - run violation service on :$(VIOLATION_PORT)
#   make run-payment     - run payment service on :$(PAYMENT_PORT)
#   make run-gateway     - run API gateway on :$(GATEWAY_PORT)
#   make run-worker      - run notification worker (no HTTP port)
#   make ports           - print the configured per-service ports
#
# Override any port from the command line, e.g.:
#   make run-violation VIOLATION_PORT=9081
# =============================================================================

SHELL := /bin/bash
COMPOSE := docker compose

# ---- Per-service ports (single source of truth; override from CLI if needed) ----
GATEWAY_PORT     ?= 8080
VIOLATION_PORT   ?= 8081
PAYMENT_PORT     ?= 8082
# WORKER has no HTTP port — it's a RabbitMQ consumer.

.PHONY: help up down logs ps build rebuild migrate seed fresh test fmt tidy clean ports \
        run-violation run-payment run-gateway run-worker

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

up: ## Start all services in background
	$(COMPOSE) up -d --build

down: ## Stop all services
	$(COMPOSE) down

logs: ## Tail logs (Ctrl-C to exit)
	$(COMPOSE) logs -f

ps: ## List running services
	$(COMPOSE) ps

build: ## Build all images
	$(COMPOSE) build

rebuild: ## Rebuild a specific service (usage: make rebuild SVC=violation-service)
	$(COMPOSE) build $(SVC)

# ---- DB ----
migrate: ## Run SQL migrations against the violation-service DB
	$(COMPOSE) run --rm violation-service /app/migrate

seed: ## Seed demo data (idempotent)
	$(COMPOSE) run --rm violation-service /app/seed

fresh: ## Drop everything and re-init (DESTRUCTIVE)
	$(COMPOSE) down -v
	$(COMPOSE) up -d postgres rabbitmq
	@echo "Waiting for postgres..."
	@sleep 5
	$(MAKE) migrate
	$(MAKE) seed
	$(COMPOSE) up -d

# ---- Go tests ----
test: ## Run all Go unit tests
	cd backend && go test ./...

fmt: ## Format Go code
	cd backend && go fmt ./...

tidy: ## Sync go.mod
	cd backend && go mod tidy

clean: ## Remove build artifacts and containers
	$(COMPOSE) down -v --remove-orphans
	rm -rf backend/**/bin/ backend/**/vendor/ storage/

# ---- Local go run ----
# Loads ../.env automatically via backend/pkg/dotenv. Port is set inline below so
# the Makefile is the single source of truth (no APP_PORT juggling in .env).
ports: ## Print the configured per-service ports
	@echo "  gateway     :$(GATEWAY_PORT)"
	@echo "  violation   :$(VIOLATION_PORT)"
	@echo "  payment     :$(PAYMENT_PORT)"
	@echo "  worker      (no HTTP port — RabbitMQ consumer)"

run-violation: ## Run the violation service on :$(VIOLATION_PORT)
	cd backend && APP_PORT=$(VIOLATION_PORT) go run ./violation-service/cmd/api

run-payment: ## Run the payment service on :$(PAYMENT_PORT)
	cd backend && APP_PORT=$(PAYMENT_PORT) go run ./payment-service/cmd/api

run-gateway: ## Run the API gateway on :$(GATEWAY_PORT)
	cd backend && APP_PORT=$(GATEWAY_PORT) go run ./gateway/cmd/gateway

run-worker: ## Run the notification worker (no HTTP port)
	cd backend && go run ./notification-worker/cmd/worker
