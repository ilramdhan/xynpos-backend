FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build deps
RUN apk add --no-cache git ca-certificates

# Copy shared module first (for caching)
COPY shared/ /workspace/shared/

# Copy service
COPY services/tenant-service/ /workspace/services/tenant-service/

WORKDIR /workspace/services/tenant-service

# Download dependencies
RUN go mod download

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /app/tenant-service \
    ./cmd/main.go

# ── Runtime image ───────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy binary and swagger docs
COPY --from=builder /app/tenant-service .
COPY --from=builder /workspace/services/tenant-service/docs ./docs

# Metadata
LABEL org.opencontainers.image.title="XynPOS Tenant Service" \
      org.opencontainers.image.description="Multi-tenant management service" \
      org.opencontainers.image.version="1.0"

EXPOSE 9000 9001 10000

USER nonroot:nonroot

ENTRYPOINT ["/app/tenant-service"]
