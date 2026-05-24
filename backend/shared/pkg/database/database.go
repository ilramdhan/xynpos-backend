package database

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Config holds database connection configuration.
type Config struct {
	URL          string
	MaxOpenConns int
	MinOpenConns int
	MaxIdleTime  time.Duration
}

// New creates a new GORM database connection.
// Designed to connect through PgBouncer (session mode).
func New(cfg Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.URL,
		PreferSimpleProtocol: true, // Required for PgBouncer compatibility
	}), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(gormlogger.Silent), // Use Zap instead
		PrepareStmt:            false,                                          // Disable for PgBouncer
		NamingStrategy:         schema.NamingStrategy{SingularTable: false},
		SkipDefaultTransaction: false,
	})
	if err != nil {
		return nil, fmt.Errorf("database: open connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MinOpenConns)
	sqlDB.SetConnMaxIdleTime(cfg.MaxIdleTime)

	return db, nil
}

// validSchemaName matches only tenant_<hex-uuid-without-dashes> patterns.
// This prevents SQL injection when setting search_path.
var validSchemaName = regexp.MustCompile(`^tenant_[a-z0-9]{8}[a-z0-9]{4}[a-z0-9]{4}[a-z0-9]{4}[a-z0-9]{12}$`)

// ValidateSchemaName validates that a schema name is safe to use in SQL.
func ValidateSchemaName(schemaName string) error {
	if !validSchemaName.MatchString(schemaName) {
		return fmt.Errorf("database: invalid schema name %q: must match pattern tenant_<uuid-no-dashes>", schemaName)
	}
	return nil
}

// TenantSchemaName returns the PostgreSQL schema name for a tenant.
// e.g. "550e8400-e29b-41d4-a716-446655440000" → "tenant_550e8400e29b41d4a716446655440000"
func TenantSchemaName(tenantID string) string {
	// Remove dashes from UUID
	clean := make([]byte, 0, len(tenantID))
	for i := 0; i < len(tenantID); i++ {
		if tenantID[i] != '-' {
			clean = append(clean, tenantID[i])
		}
	}
	return "tenant_" + string(clean)
}

// WithTenantSchema returns a new *gorm.DB scoped to the tenant's schema.
// This sets the search_path on the current session (PgBouncer session mode required).
func WithTenantSchema(ctx context.Context, db *gorm.DB, tenantID string) (*gorm.DB, error) {
	schemaName := TenantSchemaName(tenantID)
	if err := ValidateSchemaName(schemaName); err != nil {
		return nil, err
	}

	// Set search_path — this works only in PgBouncer session mode
	scoped := db.WithContext(ctx).Exec("SET search_path = " + schemaName)
	if scoped.Error != nil {
		return nil, fmt.Errorf("database: set search_path=%s: %w", schemaName, scoped.Error)
	}

	return db.WithContext(ctx).Scopes(func(db *gorm.DB) *gorm.DB {
		return db.Table(schemaName + "." + db.Statement.Table)
	}), nil
}

// SetSearchPath sets the PostgreSQL search_path for the current session.
// Used by the tenant middleware on each request.
func SetSearchPath(db *gorm.DB, schemaName string) error {
	if err := ValidateSchemaName(schemaName); err != nil {
		return err
	}
	// Use raw SQL - safe because we validated schemaName above
	return db.Exec("SET LOCAL search_path = " + schemaName + ", public_xyn, public").Error
}

// Ping verifies the database connection is alive.
func Ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database: get sql.DB: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database: ping failed: %w", err)
	}
	return nil
}

// Close gracefully closes the database connection.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database: get sql.DB for close: %w", err)
	}
	return sqlDB.Close()
}
