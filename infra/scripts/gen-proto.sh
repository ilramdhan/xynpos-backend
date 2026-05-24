#!/usr/bin/env bash
# =============================================================
# XynPOS — Proto generation script
# Generates Go code from .proto files in backend/shared/proto/
# =============================================================
# Requirements:
#   brew install protobuf
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# =============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
PROTO_DIR="$ROOT_DIR/backend/shared/proto"

echo "🔧 Generating proto files from: $PROTO_DIR"

for SERVICE_DIR in "$PROTO_DIR"/*/; do
    SERVICE=$(basename "$SERVICE_DIR")
    PROTO_FILE="$SERVICE_DIR/$SERVICE.proto"

    if [ ! -f "$PROTO_FILE" ]; then
        echo "  ⚠️  No $SERVICE.proto found in $SERVICE_DIR, skipping"
        continue
    fi

    echo "  ▶ Generating: $SERVICE.proto"

    protoc \
        --proto_path="$PROTO_DIR" \
        --go_out="$PROTO_DIR/$SERVICE" \
        --go_opt=paths=source_relative \
        --go-grpc_out="$PROTO_DIR/$SERVICE" \
        --go-grpc_opt=paths=source_relative \
        "$PROTO_FILE"

    echo "  ✅ Generated: $SERVICE.pb.go + ${SERVICE}_grpc.pb.go"
done

echo ""
echo "✅ All proto files generated successfully"
echo ""
echo "Generated files are committed to the repo (single source of truth for BE-to-BE gRPC)"
