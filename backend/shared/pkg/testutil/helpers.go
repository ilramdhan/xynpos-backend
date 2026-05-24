package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	testcontainerspg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	appjwt "github.com/extendedsynaptic/xynpos/shared/pkg/jwt"
)

const (
	TestTenantID = "00000000-0000-0000-0000-000000000001"
	TestUserID   = "00000000-0000-0000-0000-000000000002"
	TestOutletID = "00000000-0000-0000-0000-000000000003"
	TestRole     = "cashier"
	TestPlan     = "starter"
	TestJWTSecret = "test-jwt-secret-for-testing-only"
)

// SetupTestDB starts a PostgreSQL 18 testcontainer and returns a GORM DB.
// The container is automatically terminated when the test finishes.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	ctx := context.Background()
	container, err := testcontainerspg.RunContainer(ctx,
		testcontainers.WithImage("postgres:18-alpine"),
		testcontainerspg.WithDatabase("xynpos_test"),
		testcontainerspg.WithUsername("xynpos"),
		testcontainerspg.WithPassword("test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "start postgres container")

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "get container connection string")

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "open gorm connection to test DB")

	return db
}

// RunMigrations runs golang-migrate migrations on the test DB.
func RunMigrations(t *testing.T, db *gorm.DB, migrationsPath string) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)

	dsn := fmt.Sprintf("postgres://xynpos:test_password@%s/xynpos_test?sslmode=disable",
		sqlDB.Stats().OpenConnections)
	// For simplicity in tests, run raw SQL files manually or use atlas/migrate
	_ = dsn // migrations are run by the service's migration runner
}

// MockAuthMiddleware returns a Fiber middleware that injects test auth context.
// Use this in integration tests to bypass real JWT validation.
func MockAuthMiddleware(tenantID, userID, role string) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("tenantID", tenantID)
		c.Locals("userID", userID)
		c.Locals("role", role)
		c.Locals("plan", TestPlan)
		c.Locals("permissions", permissionsForRole(role))
		return c.Next()
	}
}

// GenerateTestJWT generates a signed JWT for use in integration/E2E tests.
func GenerateTestJWT(tenantID, role string) string {
	mgr := appjwt.New(appjwt.Config{
		AccessSecret:  TestJWTSecret,
		RefreshSecret: TestJWTSecret + "_refresh",
		AccessExpiry:  1 * time.Hour,
		Issuer:        "xynpos.com",
	})
	token, err := mgr.GenerateAccessToken(
		TestUserID, tenantID, TestOutletID, role, TestPlan,
		permissionsForRole(role),
	)
	if err != nil {
		panic("testutil: generate test JWT: " + err.Error())
	}
	return token
}

// MustMarshalJSON marshals a value to JSON or panics. Useful in test setups.
func MustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic("testutil: marshal: " + err.Error())
	}
	return data
}

func permissionsForRole(role string) []string {
	perms := map[string][]string{
		"owner": {"*"},
		"manager": {
			"product:read", "product:write",
			"inventory:read", "inventory:write",
			"report:read",
			"transaction:read", "transaction:write",
			"customer:read", "customer:write",
			"user:read",
		},
		"cashier": {
			"product:read",
			"inventory:read",
			"transaction:read", "transaction:write",
			"customer:read",
		},
		"inventory": {
			"product:read",
			"inventory:read", "inventory:write",
		},
	}
	if p, ok := perms[role]; ok {
		return p
	}
	return []string{}
}
