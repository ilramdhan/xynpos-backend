# XynPOS Backend — Engineering Rules & Known Gotchas

> **Purpose:** This file is the single source of truth for recurring bugs, CI/CD gotchas,
> and project-wide rules. Update this file whenever you fix a non-obvious issue.
> AI agents and engineers MUST read this before making changes.

---

## Table of Contents
1. [Go Module & Version Rules](#1-go-module--version-rules)
2. [GitHub Actions CI/CD Rules](#2-github-actions-cicd-rules)
3. [Proto / gRPC Rules](#3-proto--grpc-rules)
4. [Test & Coverage Rules](#4-test--coverage-rules)
5. [Domain / Validation Rules](#5-domain--validation-rules)
6. [Docker & Build Rules](#6-docker--build-rules)
7. [Config & Environment Rules](#7-config--environment-rules)

---

## 1. Go Module & Version Rules

### RULE-GO-001: Go version in go.mod must be ≤ CI runner Go version
**Problem:** If `go.mod` says `go 1.26.2` but golangci-lint is built with Go 1.24,
lint will fail with: `can't load config: the Go language version (go1.24) used to build
golangci-lint is lower than the targeted Go version (1.26.2)`

**Rule:** The `go` directive in ALL `go.mod` files must be pinned to the same version
as `GO_VERSION` in CI workflows. Currently pinned to **`go 1.22`**.

```
# Correct ✅
go 1.22

# Wrong ❌ — will break golangci-lint in CI
go 1.26.2
```

**When bumping Go version:** Update ALL these files simultaneously:
- `backend/shared/go.mod`
- `backend/services/auth-service/go.mod`
- `backend/services/tenant-service/go.mod`
- `backend/services/*/go.mod` (all future services)
- `.github/workflows/ci.yml` → `GO_VERSION`
- `.github/workflows/ci-auth-service.yml` → `GO_VERSION`

---

### RULE-GO-002: run go mod tidy after changing go.mod go version
After manually editing the `go` directive, always run `go mod tidy` in each module directory.
This updates `go.sum` and removes any toolchain-specific artifacts.

```bash
cd backend/shared && go mod tidy
cd backend/services/auth-service && go mod tidy
cd backend/services/tenant-service && go mod tidy
```

---

### RULE-GO-004: `go mod tidy` will UPGRADE the go directive — reset it manually
**Problem:** Running `go mod tidy` automatically upgrades the `go` directive in `go.mod` to the
current toolchain version (e.g., 1.22 → 1.25 → 1.26). This silently breaks CI.

**Rule:** After every `go mod tidy`, immediately verify and reset if needed:
```bash
# Check versions after tidy
grep "^go " backend/shared/go.mod backend/services/*/go.mod go.work

# Reset if upgraded beyond pinned version
sed -i 's/^go 1\.[0-9][0-9]*\.[0-9]*/go 1.22/' \
  backend/shared/go.mod \
  backend/services/auth-service/go.mod \
  backend/services/tenant-service/go.mod \
  go.work
```

Also applies to `go.work` — it has its own `go` directive that must stay in sync.

---

### RULE-CI-008: Run CI tests from service working-directory, use go.work for cross-module resolution
**Problem:** gRPC tests import `shared/proto/auth` via `replace` directive.
In CI, when tests run from `backend/services/auth-service`, the `replace ../../shared`
path is resolved correctly because the full repo is checked out AND `go.work` at the
repo root lists all modules. The `go.work` file enables Go workspace mode automatically.

**Rule:** Keep `go.work` up-to-date with all `use` entries. Every new service must be added.
`go.work` must have the same `go` version directive as `go.mod` files.

---

### RULE-CI-009: go.work has its own `go` directive — must pin it too
`go.work` has `go 1.xx` directive that must match the pinned version in all `go.mod` files.
`go mod tidy` does NOT touch `go.work`, but `go work sync` will upgrade it.

```
# go.work must match go.mod version
go 1.22

use (
  ./backend/services/auth-service
  ./backend/services/tenant-service
  ./backend/shared
)
```

---

### RULE-GO-003: Shared module proto packages resolve via replace directive
The `shared/proto/auth` package is part of the `shared` Go module. Services access it via:
```
replace github.com/extendedsynaptic/xynpos/shared => ../../shared
```
This replace path is **relative to the go.mod file location**, NOT to CWD.
The generated proto files (`auth.pb.go`, `auth_grpc.pb.go`) are committed to the repo
and versioned in `backend/shared/proto/auth/`.

**Do NOT** create a separate module for proto packages — it creates circular dependency risk.

---

## 2. GitHub Actions CI/CD Rules

### RULE-CI-001: `env.*` context NOT available in `concurrency.group`
**Problem:** Using `${{ env.SERVICE }}` inside `jobs.<job>.concurrency.group` causes:
```
Unrecognized named-value: 'env'. Located at position 1 within expression: env.SERVICE
```

**Rule:** The `concurrency.group` field only supports `github.*`, `inputs.*`, and literal strings.
Use hardcoded service names:

```yaml
# Wrong ❌
concurrency:
  group: deploy-staging-${{ env.SERVICE }}

# Correct ✅
concurrency:
  group: deploy-staging-auth-service-${{ github.ref }}
```

---

### RULE-CI-002: `cache-dependency-path` is relative to REPO ROOT, not working-directory
**Problem:** When using `defaults.run.working-directory`, Go cache action still expects
paths relative to the **repository root**.

```yaml
# Wrong ❌ — relative to working-directory
- uses: actions/setup-go@v5
  with:
    cache-dependency-path: go.sum

# Correct ✅ — relative to repo root
- uses: actions/setup-go@v5
  with:
    cache-dependency-path: backend/services/auth-service/go.sum
```

---

### RULE-CI-003: Use `go test ./internal/...` NOT `./...` in CI
**Problem:** Running `go test ./...` includes `cmd/`, `docs/`, `event/`, `repository/`
packages that may require external tools (`swag`, `covdata`) not available in CI runners.
This causes:
```
go: no such tool "covdata"
```

**Rule:** In CI test jobs, always scope tests to `./internal/...` to test only
the testable business logic layers:

```yaml
# Wrong ❌
run: go test ./...

# Correct ✅
run: go test ./internal/...
```

---

### RULE-CI-004: Always install protoc Go plugins before running protoc
**Problem:** `sudo apt-get install -y protobuf-compiler` only installs the `protoc` binary.
It does NOT install `protoc-gen-go` or `protoc-gen-go-grpc`. Running protoc without these
plugins causes:
```
protoc-gen-go: program not found or is not executable
--go_out: protoc-gen-go: Plugin failed with status code 1.
```

**Rule:** Always install both Go plugins before protoc:
```yaml
- name: Install protoc
  run: sudo apt-get install -y protobuf-compiler

- name: Install Go protoc plugins
  run: |
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    echo "$(go env GOPATH)/bin" >> $GITHUB_PATH
```

---

### RULE-CI-005: Pin golangci-lint version, never use `latest`
**Problem:** `version: latest` in golangci-lint-action can pull a version built with a
newer Go that conflicts with go.mod, or introduce breaking linter changes.

**Rule:** Always pin the version. Currently using **`v1.64.8`**. Update only deliberately.

```yaml
- uses: golangci/golangci-lint-action@v6
  with:
    version: v1.64.8  # Pinned — do NOT use 'latest'
```

---

### RULE-CI-006: golangci-lint `working-directory` needs `--config` path adjustment
When using `working-directory` option in golangci-lint-action, the `--config` path
is relative to the `working-directory`, not the repo root.

```yaml
# The .golangci.yml is at repo root (../../.golangci.yml from service dir)
args: --config=../../.golangci.yml --timeout 5m
```

---

### RULE-CI-007: Measure coverage PER LAYER, not aggregate across all packages
**Problem:** `go tool cover -func=coverage.out | grep total` returns coverage across ALL
packages in the coverage profile, including `domain/` and `repository/postgres/` which
have no unit tests. This dilutes the metric:
- delivery: 80%, usecase: 77% → aggregate total: **46%** ❌ (falsely fails 70% gate)

**Rule:** Check coverage per testable layer using awk averaging:
```bash
HANDLER=$(go tool cover -func=coverage.out | grep '/delivery/' | \
  grep -v '_test' | \
  awk '{sum+=$3; count++} END {if(count>0) printf "%.1f", sum/count; else print "0"}' | tr -d '%')
```
Gate: delivery layer avg >= 70% (hard fail). Usecase warn-only (grows with integration tests).

---

## 3. Proto / gRPC Rules

### RULE-PROTO-001: Commit generated proto files to the repo
Generated files (`*.pb.go`, `*_grpc.pb.go`) MUST be committed to the repository.
Do NOT add them to `.gitignore`.

**Reason:** CI runners don't have the same proto toolchain as local dev.
Having generated files committed ensures CI can build without running protoc.

Location: `backend/shared/proto/<service>/`

---

### RULE-PROTO-002: Proto package path must match shared module import path
```proto
option go_package = "github.com/extendedsynaptic/xynpos/shared/proto/auth;authpb";
```
This ensures the generated Go package has the correct import path that matches
the `replace` directive in service go.mod files.

---

### RULE-PROTO-003: gRPC server must implement ALL interface methods
When implementing a gRPC server from a proto-generated interface:
- The interface includes `mustEmbedUnimplemented*Server()` method
- Always embed `pb.Unimplemented*Server` in the concrete struct

```go
// Correct ✅
type AuthServer struct {
    authpb.UnimplementedAuthServiceServer  // embeds all unimplemented methods
    uc  domain.AuthUsecase
    log *zap.Logger
}
```

---

## 4. Test & Coverage Rules

### RULE-TEST-001: Coverage threshold is 70% on internal layers only
Coverage gate applies to `./internal/...` packages only. `cmd/`, `docs/`, `event/`,
`repository/postgres/` are excluded (integration tested separately).

Target coverage per layer:
- handler (delivery/http): ≥ 70%
- usecase: ≥ 70%
- delivery/grpc: ≥ 80%

---

### RULE-TEST-002: `uuid4` validate tag does NOT work on `uuid.UUID` struct fields
**Problem:** Adding `validate:"uuid4"` on a `uuid.UUID` typed field (not string) causes
validation to always fail with `VALIDATION_ERROR` because the `uuid4` validator expects
a string representation, not a parsed UUID struct.

**Rule:** Only use `validate:"uuid4"` on `string` fields. On `uuid.UUID` fields, use
`validate:"required"` only.

```go
// Wrong ❌ — uuid4 on uuid.UUID type always fails validation
RoleID uuid.UUID `json:"role_id" validate:"required,uuid4"`

// Correct ✅
RoleID uuid.UUID `json:"role_id" validate:"required"`

// Also correct ✅ — uuid4 is fine on string
RoleIDStr string `json:"role_id" validate:"required,uuid4"`
```

---

### RULE-TEST-003: Handler tests must use `app.Test()` with `httptest.NewRequest()`
Use `fiber.App.Test()` for handler tests — it handles Fiber's internal context properly:

```go
req := httptest.NewRequest("GET", "/v1/endpoint", body)
req.Header.Set("Content-Type", "application/json")
resp, err := app.Test(req)
```

---

## 5. Domain / Validation Rules

### RULE-DOMAIN-001: InviteUserInput uses uuid.UUID for RoleID and OutletID
Do not change these to strings without also updating handler binding and test mocks.
The JSON binding handles UUID parsing automatically via fiber's bind.

---

## 6. Docker & Build Rules

### RULE-DOCKER-001: Dockerfile context is `./backend`, not the service directory
When building from docker-compose or CI, the build context is the `backend/` directory
so that shared module is accessible:

```yaml
# Correct ✅
context: ./backend
file: ./backend/services/auth-service/Dockerfile
```

```dockerfile
# In Dockerfile — copy shared BEFORE the service
COPY shared/ /workspace/shared/
COPY services/auth-service/ /workspace/services/auth-service/
```

---

### RULE-DOCKER-002: Use distroless/static-debian12 for minimal runtime image
All service Dockerfiles must use `gcr.io/distroless/static-debian12:nonroot` as the
runtime base image for minimal attack surface and small image size.

---

## 7. Config & Environment Rules

### RULE-CONFIG-001: Never hardcode secrets in config.yaml
`config.yaml` and `config.local.yaml` use `${ENV_VAR}` placeholders only.
Actual values come from:
1. `.env` file (local dev, gitignored)
2. Environment variables (production)
3. Docker Compose `environment:` block

### RULE-CONFIG-002: GRPCConfig fields in shared/pkg/config
All services share the `GRPCConfig` struct. New gRPC addresses must be added there:
- `GRPC_PORT` — this service's gRPC port
- `TENANT_SERVICE_GRPC_ADDR` — tenant-service gRPC address
- `AUTH_SERVICE_GRPC_ADDR` — auth-service gRPC address

---

*Last updated: 2026-05-24 | Maintained by: Claude AI (claude-ai@anthropic.com)*
