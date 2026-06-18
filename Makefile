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
#   make test            - run all Go tests
#   make run-violation   - run violation service locally (loads ../.env)
#   make run-payment     - run payment service locally
#   make run-gateway     - run API gateway locally
#   make run-worker      - run notification worker locally
# =============================================================================

SHELL := /bin/bash
COMPOSE := docker compose

.PHONY: help up down logs ps build rebuild migrate seed fresh test fmt tidy clean \
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

# ---- Local go run (loads ../.env automatically via backend/pkg/dotenv) ----
run-violation: ## Run the violation service locally
	cd backend && go run ./violation-service/cmd/api

run-payment: ## Run the payment service locally
	cd backend && go run ./payment-service/cmd/api

run-gateway: ## Run the API gateway locally
	cd backend && go run ./gateway/cmd/gateway

run-worker: ## Run the notification worker locally
	cd backend && go run ./notification-worker/cmd/worker
