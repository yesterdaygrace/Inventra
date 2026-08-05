.PHONY: dev build test test-cover lint run docker-up docker-down seed seed-demo swagger pre-commit

# Go toolchain
GOTOOLCHAIN := go1.24.0
GO := GOTOOLCHAIN=$(GOTOOLCHAIN) go

# Build the application
build:
	$(GO) build ./...

# Run tests
test:
	$(GO) test ./...

# Run tests with coverage
test-cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Run the server (once implemented)
run:
	@test -f cmd/server/main.go || { echo "cmd/server not yet implemented"; exit 0; }
	$(GO) run ./cmd/server

# Development mode with auto-reload (placeholder)
dev:
	@echo "TODO: implement dev mode with air or similar"

# Lint the codebase (placeholder)
lint:
	@echo "TODO: implement golangci-lint"

# Start Docker services (placeholder)
docker-up:
	@echo "TODO: implement docker compose up"

# Stop Docker services (placeholder)
docker-down:
	@echo "TODO: implement docker compose down"

# Seed database (placeholder)
seed:
	@echo "TODO: implement database seeding"

# Seed demo data (placeholder)
seed-demo:
	@echo "TODO: implement demo data seeding"

# Generate Swagger docs (placeholder)
swagger:
	@echo "TODO: implement swagger generation"

# Pre-commit checks
pre-commit: build test lint
	@echo "Pre-commit checks passed"
