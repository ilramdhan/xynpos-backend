package database

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// MigrateGlobal runs migrations on the global schema (public_xyn).
// migrationsPath: path to the migrations directory e.g. "migrations/global"
func MigrateGlobal(databaseURL, migrationsPath string) error {
	m, err := migrate.New(
		"file://"+migrationsPath,
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("migrate global: create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate global: run up: %w", err)
	}
	return nil
}

// MigrateTenant runs migrations on a specific tenant schema.
// It creates the schema first if it doesn't exist.
func MigrateTenant(databaseURL, tenantID, migrationsPath string) error {
	schemaName := TenantSchemaName(tenantID)
	if err := ValidateSchemaName(schemaName); err != nil {
		return fmt.Errorf("migrate tenant: %w", err)
	}

	// Append search_path to DSN so golang-migrate uses the right schema
	// Postgres DSN: postgres://user:pass@host/db?search_path=tenant_xxx
	tenantDSN := appendSearchPath(databaseURL, schemaName)

	m, err := migrate.New(
		"file://"+migrationsPath,
		tenantDSN,
	)
	if err != nil {
		return fmt.Errorf("migrate tenant %s: create migrator: %w", tenantID, err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate tenant %s: run up: %w", tenantID, err)
	}
	return nil
}

// appendSearchPath appends the search_path parameter to a PostgreSQL DSN.
func appendSearchPath(dsn, schemaName string) string {
	// Handle both URL-format and key=value format
	// Simple approach: append as query param
	separator := "?"
	for _, ch := range dsn {
		if ch == '?' {
			separator = "&"
			break
		}
	}
	return dsn + separator + "search_path=" + schemaName
}
