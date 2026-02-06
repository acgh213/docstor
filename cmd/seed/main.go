package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/exedev/docstor/internal/auth"
)

func main() {
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Check if any users exist
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Fatalf("failed to query users: %v", err)
	}

	if count > 0 {
		fmt.Println("Database already has users. Skipping seed.")
		return
	}

	// Create tenant
	tenantID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO tenants (id, name) VALUES ($1, $2)
	`, tenantID, "Default Tenant")
	if err != nil {
		log.Fatalf("failed to create tenant: %v", err)
	}
	fmt.Printf("Created tenant: %s (Default Tenant)\n", tenantID)

	// Create admin user
	passwordHash, err := auth.HashPassword("admin123")
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}

	userID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, $3, $4)
	`, userID, "admin@example.com", "Admin User", passwordHash)
	if err != nil {
		log.Fatalf("failed to create user: %v", err)
	}
	fmt.Printf("Created user: %s (admin@example.com)\n", userID)

	// Create membership
	_, err = pool.Exec(ctx, `
		INSERT INTO memberships (tenant_id, user_id, role) VALUES ($1, $2, $3)
	`, tenantID, userID, "admin")
	if err != nil {
		log.Fatalf("failed to create membership: %v", err)
	}
	fmt.Println("Created admin membership")

	fmt.Println("\n=== Seed Complete ===")
	fmt.Println("Login with:")
	fmt.Println("  Email: admin@example.com")
	fmt.Println("  Password: admin123")
	fmt.Println("\n⚠️  Change this password in production!")
}
