.PHONY: help test test-coverage lint clean install-tools db-up db-down
.PHONY: version validate-version update-deps bump-patch bump-minor bump-major
.PHONY: release release-dry-run release-patch release-minor release-major

# Get current version
VERSION := $(shell cat .version 2>/dev/null || echo "0.0.0")

# Default target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  test              - Run all tests"
	@echo "  test-coverage     - Run tests with coverage"
	@echo "  lint              - Run linters"
	@echo "  clean             - Clean build artifacts"
	@echo ""
	@echo "Version Management:"
	@echo "  version           - Show current version"
	@echo "  validate-version  - Validate .version file"
	@echo "  update-deps       - Update gostratum dependencies"
	@echo "  bump-patch        - Bump patch version (0.0.X)"
	@echo "  bump-minor        - Bump minor version (0.X.0)"
	@echo "  bump-major        - Bump major version (X.0.0)"
	@echo ""
	@echo "Release Management:"
	@echo "  release           - Create new release (default: patch)"
	@echo "  release-patch     - Create patch release"
	@echo "  release-minor     - Create minor release"
	@echo "  release-major     - Create major release"
	@echo "  release-dry-run   - Test release without committing"
	@echo ""
	@echo "Database (Development):"
	@echo "  db-up             - Start PostgreSQL database (Docker)"
	@echo "  db-down           - Stop PostgreSQL database"
	@echo ""
	@echo "Current version: v$(VERSION)"

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

# Version management
version:
	@echo "Current version: v$(VERSION)"

validate-version:
	@./scripts/validate-version.sh

update-deps:
	@./scripts/update-deps.sh

bump-patch:
	@./scripts/bump-version.sh patch

bump-minor:
	@./scripts/bump-version.sh minor

bump-major:
	@./scripts/bump-version.sh major

# Release management
release:
	@./scripts/release.sh $(or $(TYPE),patch)

release-dry-run:
	@DRY_RUN=true ./scripts/release.sh $(or $(TYPE),patch)

release-patch:
	@./scripts/release.sh patch

release-minor:
	@./scripts/release.sh minor

release-major:
	@./scripts/release.sh major


