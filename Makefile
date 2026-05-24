# =============================================================
# XynPOS Backend — Root Makefile
# =============================================================
# Usage: make <target> [SVC=service-name] [TENANT=tenant-uuid]
# =============================================================

.PHONY: all help setup docker-up docker-mon docker-down \
        proto-gen mock-gen swagger-gen \
        test test-unit test-integration test-e2e coverage \
        lint vuln-check secret-scan \
        run migrate-global migrate-tenant \
        build-all k8s-apply

# Default
.DEFAULT_GOAL := help

# Colors
BOLD   := $(shell tput bold)
GREEN  := $(shell tput setaf 2)
YELLOW := $(shell tput setaf 3)
CYAN   := $(shell tput setaf 6)
RESET  := $(shell tput sgr0)

## ── Help ──────────────────────────────────────────────────────

help: ## Show this help message
	@echo ""
	@echo "$(BOLD)XynPOS Backend — Available Commands$(RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-25s$(RESET) %s\n", $$1, $$2}'
	@echo ""

## ── Setup ─────────────────────────────────────────────────────

setup: ## Setup local dev environment (run once)
	@bash infra/scripts/setup-local.sh

docker-up: ## Start all infrastructure services (no monitoring)
	docker compose up -d postgres pgbouncer redis nats meilisearch minio minio-setup kong

docker-all: ## Start all infrastructure + all services
	docker compose up -d

docker-mon: ## Start monitoring stack (Jaeger, Prometheus, Grafana, Loki)
	docker compose --profile monitoring up -d

docker-down: ## Stop and remove all containers
	docker compose --profile monitoring down

docker-clean: ## Stop containers and remove all volumes (⚠ data loss!)
	docker compose --profile monitoring down -v

docker-logs: ## Follow logs for a service (make docker-logs SVC=auth-service)
	docker compose logs -f $(SVC)

## ── Code Generation ───────────────────────────────────────────

proto-gen: ## Generate Go code from all .proto files
	@bash infra/scripts/gen-proto.sh

mock-gen: ## Generate mocks using mockery (run in a service dir: make mock-gen SVC=auth-service)
	@cd backend/services/$(SVC) && \
		mockery --all --output internal/repository/mock --outpkg mock --case snake

swagger-gen: ## Generate Swagger docs for a service (make swagger-gen SVC=auth-service)
	@cd backend/services/$(SVC) && \
		swag init -g cmd/main.go -o docs --parseDependency --parseInternal
	@echo "$(GREEN)✅ Swagger generated for $(SVC)$(RESET)"

swagger-gen-all: ## Generate Swagger docs for all services
	@for svc in backend/services/*/; do \
		SVC=$$(basename $$svc); \
		echo "$(CYAN)▶ Generating swagger for $$SVC...$(RESET)"; \
		cd $$svc && swag init -g cmd/main.go -o docs --parseDependency --parseInternal 2>/dev/null || true; \
		cd ../../../; \
	done

## ── Testing ───────────────────────────────────────────────────

test: ## Run all tests (unit + integration)
	@go test ./... -race -timeout 10m

test-unit: ## Run unit tests only (fast, no external dependencies)
	@go test ./... -short -race -timeout 5m

test-service: ## Run tests for a specific service (make test-service SVC=auth-service)
	@cd backend/services/$(SVC) && go test ./... -v -race

test-integration: ## Run integration tests (requires Docker services running)
	@go test ./... -run Integration -race -timeout 10m

test-e2e: ## Run end-to-end tests (requires full stack running)
	@go test ./e2e/... -v -timeout 15m

coverage: ## Generate coverage report for all services
	@echo "$(BOLD)Coverage Report$(RESET)"
	@for svc in backend/services/*/; do \
		SVC=$$(basename $$svc); \
		echo "$(CYAN)▶ $$SVC:$(RESET)"; \
		cd $$svc && go test ./internal/... -short -coverprofile=/tmp/cov-$$SVC.out 2>/dev/null && \
			go tool cover -func=/tmp/cov-$$SVC.out | grep "total:" | awk '{print "  Coverage: "$$3}'; \
		cd ../../../; \
	done

## ── Code Quality ──────────────────────────────────────────────

lint: ## Run golangci-lint across all code
	@golangci-lint run ./... --config=.golangci.yml

lint-service: ## Lint a specific service (make lint-service SVC=auth-service)
	@cd backend/services/$(SVC) && golangci-lint run ./... --config=../../.golangci.yml

vuln-check: ## Scan for known vulnerabilities (govulncheck)
	@govulncheck ./...

secret-scan: ## Scan for leaked secrets (gitleaks)
	@gitleaks detect --source . --verbose

vet: ## Run go vet across all code
	@go vet ./...

fmt: ## Format all Go code
	@gofmt -w -s .

## ── Development ───────────────────────────────────────────────

run: ## Run a service locally with hot-reload (make run SVC=auth-service)
	@cd backend/services/$(SVC) && air -c .air.toml 2>/dev/null || \
		go run ./cmd/main.go

run-all: ## Start all services via Docker Compose
	@docker compose up -d

## ── Database Migrations ───────────────────────────────────────

migrate-global: ## Run global schema migrations
	@bash infra/scripts/db-migrate.sh global up

migrate-tenant: ## Run migrations for a specific tenant (make migrate-tenant TENANT=<uuid>)
	@bash infra/scripts/db-migrate.sh tenant up $(TENANT)

migrate-down: ## Rollback last migration (make migrate-down SVC=auth-service)
	@bash infra/scripts/db-migrate.sh global down 1

## ── Docker Build ──────────────────────────────────────────────

build: ## Build Docker image for a service (make build SVC=auth-service)
	@docker build -t ghcr.io/extendedsynaptic/xynpos/$(SVC):dev \
		backend/services/$(SVC)/

build-all: ## Build Docker images for all services
	@for svc in backend/services/*/; do \
		SVC=$$(basename $$svc); \
		echo "$(CYAN)▶ Building $$SVC...$(RESET)"; \
		docker build -t ghcr.io/extendedsynaptic/xynpos/$$SVC:dev $$svc; \
	done

## ── Kubernetes ────────────────────────────────────────────────

k8s-apply-staging: ## Apply K8s manifests to staging
	@kubectl apply -k infra/kubernetes/overlays/staging

k8s-apply-prod: ## Apply K8s manifests to production (⚠ careful!)
	@kubectl apply -k infra/kubernetes/overlays/production

k8s-rollout: ## Check rollout status for a service (make k8s-rollout SVC=auth-service)
	@kubectl rollout status deployment/$(SVC) -n xynpos

## ── Workspace ─────────────────────────────────────────────────

tidy: ## Run go mod tidy for all modules
	@cd backend/shared && go mod tidy
	@for svc in backend/services/*/; do \
		cd $$svc && go mod tidy; \
		cd ../../../; \
	done

workspace-sync: ## Sync go.work with current modules
	@go work sync
