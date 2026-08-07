.PHONY: dev build test test-cover lint run docker-up docker-down docker-logs seed seed-demo swagger coverage coverage-gate pre-commit

# Coverage gate: application code under internal/ (cmd/ entrypoints and
# docs/swagger generated code are uncoveable boilerplate).
COVERAGE_MIN := 80

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

# Development mode with hot-reload (requires air: go install github.com/air-verse/air@latest)
dev:
	@command -v air >/dev/null 2>&1 || { echo "air not found — install: go install github.com/air-verse/air@latest"; exit 1; }
	air -c .air.toml

# Lint the codebase (golangci-lint v2)
lint:
	golangci-lint run ./...

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
coverage:
	$(GO) test ./internal/... -p 1 -timeout 120s -covermode=atomic -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | tail -1

# Enforce the coverage floor (COVERAGE_MIN). Fails with exit 1 below the gate.
coverage-gate: coverage
	@total=$$($(GO) tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	echo "coverage: $${total}% (min $(COVERAGE_MIN)%)"; \
	awk -v c="$$total" -v min="$(COVERAGE_MIN)" 'BEGIN { if (c + 0 < min + 0) { print "coverage gate FAILED: " c "% < " min "%"; exit 1 } else { print "coverage gate PASSED: " c "%" } }'

# Pre-commit checks
pre-commit: build test lint
	@echo "Pre-commit checks passed"
