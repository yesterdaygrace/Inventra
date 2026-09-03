# Inventra — Inventory Management System

A full-stack inventory management system: Go (Gin) REST API + PostgreSQL with a
Vite/TypeScript frontend. Contains role-based access control (ADMIN/STAFF),
products, categories, inventory movements, activity audit log, dashboard
aggregates, and CSV exports.

---

## Prerequisites

- [Go](https://go.dev/) `>= 1.24`
- [Docker](https://docs.docker.com/) + Docker Compose (for the DB and full stack)
- [Node.js](https://nodejs.org/) `>= 22` + npm (frontend only)
- PostgreSQL (optional — the compose file provides one)

## Quick start (Docker — recommended)

```bash
# 1. Configure secrets (never commit real values)
cp .env.example .env         # edit JWT_SECRET to a strong random value

# 2. Build + start the API and its Postgres
make docker-up               # docker compose up -d --build
curl localhost:8080/healthz  # -> {"status":"ok"}

# 3. Seed base data (roles + default admin)
make seed
```

If `:8080` or `:5432` are already taken on your host, override them:

```bash
API_PORT=8081 DB_PORT=5434 make docker-up
```

Stop the stack (data persists): `make docker-down`.

## Quick start (local dev)

```bash
# launch the dev Postgres on :5433 (or use your own)
docker run -d --name inventory-pg -p 5433:5432 \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=inventory \
  postgres:17-alpine

make seed          # roles + admin (uses :5433)
make seed-demo     # optional demo data
make run           # server on :8080 (DB_* env must match)
```

## Make targets

| Target | Purpose |
|---|---|
| `make build` | compile all packages |
| `make test` | run all tests |
| `make test-cover` | coverage report (HTML) |
| `make run` | run the API server |
| `make seed` / `make seed-demo` | seed base / demo data |
| `make docker-up` / `make docker-down` / `make docker-logs` | compose stack |
| `make swagger` | regenerate OpenAPI docs into `docs/swagger/` |
| `make pre-commit` | build + test + lint |

## Configuration (environment)

Required: `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `JWT_SECRET`. See
[docs/deployment.md](docs/deployment.md#2-environment-variables) for the full
table and `.env.example` for a template.

Optional: set `DEMO_MODE=true` to enable a password-free demo auto-login
endpoint — `POST /api/v1/auth/demo` returns tokens for a STAFF demo user
(`demo@inventory.local`, created on first use). Intended for development and
demoing only; never enable it in production.

## API

- **Swagger UI:** `GET /swagger/index.html` (after `make swagger` + run)
- **Route contract:** [docs/api.md](docs/api.md)
- **Healthcheck:** `GET /healthz`

## Architecture & docs

| Doc | Purpose |
|---|---|
| [docs/architecture.md](docs/architecture.md) | system architecture |
| [docs/api.md](docs/api.md) | authoritative API route contract |
| [docs/database.md](docs/database.md) | schema/data design |
| [docs/er.md](docs/er.md) | entity-relationship diagram |
| [docs/backend.md](docs/backend.md) | Go implementation standards |
| [docs/security.md](docs/security.md) | auth/RBAC security design |
| [docs/stack-versions.md](docs/stack-versions.md) | pinned toolchain versions |
| [docs/deployment.md](docs/deployment.md) | docker + CI deployment runbook |
| [docs/testing.md](docs/testing.md) | testing conventions |

Backend source lives under `internal/<module>/`: `auth`, `category`, `product`,
`warehouses`, `inventory`, `activitylog`, `user`, `dashboard`, `report`, plus `shared/` for
cross-cutting concerns (response envelope, middleware, errors, export, config,
database, logger, validator). Entrypoint: `cmd/server/main.go`.

## Roles & permissions

- **ADMIN** — full CRUD on all modules; user/role management; audit logs.
- **STAFF** — views, product/inventory operations (stock-in/out), no user
  management, no audit-log read.

## Security

- Passwords hashed with bcrypt; JWTs signed with `JWT_SECRET` (HS256), short
  access TTL + refresh token flow.
- **Rotate the seeded admin password** and **set a strong `JWT_SECRET`** before
  any non-local deployment. See [docs/security.md](docs/security.md).

## License

Proprietary — internal project.
