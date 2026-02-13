// Package testutil provides helpers for integration tests requiring a real Postgres database.
//
// Each test creates its own tenant, users, etc. with unique UUIDs embedded
// in emails to avoid cross-test and cross-package collisions.  No TRUNCATE
// is needed, so packages can run in parallel safely.
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acgh213/docstor/internal/auth"
	"github.com/acgh213/docstor/internal/db"
)

// TestDatabaseURL returns the connection string for the test database.
func TestDatabaseURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://docstor:docstor@localhost:5432/docstor_test?sslmode=disable"
}

// SetupDB connects to the test database and runs migrations.
// Each test creates isolated tenants so no TRUNCATE is needed.
func SetupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test (short mode)")
	}

	ctx := context.Background()
	url := TestDatabaseURL()

	// Run migrations (idempotent).
	if err := db.RunMigrations(url); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}

	t.Cleanup(func() { pool.Close() })
	return pool
}

// TestEnv bundles the pool so handler tests can access it.
type TestEnv struct {
	Pool *pgxpool.Pool
}

// uniqueEmail returns an email with a short UUID prefix to avoid
// UNIQUE constraint violations across parallel tests and re-runs.
func uniqueEmail(base string) string {
	return fmt.Sprintf("%s.%s", uuid.New().String()[:8], base)
}

// ---- Seed helpers ---------------------------------------------------------

// CreateTenant creates a tenant and returns its ID.
func CreateTenant(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO tenants (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	if err != nil {
		t.Fatalf("create tenant %q: %v", name, err)
	}
	return id
}

// CreateUser creates a user with a bcrypt-hashed password.
// The email is made globally unique by prepending a short UUID.
// Returns (userID, actualEmail).
func CreateUser(t *testing.T, pool *pgxpool.Pool, email, name, password string) uuid.UUID {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	email = uniqueEmail(email)
	var id uuid.UUID
	err = pool.QueryRow(context.Background(),
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		email, name, hash).Scan(&id)
	if err != nil {
		t.Fatalf("create user %q: %v", email, err)
	}
	return id
}

// CreateMembership links a user to a tenant with the given role.
func CreateMembership(t *testing.T, pool *pgxpool.Pool, tenantID, userID uuid.UUID, role string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO memberships (tenant_id, user_id, role) VALUES ($1, $2, $3) RETURNING id`,
		tenantID, userID, role).Scan(&id)
	if err != nil {
		t.Fatalf("create membership: %v", err)
	}
	return id
}

// CreateClient creates a client within a tenant.
func CreateClient(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, name, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO clients (tenant_id, name, code) VALUES ($1, $2, $3) RETURNING id`,
		tenantID, name, code).Scan(&id)
	if err != nil {
		t.Fatalf("create client %q: %v", name, err)
	}
	return id
}

// CreateSession creates a session and returns the raw token (not hash).
func CreateSession(t *testing.T, pool *pgxpool.Pool, userID, tenantID uuid.UUID) string {
	t.Helper()
	sm := auth.NewSessionManager(pool)
	token, err := sm.Create(context.Background(), userID, tenantID, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token
}

// Fixture bundles commonly needed IDs for a single tenant + user.
type Fixture struct {
	TenantID     uuid.UUID
	UserID       uuid.UUID
	MembershipID uuid.UUID
	SessionToken string
}

// QuickFixture creates a tenant, user, membership, and session in one call.
func QuickFixture(t *testing.T, pool *pgxpool.Pool, tenantName, email, role string) Fixture {
	t.Helper()
	tid := CreateTenant(t, pool, tenantName)
	uid := CreateUser(t, pool, email, fmt.Sprintf("User %s", email), "password123")
	mid := CreateMembership(t, pool, tid, uid, role)
	tok := CreateSession(t, pool, uid, tid)
	return Fixture{TenantID: tid, UserID: uid, MembershipID: mid, SessionToken: tok}
}
