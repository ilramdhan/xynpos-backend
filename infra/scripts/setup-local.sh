#!/usr/bin/env bash
# =============================================================
# XynPOS — Local Development Setup
# =============================================================
# Run this once to set up your local environment
# =============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo ""
echo "╔═══════════════════════════════════════════════════════╗"
echo "║         XynPOS Backend — Local Setup                  ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# ── Check prerequisites ──────────────────────────────────────
check_command() {
    if ! command -v "$1" &> /dev/null; then
        echo "❌ Required tool not found: $1"
        echo "   Install: $2"
        exit 1
    fi
    echo "  ✅ $1 found: $(command -v $1)"
}

echo "📋 Checking prerequisites..."
check_command "go"       "https://go.dev/doc/install"
check_command "docker"   "https://docs.docker.com/engine/install/"
check_command "docker compose" "https://docs.docker.com/compose/install/"
check_command "protoc"   "brew install protobuf"
check_command "git"      "https://git-scm.com"
echo ""

# ── Setup .env ───────────────────────────────────────────────
if [ ! -f "$ROOT_DIR/.env" ]; then
    echo "📄 Creating .env from .env.example..."
    cp "$ROOT_DIR/.env.example" "$ROOT_DIR/.env"
    echo "  ⚠️  Please update .env with your actual values before running services"
fi

# ── Install Go tools ─────────────────────────────────────────
echo "🔧 Installing Go tools..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/swaggo/swag/cmd/swag@latest
go install github.com/vektra/mockery/v2@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/air-verse/air@latest   # Live reload for development
echo "  ✅ Go tools installed"
echo ""

# ── Generate proto files ─────────────────────────────────────
echo "📦 Generating proto files..."
bash "$SCRIPT_DIR/gen-proto.sh"
echo ""

# ── Start infrastructure ─────────────────────────────────────
echo "🐳 Starting Docker infrastructure (infra only, not services)..."
cd "$ROOT_DIR"
docker compose up -d postgres pgbouncer redis nats meilisearch minio minio-setup
echo "  ✅ Infrastructure started"
echo ""

# ── Wait for PostgreSQL ──────────────────────────────────────
echo "⏳ Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if docker compose exec postgres pg_isready -U xynpos -q 2>/dev/null; then
        echo "  ✅ PostgreSQL is ready"
        break
    fi
    echo "  Waiting... ($i/30)"
    sleep 2
done
echo ""

echo "╔═══════════════════════════════════════════════════════╗"
echo "║         Setup Complete! 🎉                            ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""
echo "Next steps:"
echo "  1. Update .env with your actual values"
echo "  2. Run migrations: make migrate-global"
echo "  3. Start a service: make run SVC=auth-service"
echo "  4. Start monitoring: docker compose --profile monitoring up -d"
echo ""
echo "Useful URLs:"
echo "  PostgreSQL (via PgBouncer): localhost:5433"
echo "  Redis:                      localhost:6379"
echo "  NATS:                       localhost:4222 | UI: localhost:8222"
echo "  Meilisearch:                localhost:7700"
echo "  MinIO Console:              localhost:9001"
echo "  Kong API Gateway:           localhost:8000 | Admin: localhost:8001"
echo "  Jaeger UI:                  localhost:16686 (after monitoring profile)"
echo "  Grafana:                    localhost:3001  (after monitoring profile)"
echo ""
