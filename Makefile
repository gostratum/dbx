.PHONY: help test test-coverage lint build clean migrate-up migrate-down migrate-status migrate-create install-tools

# Default target
help:
	@echo "Available targets:"
	@echo "  test              - Run all tests"
	@echo "  test-coverage     - Run tests with coverage"
	@echo "  lint              - Run linters"
	@echo "  build             - Build the db-migrate CLI tool"
	@echo "  clean             - Clean build artifacts"
	@echo "  migrate-up        - Run all pending migrations (embedded)"
	@echo "  migrate-down      - Revert last migration"
	@echo "  migrate-status    - Show migration status"
	@echo "  migrate-create    - Create a new migration file (NAME=name_required)"
	@echo "  install-tools     - Install required development tools"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linters
lint:
	@echo "Running linters..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Run: make install-tools"; exit 1; }
	golangci-lint run ./...

# Build the CLI tool
build:
	@echo "Building db-migrate CLI..."
	go build -o bin/db-migrate ./cmd/db-migrate
	@echo "Binary created: bin/db-migrate"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache

# Migration targets

# Run migrations (using embedded migrations)
migrate-up:
	@echo "Running migrations (embedded)..."
	go run ./cmd/db-migrate up --embed

# Run migrations (using filesystem)
migrate-up-fs:
	@echo "Running migrations (filesystem)..."
	go run ./cmd/db-migrate up --dir ./migrate/files

# Revert last migration
migrate-down:
	@echo "Reverting last migration..."
	go run ./cmd/db-migrate down --embed

# Show migration status
migrate-status:
	@echo "Checking migration status..."
	go run ./cmd/db-migrate status --embed

# Create a new migration file
# Usage: make migrate-create NAME=add_users_table
migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Usage: make migrate-create NAME=add_users_table"; \
		exit 1; \
	fi
	@timestamp=$$(date +%s); \
	version=$$(printf "%06d" $$timestamp); \
	up_file="migrate/files/$${version}_$(NAME).up.sql"; \
	down_file="migrate/files/$${version}_$(NAME).down.sql"; \
	echo "-- Add migration SQL here" > $$up_file; \
	echo "-- Add rollback SQL here" > $$down_file; \
	echo "Created migration files:"; \
	echo "  $$up_file"; \
	echo "  $$down_file"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@command -v golangci-lint >/dev/null 2>&1 || \
		(echo "Installing golangci-lint..." && \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@echo "Tools installed successfully"

# Database setup for local development (requires Docker)
db-up:
	@echo "Starting PostgreSQL database..."
	docker run --name dbx-postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=dbx_dev \
		-p 5432:5432 \
		-d postgres:16-alpine
	@echo "Waiting for database to be ready..."
	@sleep 3
	@echo "Database ready at: postgres://postgres:postgres@localhost:5432/dbx_dev?sslmode=disable"

# Stop database
db-down:
	@echo "Stopping PostgreSQL database..."
	docker stop dbx-postgres || true
	docker rm dbx-postgres || true

# Run example application
example:
	@echo "Running example application..."
	cd example && go run .
