.PHONY: help test test-coverage lint clean install-tools db-up db-down

# Default target
help:
	@echo "Available targets:"
	@echo "  test              - Run all tests"
	@echo "  test-coverage     - Run tests with coverage"
	@echo "  lint              - Run linters"
	@echo "  clean             - Clean build artifacts"
	@echo "  install-tools     - Install required development tools"
	@echo "  db-up             - Start PostgreSQL database (Docker)"
	@echo "  db-down           - Stop PostgreSQL database"

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

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f coverage.out coverage.html
	go clean -cache

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


