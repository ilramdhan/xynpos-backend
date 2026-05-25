// RULE-CI-014: Stub package for golangci-lint CI runs.
// When golangci-lint runs with -tags=cilint, the real auth_server.go is excluded
// (it has //go:build !cilint) because its import of shared/proto/auth cannot be
// resolved by golangci-lint v2's type checker in workspace mode.
// This stub provides the package declaration so cmd/main.go can still import it.
//go:build cilint

package grpc
