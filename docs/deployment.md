# Deployment & Operations Runbook

**Document Version:** 1.0
**Phase:** 3 — Docs + Infra
**Date:** 2026-08-06
**Status:** SPEC-FIRST — authoritative contract for T10.2 (Docker/compose) and T10.3 (CI).
Any deviation in the implementation must be reflected back into this document.

---

## 1. Deployment Topology

Two runtimes compose the stack. Both are fully containerized.

```
┌─────────────────────────────────────────────────────────────┐
│  docker compose  (project: inventra)                         │
│                                                             │
│  ┌───────────────┐   depends_on: healthy   ┌──────────────┐ │
│  │     api       │ ───────────────────────►│     db       │ │
│  │  Go 1.24      │   network: inventra-net │  postgres    │ │
│  │  Gin API      │                         │  17-alpine   │ │
│  │  :8080        │                         │  :5432       │ │
│  └───────┬───────┘                         └───┬──────────┘ │
│          │                                     │            │
│  host 127.0.0.1:8080        named volume ┌─────▼──────┐     │
│                                     │ inventra_pgdata │     │
```

- **api** — the Go HTTP service (`cmd/server`). Multi-stage image
  (`golang:1.24-alpine` build → `alpine` runtime). Exposes `:8080`.
- **db** — PostgreSQL 17 on Alpine. Internal `:5432`; host port mapped to
  `5433` to coexist with other local Postgres instances.
- **Storage** — a Docker **named volume** (`inventra_pgdata`) is the only
  persistent state. `docker compose down` must NOT remove it; only
  `docker compose down -v` wipes data.
- The frontend (Vite, `web/`) is added to the compose file in the frontend
  phase (W11+). Nothing in the backend deployment depends on it.

---

## 2. Environment variables

Full variable set consumed by `internal/shared/config`:

| Variable | Required | Default | Notes |
|---|---|---|---|
| `PORT` | no | `8080` | API listen port |
| `APP_ENV` | no | `development` |development/production |
| `DB_HOST` | yes | `localhost` | container value: `db` |
| `DB_PORT` | yes | `5432` | compose container port |
| `DB_USER` | **yes** | — | compose default: `postgres` |
| `DB_PASSWORD` | **yes** | — | **must not be the shipped default in prod** |
| `DB_NAME` | **yes** | — | compose default: `inventory` |
| `DB_SSLMODE` | no | `disable` | keep `disable` for in-network compose |
| `JWT_SECRET` | **yes** | — | **secret — see §6** |
| `JWT_ACCESS_TTL` | no | `15m` | |
| `JWT_REFRESH_TTL` | no | `168h` | |
| `BCRYPT_COST` | no | `12` | |
| `LOW_STOCK_THRESHOLD` | no | `10` | default for new products |
| `CORS_ORIGINS` | no | `` | comma-separated allowed origins |
| `LOG_LEVEL` | no | `info` | debug/info/warn/error |

Missing required vars (`DB_USER`, `DB_PASSWORD`, `DB_NAME`, `JWT_SECRET`)
cause the API to refuse to start (`missing required configuration`). See
`.env.example` (documented in §7).

## 3. Volume strategy

- **DB data** lives on `inventra_pgdata` (named volume). Survives
  `docker compose down`. Wiped only by `down -v`.
- **No other state.** The API is stateless; JWTs are stateless (HMAC); audit
  log, categories, products, inventory, transactions all live in Postgres.
  Nothing on the web/services filesystem matters across restarts.

## 4. Makefile docker targets

To be implemented in T10.2 matching this contract exactly:

| Target | Behaviour |
|---|---|
| `make docker-up` | `docker compose up -d --build` (db + api) |
| `make docker-down` | `docker compose down` (keeps volume) |
| `make docker-logs` | `docker compose logs -f api` |

Local parity: the same DB image can run standalone for development on `:5433`
(that instance already runs as `inventory-pg`). CI/local differences are in
§5.

## 5. Local vs CI differences

| Concern | Local (`docker compose`) | CI (GitHub Actions) |
|---|---|---|
| Postgres run-time | container, `postgres:17-alpine` with healthcheck | service container `postgres:17-alpine`, health via `pg_isready` |
| Test DB | the compose `db`, port 5433 | ephemeral per-job service |
| Lint/vet/test | `make pre-commit` | workflow steps (§6 below) |
| Coverage gate | local `go test -cover` informs | `awk` gate enforces ≥ 80% |
| Frontend build | separate vite command | `tsc --noEmit` + `vite build` job |

## 6. CI pipeline steps (to be implemented in T10.3)

`.github/workflows/ci.yml`:
1. **gofmt** — `gofmt -l` on all `.go` files must be empty.
2. **go vet** — `go vet ./...` clean.
3. **golangci-lint** — v2 config `golangci.yml`, default linters, no blockers.
4. **go test -cover** — `-covermode=atomic -coverprofile`; an `awk` block
   fails the job if the **per-package** coverage falls below the 80% gate.
5. **Frontend build** — `npm ci && npx tsc --noEmit && npx vite build` in
   `web/` (empty-safe until W11).

Gates fail the workflow; a pull request cannot merge with a failing CI.

## 7. Secrets note (`JWT_SECRET`)

- `JWT_SECRET` signs both access and refresh tokens. It is the single most
  important secret for the API.
- Local dev uses `dev-only-seed-secret` (see `Makefile SEED_DB`). Compose
  must override this via `.env`/environment, never a baked default that the
  source tree calls production-grade.
- **Production must** derive `JWT_SECRET` (and `DB_PASSWORD`) from a real
  secrets manager (e.g. Docker secrets, vault, CI secrets) — not from
  `.env.example`.
- The seeded default admin password `Admin123!` must be rotated after first
  login (see `README` §security).

## 8. Reference: reproduction (T10.2 TDD acceptance)

Given a clean `docker compose up`:

1. `docker compose ps` → `api` state `healthy`; `db` `healthy`.
2. `curl localhost:8080/healthz` → `{"status":"ok"}`.
3. `docker compose down` → containers stop; `docker volume ls` still lists
   `inventra_pgdata`.
4. `docker compose up -d` again → previous data intact (volume persisted).

This is the acceptance contract `docs/deployment.md` documents and the
Docker/compose implementation must satisfy to be declared complete.