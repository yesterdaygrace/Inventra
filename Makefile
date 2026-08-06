.PHONY: dev build test test-cover lint run docker-up docker-down docker-logs seed seed-demo swagger pre-commit

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

# Run the server. Override the host port with API_PORT (default 8080):
# API_PORT=8090 DEMO_MODE=true make run
run:
	@test -f cmd/server/main.go || { echo "cmd/server not yet implemented"; exit 0; }
	$(SEED_DB) DEMO_MODE=$(DEMO_MODE) PORT=$(API_PORT) $(GO) run ./cmd/server

# Development mode with auto-reload (placeholder)
dev:
	@echo "TODO: implement dev mode with air or similar"

# Lint the codebase (placeholder)
lint:
	@echo "TODO: implement golangci-lint"

# Start the containerized stack (db + api)
docker-up:
	docker compose up -d --build

# Stop the stack (keeps the named volume)
docker-down:
	docker compose down

# Tail API logs
docker-logs:
	docker compose logs -f api

# Database connection for seed targets (dockerized dev Postgres on 5433)
SEED_DB := DB_HOST=localhost DB_PORT=5433 DB_USER=postgres DB_PASSWORD=postgres DB_NAME=inventory DB_SSLMODE=disable JWT_SECRET=dev-only-seed-secret

# Seed base data (roles + default admin) idempotently
seed: build
	$(SEED_DB) $(GO) run ./cmd/seed

# Seed opt-in demo data idempotently
seed-demo: build
	$(SEED_DB) $(GO) run ./cmd/seed demo

# Regenerate OpenAPI docs into docs/swagger
swagger:
	$(GO) run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -o docs/swagger

# Pre-commit checks
pre-commit: build test lint
	@echo "Pre-commit checks passed"
