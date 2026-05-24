# XynPOS Backend

**XynPOS** — Production-ready Go Microservices Backend for Indonesian POS System

[![CI Auth Service](https://github.com/extendedsynaptic/xynpos-backend/actions/workflows/ci-auth-service.yml/badge.svg)](https://github.com/extendedsynaptic/xynpos-backend/actions)
[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://go.dev)
[![License](https://img.shields.io/badge/license-Proprietary-red.svg)]()

---

## Architecture

```
Cloudflare → Kong API Gateway → 12 Go Microservices → PostgreSQL 18 + Redis + NATS
```

- **12 Microservices**: Auth, Tenant, Product, Inventory, POS, Payment, Customer, Report, Notification, File, Subscription, Audit
- **API Gateway**: Kong (DB-less declarative mode) — all external traffic via port 8000
- **Database**: PostgreSQL 18 with schema-per-tenant isolation via PgBouncer (session mode)
- **Cache/Queue**: Redis 7 (sessions, rate limiting, idempotency, pub/sub)
- **Messaging**: NATS JetStream (async events, CloudEvents v1.0 envelope)
- **Search**: Meilisearch (product search, typo-tolerant)
- **Observability**: OpenTelemetry → Jaeger + Prometheus + Grafana + Loki
- **Internal RPC**: gRPC with proto files in `backend/shared/proto/`
- **Notifications**: SSE (web), Firebase FCM (mobile), Resend (email)

## Quick Start

```bash
# 1. Clone and setup
git clone https://github.com/extendedsynaptic/xynpos-backend.git
cd xynpos-backend

# 2. Setup local environment
make setup

# 3. Update .env with your values
cp .env.example .env && nano .env

# 4. Start infrastructure
make docker-up

# 5. Run migrations
make migrate-global

# 6. Start a service (example: auth-service)
make run SVC=auth-service

# 7. Optional: Start monitoring (Jaeger, Grafana, Prometheus, Loki)
make docker-mon
```

## Available URLs (Local Dev)

| Service | URL |
|---------|-----|
| Kong API Gateway | http://localhost:8000 |
| Kong Admin | http://localhost:8001 |
| Auth Service (direct) | http://localhost:8011 |
| Swagger UI | http://localhost:8011/swagger/index.html |
| PgAdmin | localhost:5433 |
| Meilisearch | http://localhost:7700 |
| MinIO Console | http://localhost:9001 |
| Jaeger UI | http://localhost:16686 |
| Grafana | http://localhost:3001 |
| Prometheus | http://localhost:9090 |
| NATS Monitoring | http://localhost:8222 |

## Project Structure

```
xynpos-backend/
├── backend/
│   ├── shared/           # Shared Go packages (config, db, jwt, tracer, etc.)
│   │   ├── pkg/          # All shared utilities
│   │   └── proto/        # gRPC proto definitions (single source of truth)
│   ├── services/         # 12 microservices
│   └── gateway/          # Kong configuration
├── infra/
│   ├── docker/           # Dockerfiles and init scripts
│   ├── kubernetes/       # K8s manifests + Kustomize overlays
│   ├── monitoring/       # Prometheus, Grafana, Loki configs
│   └── scripts/          # Setup and migration scripts
├── e2e/                  # End-to-end tests
├── docker-compose.yml
├── go.work
└── Makefile
```

## Make Commands

```bash
make help           # Show all commands
make test           # Run all tests
make lint           # Run linter
make swagger-gen SVC=auth-service  # Generate Swagger docs
make proto-gen      # Generate gRPC code from protos
make coverage       # Coverage report for all services
```

## Multi-Tenant Architecture

Each tenant gets an isolated PostgreSQL schema (`tenant_<uuid>`).

```sql
-- Created automatically on tenant registration:
CREATE SCHEMA tenant_550e8400e29b41d4a716446655440000;

-- App uses: SET search_path = tenant_xxx (via PgBouncer session mode)
```

⚠️ PgBouncer **MUST** use `pool_mode = session` for `SET search_path` to work.

## Service Communication

| Pattern | Use Case |
|---------|----------|
| **REST via Kong** | External (Web & Mobile clients) |
| **gRPC** | Internal sync: POS → Product/Inventory/Payment |
| **NATS JetStream** | Internal async: events (transaction.completed, etc.) |

## Testing Strategy

| Layer | Coverage Target | Tool |
|-------|----------------|------|
| Unit | ≥ 80% (usecase) | testify + mockery |
| Integration | ≥ 70% (handler) | testcontainers-go |
| E2E | Critical journeys | httptest + k6 |

## License

Proprietary — Extended Synaptic. All rights reserved.
