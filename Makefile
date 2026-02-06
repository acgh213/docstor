.PHONY: dev build run test clean db-up db-down migrate seed

# Development
dev: db-up
	@echo "Starting Docstor in development mode..."
	go run ./cmd/docstor

# Build
build:
	go build -o bin/docstor ./cmd/docstor

# Run binary
run: build
	./bin/docstor

# Tests
test:
	go test -v ./...

# Clean
clean:
	rm -rf bin/

# Database
db-up:
	docker compose up -d postgres
	@echo "Waiting for Postgres to be ready..."
	@sleep 2

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

# Migrations (using golang-migrate CLI if installed)
migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir internal/db/migrations -seq $$name

# Seed development data
seed:
	go run ./cmd/seed

# Full reset
reset: db-down
	docker volume rm switch-dune_postgres_data 2>/dev/null || true
	$(MAKE) db-up
	@sleep 3
	$(MAKE) dev
