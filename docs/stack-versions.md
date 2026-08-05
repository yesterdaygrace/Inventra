# Stack Versions — Inventra (IMS Phase 1)

Status: RESOLVED 2026-08-05 by T1.0 version-lock spike (empirical: `go list -m -versions`, `npm view`, `go mod download`). Locked into `go.mod` (backend) — see commit `chore(deps): bootstrap go.mod with pinned backend deps`.

Toolchain rule: installed Go binary is 1.22.2; plan requires Go 1.24+. All go commands MUST run with `export GOTOOLCHAIN=go1.24.0` (auto-download verified working). `go.mod` declares `go 1.24.0`.

## Backend (Go) — locked in go.mod

| Dependency | Pinned version | Compat note |
|---|---|---|
| github.com/gin-gonic/gin | **v1.11.0** | v1.12.0 requires go >= 1.25 — DEVIATION from plan target v1.10.x upward to newest Go-1.24-compatible. |
| gorm.io/gorm | **v1.31.2** | requires go 1.18; newest available. Plan target v1.30.x exceeded by latest patch. |
| gorm.io/driver/postgres (pgx) | **v1.6.0** | v1.6.1+ require go >= 1.25 — DEVIATION: pinned newest Go-1.24-compatible. Pairs with gorm v1.31.2; uses jackc/pgx/v5 v5.6.0. |
| github.com/spf13/viper | **v1.21.0** | requires go 1.23; newest. Plan target v1.20.x exceeded by latest. |
| go.uber.org/zap | **v1.28.0** | requires go 1.19; newest. Plan target v1.27.x exceeded by latest. |
| github.com/go-playground/validator/v10 | **v10.30.0** | requires go 1.24.0 (OK). v10.30.3 requires go >= 1.25 — DEVIATION. |
| github.com/golang-jwt/jwt/v5 | **v5.3.1** | newest; plan target v5.2.x exceeded by latest. |
| github.com/google/uuid | **v1.6.0** | newest; matches plan target. |
| github.com/stretchr/testify | **v1.11.1** | requires go 1.17; newest. Plan target v1.10.x exceeded by latest. |
| github.com/swaggo/swag | **v1.16.6** | newest; requires go 1.18. |
| github.com/swaggo/gin-swagger | **v1.6.1** | newest. |
| github.com/swaggo/files | **v1.0.1** | newest. |
| github.com/gin-contrib/cors | **v1.7.6** | v1.7.7 requires go >= 1.25 — DEVIATION: pinned newest Go-1.24-compatible. |
| golangci-lint (CI tool) | **v2.x (latest v2 release)** | CI-stage install `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`; not a module dep. |

## Frontend (npm) — resolved for W11 install (NOT yet in package.json)

| Dependency | Current published | Note |
|---|---|---|
| react / react-dom | **19.2.8** | React 19.x confirmed current. |
| vite | **8.2.0** | DEVIATION: plan targeted Vite 7.x; current major is 8 — record for W11; verify shadcn/ui CLI support at install time. |
| tailwindcss | **4.3.3** | Tailwind v4 confirmed current. |
| @tailwindcss/vite | **4.3.3** | Tailwind v4 uses this Vite plugin (no postcss config needed). |
| @tanstack/react-query | **5.101.4** | v5 current. |
| react-hook-form | **7.84.0** | v7 current. |
| @hookform/resolvers | **5.7.1** | Resolver bridge RHF -> zod; v5 supports zod v4 resolvers. |
| zod | **4.4.3** | v4 current. RHF v7 + zod v4 via @hookform/resolvers is supported (resolver package handles schema versions). |
| recharts | **3.10.1** | v3 current; shadcn-compatible. |
| lucide-react | **1.28.0** | current. |
| react-router-dom | **7.18.2** | v7 current (matches plan "React Router"). |
| typescript | **7.0.2** | DEVIATION: current is 7.x — record for W11 (tsc strict still applies; verify @types compat). |
| @types/react / @types/react-dom | **19.2.18 / 19.2.4** | match React 19. |
| shadcn/ui | CLI (versionless) | `npx shadcn@latest init`; Tailwind v4 supported. Verify Vite 8 compat at W11 install. |

## Compatibility investigations (spike conclusions)

1. **GORM + pgx driver**: `gorm.io/driver/postgres` IS the pgx-based GORM driver (wraps jackc/pgx/v5). Pair gorm.io/gorm v1.31.2 + driver v1.6.0. No fork needed (resolves PRD's "GORM + pgx" ambiguity).
2. **React 19 + Tailwind v4 + Vite + shadcn/ui**: Tailwind v4 is configured via the `@tailwindcss/vite` plugin in vite.config, no tailwind.config.js/postcss needed; shadcn/ui supports Tailwind v4. Watch: current Vite is major 8 (plan said 7) — confirm `shadcn init` and `@vitejs/plugin-react` support at W11.
3. **RHF v7 + zod v4**: supported through `@hookform/resolvers` v5 (`zodResolver`); zod v4 is accepted by resolvers v5. No incompatibility.
4. **Go 1.24 ceiling**: several newest Go deps (gin v1.12.0, gorm pgx driver v1.6.1+, validator v10.30.3, cors v1.7.7) now require Go >= 1.25. All pinned to newest Go-1.24-compatible releases (see table). If a later wave upgrades the toolchain, unpin these upward.
