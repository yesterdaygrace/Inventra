# ims-phase1 - Work Plan
## TL;DR (For humans)

**What you'll get**: The complete Phase 1 Enterprise Inventory Management System — a Go modular monolith backend (Gin + GORM + PostgreSQL 17: auth with JWT/refresh rotation + RBAC, users, products, categories, inventory stock in/out with history, activity logs, dashboard + reports with charts and CSV export, Swagger, Docker, GitHub Actions CI, tests ≥ 80%) plus a full React 19 + Vite + Tailwind v4 + shadcn/ui frontend implementing the design.md retro-enterprise system (10-page sidebar SPA, responsive, WCAG AA) — and 9 spec docs (database/api/architecture/security/backend/testing/frontend/deployment/coding-standards) written spec-first so they govern the code instead of describing it after. One worker executes it end-to-end.

**Why this approach**: Your PRD defines the outcome and stack; design.md defines the UI system. Per your decisions: GORM AutoMigrate (no golang-migrate), TDD with ≥80% coverage, ONE plan at full fidelity to BOTH docs (incl. design.md's extra pages/export/KPIs — all user-authorized), 16 dependency-ordered waves so the DB exists before repos and the API exists before frontend pages.

**What it will NOT do**: No microservices/multi-tenancy/notifications (later phases); no frontend unit-test suite (PRD lists Go tests only; frontend verified by agent-run browser QA); no rate limiting; no work outside PRD §9 + design.md §15 layout.

**Effort**: 16 waves ≈ 60+ todos (each = implementation + tests + commit), incl. 9 spec-first doc todos. This is a multi-session plan; the worker splits if a todo exceeds a session.

**Risk**: Main risks are stack-version drift (handled by a T1.0 version-lock spike) and the size of the frontend surface (mitigated by shared DataTable/feedback components + browser QA per wave). Greenfield repo — no legacy constraints.

**Decisions you made**: AutoMigrate · TDD · one full plan · full fidelity to PRD+design.md · Recharts · dockerized-Postgres tests · bare `inventory` module path.
## Scope

Build Phase 1 (Foundation) of the Enterprise Inventory Management System as a
modular monolith, per `/home/vinkanika/Documents/CODING/golang/prd_phase1.md`
and `/home/vinkanika/Documents/CODING/golang/design.md`, in ONE plan executed
by a worker with zero further interview. Workspace is greenfield (only the two
specs exist; not a git repo; no code).

### IN scope (full fidelity to both docs - user decision 2026-08-05)

Backend (Go 1.24+, Gin, GORM + gorm.io/driver/postgres/pgx, PostgreSQL 17):
- Shared foundation: Viper config, Zap structured logging, GORM connection +
  AutoMigrate, unified response envelope, validator/v10 wrapper, typed errors,
  middleware (CORS, secure headers, request ID, JWT auth, RBAC role check).
- Auth: register, login, logout, refresh (rotating), change password, update
  profile. JWT access 15m + refresh 7d hashed in refresh_tokens. bcrypt cost 12.
- User module: profile CRUD, admin user management (list/search/paginate,
  role assignment, activate/deactivate).
- Category module: full CRUD + pagination + CSV export.
- Product module: CRUD + search/sort/filter/pagination + CSV export +
  low-stock threshold per product (default 10).
- Inventory module: stock-in, stock-out (writes inventory_transactions,
  updates inventory quantity atomically in a DB transaction), inventory list
  joined with product, history endpoint, low-stock indicator, CSV export.
- Activity Log module: writes on auth + mutation actions; paginated/filtered
  read endpoint (feeds design.md Activity Logs page).
- Dashboard module (PRD KPIs + design.md extensions): total products, total
  categories, inventory value, low stock summary, recent activities, pending
  restock, warehouse health, top-selling products, inventory movement series,
  category distribution.
- Reports module (lightweight, design.md Reports page): stock summary report +
  CSV export.
- Swagger (Swaggo + gin-swagger) served by the API.
- Seeds: roles (ADMIN, STAFF), default admin user (admin@inventory.local /
  Admin123! - documented, force-change recommended), sample categories/products
  only via a separate `seed:demo` target (opt-in).
- TDD throughout; repository tests against dockerized Postgres; service tests
  with mocked repos; handler + middleware tests via httptest. Coverage >= 80%
  enforced in CI.

Frontend (React 19 + TypeScript + Vite + Tailwind CSS v4 + shadcn/ui):
- Design system per design.md: retro-enterprise identity, status colors,
  radius tokens (buttons 8px, cards 12px), cards 24px padding + small shadow,
  toasts top-right 4s, max 200ms animations, WCAG AA, keyboard-first, focus
  states, breakpoints >=1280 / 768-1279 / <768.
- Layout: fixed sidebar (10 destinations: Dashboard, Products, Categories,
  Inventory, Transactions, Users, Reports, Activity Logs, Settings, Logout),
  header (breadcrumb, page title, Ctrl+K global search, notifications, quick
  create, theme toggle, profile menu); tablet collapsible, mobile drawer.
- Pages: Login, Register, Dashboard, Products, Categories, Inventory,
  Transactions, Users, Reports, Activity Logs, Settings, Profile.
- Module standards on every list page: search, filters, pagination, sorting,
  export, responsive table (column visibility, bulk actions, sticky header,
  row selection), empty state, loading skeleton, error state.
- Forms: label + helper text + inline validation (no modal alerts).
- Tech: TanStack Query, React Hook Form + Zod, React Router, Recharts
  (charts), lucide-react icons, shadcn/ui components.
- Frontend QA: agent-executed browser QA via webapp-testing (Playwright)
  per page wave; no frontend unit-test suite in Phase 1 (PRD lists Go tests).

Infra/DevOps:
- Docker multi-stage backend image (golang:1.24-alpine -> distroless or
  alpine runtime), frontend (node build -> nginx serve + proxy /api to api),
  docker-compose with postgres:17-alpine (healthcheck, named volume) + api +
  web; env via .env + compose env_file.
- Makefile: dev (air), build, test, test-cover, lint (golangci-lint), swagger,
  docker-up, docker-down, seed, seed-demo.
- GitHub Actions CI: gofmt check, golangci-lint, go vet, go test -cover with
  80% gate, frontend tsc + vite build.
- README (setup, architecture, ER diagram), .gitignore, git init + conventional
  commits per todo.
- 9 SPEC DOCS (user scope change 2026-08-05): database.md, api.md,
  architecture.md, coding-standards.md, security.md, backend.md, testing.md,
  frontend.md, deployment.md — all under `docs/`, written SPEC-FIRST (each in
  the wave before the code it governs). Priority: P0 database/api/architecture,
  P1 coding-standards/security/backend/testing, P2 frontend, P3 deployment.
  Deliverable placement: architecture+standards+testing (W1), database (W2),
  api+security (W3), backend (W4), deployment (W10), frontend (W11). README +
  ER (T10.4).

### OUT of scope (guardrails, not reductions)

- Microservices, multi-tenancy, email notifications, reporting/analytics
  beyond the defined reports, frontend unit-test suite (Vitest), realtime
  (websockets), rate limiting (not in PRD), localization/i18n, payment/orders,
  audit of auth token revocation list beyond refresh_tokens rotation.
- Later phases of the PRD (Phase 2+) are untouched.
- Do NOT create files outside the module layout in PRD #9 + design.md #15
  without a plan amendment.

### Locked conventions (Metis folds, 2026-08-05)

- RBAC matrix: ADMIN = all routes incl. user mgmt + reports + activity logs +
  product/category/inventory writes; STAFF = product/category/inventory
  read-write + dashboard + transactions read + own profile; anonymous =
  register/login only. Enforced by RoleRequired(ADMIN) on user/report/activity
  routes.
- Pagination defaults: page=1, per_page=20 (max 100), response meta
  {page, per_page, total, total_pages}; sort default created_at desc; filters
  are equality + range (min_price/max_price) + text LIKE for q.
- Low-stock semantics: per-product LowStockThreshold (default 10, env
  LOW_STOCK_THRESHOLD as global default); product is low-stock when
  inventory.quantity <= threshold AND not archived.
- CSV format: RFC 4180 (quoting + CRLF), UTF-8 with BOM, header row, filename
  `module_YYYYMMDDHHMM.csv`, Content-Disposition attachment; built by shared
  `internal/shared/export` util (single source of truth).
- Stack compatibility is verified by a spike todo (W1 T1.0) BEFORE code:
  pin exact versions (Go 1.24.x, Gin v1.10.x, GORM v1.30.x, pgx driver,
  React 19.x, Tailwind v4.x, shadcn/ui, Vite 7.x, TanStack Query v5, RHF v7,
  Zod v4, Recharts 3.x) and record the lockfile/go.sum as evidence; any
  incompatibility found is resolved at the spike, not during implementation.
- Metis "scope creep" findings (activity logs, reports, CSV export, extended
  KPIs) are NOT creep: user decision #4 (full fidelity to design.md) explicitly
  authorized all four. Recorded for the F4 scope-fidelity audit.

## Verification strategy

- TDD per todo: write failing test first (happy + failure), implement, green.
- Backend: `go test ./... -cover` against dockerized Postgres
  (`docker compose up -d db`; DATABASE_URL_TEST with a dedicated test schema
  truncated between tests); `golangci-lint run`; `gofmt -l` clean.
- Frontend: `npm run build` (tsc strict + vite build) clean; agent-executed
  browser QA per page wave (login flow, CRUD flows, empty/loading/error states,
  responsive at <768 / 768-1279 / >=1280, keyboard nav, toast behavior).
- Evidence: every todo records the exact command(s) run + artifact paths
  (test names, coverage % file, screenshots/logs under .omo/evidence/).
- Final verification wave (F1-F4, parallel, ALL must pass) after all todos.

## Execution strategy

- ONE worker session (`/start-work` or equivalent) executes waves in order.
- 14 waves, dependency-ordered (not priority-ordered): the dockerized DB
  arrives in W1 because repository tests need it from W2 onward; P0 modules
  (auth/user/category/product/inventory) ship before P1 (logs/dashboard/
  reports/export) before P2/P3 (frontend + infra polish).
- Each wave = 3-6 todos; each todo = ONE file set + tests + commit.
- After each wave: `go test ./...` green, lint clean, wave commit.
- Frontend waves run against the running API (docker compose up api) with
  seeded demo data for realistic charts/tables.
- Metis gap-analysis findings (bg_226a4863) folded into todos below.
- Sticky rule: a worker that cannot complete a todo in one session requests
  task splitting; five failed review cycles for the same todo -> stop + report.

## Todos

### W1 - Project init + shared foundation (C1)
Module path: `inventory`. Dir layout per PRD #9: `cmd/server`, `internal/{auth,user,product,category,inventory,dashboard,report,activitylog,shared}`, `docs`, `migrations`, `web`, `tests`. No design.md-dependent work yet.

- [x] **T1.0a - SPEC-FIRST: docs/architecture.md + docs/coding-standards.md**
References: PRD #8 (layering), #9 (folders), #10 (Go concepts), #17 (gofmt/lint DoD); user doc priority P0/P1. Files: `docs/architecture.md` (Clean Architecture, handler->service->repo interface, DI wiring, module boundaries per PRD #9, data flow diagram per PRD #8), `docs/coding-standards.md` (gofmt, golangci-lint v2 ruleset, conventional commits, 250-LOC ceiling, no `any`/panic, TDD rule, error-wrapping rule).
Acceptance: both docs exist and are consistent with PRD #8/#9/#10; coding-standards explicitly adopted by later todos (referenced in their Acceptance).
QA happy: architecture diagram matches final module list; standards doc has a lint config section matching T10.3. QA failure: any later wave creates a module outside PRD #9 -> F1 flags — evidence: doc cross-check.
Commit: `docs(architecture+standards): spec-first foundation docs`.

- [x] **T1.0b - SPEC-FIRST: docs/testing.md**
References: PRD #14 (repo/service/handler/middleware tests, 80%+); decision D2 (TDD); Locked conventions (dockerized-Postgres repo tests). Files: `docs/testing.md` (test pyramid, TDD workflow, coverage gate command, DB test strategy with truncation, testify usage, evidence conventions).
Acceptance: doc defines the exact `make test-cover` gate used by T10.3 and every todo's QA.
QA happy: gate command in doc runs locally on W1 green. QA failure: doc gate mismatches CI gate -> F2 flags — evidence.
Commit: `docs(testing): tdd + coverage strategy`.

- [x] **T1.0 - Stack version-lock spike (Metis fold: unvalidated assumptions)**
References: Locked conventions (stack compatibility). Action: with no code yet, resolve and PIN exact versions: Go 1.24.x; Gin v1.10.x; GORM v1.30.x + gorm.io/driver/postgres (pgx); viper v1.20.x; zap v1.27.x; validator v10.x; golang-jwt/jwt/v5 v5.2.x; google/uuid v1.6.x; swaggo; testify v1.10.x; golangci-lint v2.x; React 19.x; Tailwind v4.x; Vite 7.x; shadcn/ui; TanStack Query v5; RHF v7.x; Zod v4.x; Recharts 3.x; lucide-react. Verify pairwise compatibility (esp. GORM+pgx driver, React 19+Tailwind v4+shadcn) via official docs (context7/library docs). Record chosen versions in `docs/stack-versions.md`.
Acceptance: `docs/stack-versions.md` lists every dep + version + a one-line compat note; no Go/JS code written in this todo.
QA happy: every pinned version exists on its registry (npm view / go list -m). QA failure: an incompatible pair found -> resolve now (downgrade/upgrade) and record — evidence: the compat note + go.sum/package-lock from W2+.
Commit: `docs(stack): pin and verify dependency versions`.

- [x] **T1.1 - git init, go.mod, Makefile skeleton, .gitignore**
References: PRD #9; full-workflow commit strategy. Files: `go.mod` (module `inventory`, go 1.24), `.gitignore`, `Makefile` (targets: dev/test/build/lint/run later filled).
Acceptance: `git init` succeeds; `go mod init inventory` produces valid go.mod; `make` lists targets without error.
QA happy: `go build ./...` succeeds on empty main stub. QA failure: commit with unformatted/newlines fails via committed `make lint` gate — evidence: `git log --oneline` shows only structured commits.
Commit: `chore(init): scaffold module, Makefile, .gitignore`.

- [x] **T1.2 - shared/config (Viper)**
References: PRD #9 `internal/shared/config`; PRD #6 (environment configuration). Files: `internal/shared/config/config.go` (+ `config_test.go`). Env: APP_ENV, PORT, DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE, JWT_SECRET, JWT_ACCESS_TTL, JWT_REFRESH_TTL, BCRYPT_COST(default 12), LOW_STOCK_THRESHOLD(default 10), CORS_ORIGINS, LOG_LEVEL.
Acceptance: `Load()` reads .env/env defaults; `config_test.go` asserts defaults and override behavior.
QA happy: `go test ./internal/shared/config -run TestLoad -v` passes. QA failure: missing required var (DB_USER) returns typed error — evidence: failing test + error path.
Commit: `feat(config): viper env config with required fields`.

- [x] **T1.3 - shared/logger (Zap)**
References: PRD #6 structured logging; #9 logger. Files: `internal/shared/logger/logger.go` (+ test). Production JSON logger + dev console; LOG_LEVEL wired.
Acceptance: `logger.New(cfg)` returns *zap.Logger; test asserts level + JSON fields.
QA happy: log line emitted to stdout in test. QA failure: invalid level falls back to info — evidence test.
Commit: `feat(logger): zap structured logger`.

- [x] **T1.4 - shared/database (GORM + pgx driver, AutoMigrate hook)**
References: PRD #8 layering; #9 database; GORM AutoMigrate (decision D1). Files: `internal/shared/database/database.go` (+ test), `internal/shared/database/migrate.go` with base models registry.
Acceptance: `Connect(cfg)` opens GORM with gorm.io/driver/postgres (pgx); `AutoMigrate(models...)` creates tables; connection retry with backoff.
QA happy: `go test` connects to dockerized db, migrates, drops — evidence migration applied. QA failure: wrong DB name returns error — evidence assert len(err).
Commit: `feat(database): gorm connection + automigrate`.

- [x] **T1.5 - shared/response + shared/validator + shared/errors**
References: PRD #9 response/validator; #6 input validation. Files: `internal/shared/response/{response.go,response_test.go}`, `internal/shared/validator/validator.go`, `internal/shared/errors/errors.go`.
Acceptance: envelope `{success,message,data,pagination}`; validator wraps validator/v10; typed errors (NotFound/Validation/Unauthorized/Forbidden/Conflict/Internal).
QA happy: handler test asserts envelope JSON shape. QA failure: validation error maps to 400 envelope — evidence test.
Commit: `feat(shared): response envelope, validator, typed errors`.

- [x] **T1.6 - shared/middleware (CORS, SecureHeaders, RequestID)**
References: PRD #11 (CORS, secure headers); #9 middleware. Files: `internal/shared/middleware/{cors,security,requestid}.go` (+ tests).
Acceptance: CORS honors CORS_ORIGINS; security sets X-Content-Type-Options, X-Frame-Options, CSP; RequestID adds header + injects into context/logger.
QA happy: gin test asserts headers present. QA failure: disallowed origin rejected — evidence test.
Commit: `feat(middleware): cors, secure headers, request id`.

- [x] **T1.7 - shared/export CSV util (Metis fold: CSV format)**
References: Locked conventions (CSV format); design.md #141 (Export). Files: `internal/shared/export/{export.go,export_test.go}`.
Acceptance: `WriteCSV(w, headers, rows)` produces RFC 4180 output (quotes + CRLF), UTF-8 BOM, `Content-Disposition` helper; enforces Locked conventions CSV format. All module export todos below MUST use this single util.
QA happy: golden-file test asserts exact CSV bytes incl. a field containing comma/quote/newline. QA failure: missing BOM or unquoted special char -> test fails — evidence golden diff.
Commit: `feat(export): shared rfc4180 csv writer`.

- [x] **T1.8 - cmd/server/main.go skeleton (DI wiring + router + healthcheck)**
References: PRD #8; #9 cmd/server. Files: `cmd/server/main.go`, `internal/shared/router/router.go` (+ test).
Acceptance: app boots on PORT; `GET /healthz` returns 200 `{status:"ok"}`; DB connection verified at boot.
QA happy: `go run ./cmd/server` + `curl localhost:PORT/healthz` returns ok (evidence output). QA failure: DB down at boot retries and eventually errors (evidence log).
Commit: `feat(server): bootstrap router with healthcheck`.
End W1: `go test ./...` green, lint clean, commit `feat(foundation): shared packages wired`.

### W2 - DB schema, seeds (C8 partial)
All models defined here via AutoMigrate (D1). Tables: users, roles, products, categories, inventory, inventory_transactions, refresh_tokens, activity_logs (PRD #7).

- [x] **T2.0 - SPEC-FIRST: docs/database.md (ER + schema)**
References: PRD #7 (8 tables), #15 (ER diagram); user doc priority P0. Files: `docs/database.md` (mermaid ER diagram, per-table columns/types/constraints/FKs/indexes, low-stock semantics, soft-delete convention).
Acceptance: doc specifies every column used by W2 model todos (T2.1-T2.3) exactly; ER diagram renders.
QA happy: mermaid renders in markdown; models in T2.1-2.3 match doc columns. QA failure: a model field missing from database.md -> F1 flags — evidence diff.
Commit: `docs(database): er diagram + schema spec`.

- [x] **T2.1 - Role + User models**
Files: `internal/auth/model.go` (User{ID uuid, Name, Email unique, PasswordHash, RoleID, IsActive, timestamps}), `internal/shared/database/models.go` registers Role{ID, Name: ADMIN/STAFF}. No GORM auto-migration drift.
Acceptance: AutoMigrate creates `users` + `roles`; unique index on users.email; FK roles.
QA happy: migrate.test asserts table exists. QA failure: duplicate email at DB violated — evidence test.
Commit: `feat(auth): user + role models`.

- [x] **T2.2 - Product + Category + Inventory models**
Files: `internal/category/model.go`, `internal/product/model.go` (SKU unique, Price decimal, CategoryID FK, LowStockThreshold default 10, IsArchived), `internal/inventory/model.go` (Inventory{ProductID unique, Quantity}, InventoryTransaction{Type in/out, Qty, UnitCost, Note, UserID}).
Acceptance: AutoMigrate creates products, categories, inventory, inventory_transactions; unique index inventory.product_id; FK product.category_id.
QA happy: tables exist. QA failure: SKU duplicate violated — evidence test.
Commit: `feat(models): product, category, inventory`.

- [x] **T2.3 - RefreshToken + ActivityLog models**
Files: `internal/auth/refresh_tokens` model (TokenHash unique, ExpiresAt, RevokedAt, UserID FK), `internal/activitylog/model.go` (Action, EntityType, EntityID, Details JSON, UserID, IP, CreatedAt).
Acceptance: AutoMigrate creates refresh_tokens, activity_logs; unique token_hash; FK user_id.
QA happy: tables exist. QA failure: FKs enforced — evidence test.
Commit: `feat(models): refresh_tokens, activity_logs`.

- [x] **T2.4 - Seeds (roles + default admin)**
Files: `cmd/seed/seed.go`, `cmd/seed/demo.go` (opt-in), Makefile target `seed`/`seed-demo`. Admin: admin@inventory.local / Admin123! (bcrypt cost 12). Demo: 5 categories, 20 products, 30 transactions for realistic UI.
Acceptance: `make seed` inserts roles + admin idempotently; `make seed-demo` adds demo data.
QA happy: run seed twice = no duplicate roles — evidence row count. QA failure: missing env fails clearly — evidence error.
Commit: `feat(seed): roles, admin user, opt-in demo data`.
End W2: healthcheck + seed runnable against dockerized db.

### W3 - Auth module (C2, P0)
- [x] **T3.0 - SPEC-FIRST: docs/api.md + docs/security.md**
References: PRD #12 (API modules), #11 (security), #15 (API doc); user doc priority P0/P1; Locked conventions (RBAC matrix, JWT TTLs). Files: `docs/api.md` (full route table: method, path, auth, role, request DTO, response envelope + pagination meta, error codes per route), `docs/security.md` (JWT access/refresh design, rotation, bcrypt cost 12, secure headers, CORS allowlist, RBAC matrix, injection-safety rules).
Acceptance: api.md covers every route the W3-W9 handler todos implement; security.md matches Locked conventions.
QA happy: swagger (T10.1) later matches api.md — evidence cross-check. QA failure: a handler route absent from api.md -> F1 flags — evidence.
Commit: `docs(api+security): route contract + threat design`.

- [x] **T3.1 - Auth service: register, login, refresh, logout, change-password, update-profile**
References: PRD #11 (JWT, refresh, bcrypt); #10 (deps, error wrapping); D2 (access 15m, refresh 7d rotating, hashed). Files: `internal/auth/service.go`, `internal/auth/service_test.go`.
Acceptance: Register bcrypt-hashes + creates user + activity log; Login verifies; Refresh rotates token (revoke old, issue new); Logout revokes; ChangePassword verifies old; UpdateProfile updates name/email. All flows on service level with mocked repo (TDD first).
QA happy: testify mock asserts calls + returned tokens. QA failure: wrong password -> ErrUnauthorized; duplicate email -> ErrConflict — evidence test.
Commit: `feat(auth): auth service with token rotation`.

- [x] **T3.2 - Auth JWT token manager**
Files: `internal/auth/token.go`, `internal/auth/token_test.go`. golang-jwt/jwt/v5 HS256; access claims (sub, role, exp 15m); refresh = 40-byte random, sha256-hashed stored.
Acceptance: access token signs/verifies; refresh hash round-trips; expiry enforced.
QA happy: issued token verifies with sub. QA failure: tampered/expired token rejected — evidence test.
Commit: `feat(auth): jwt access + refresh hash`.

- [x] **T3.3 - Auth handlers + routes + middleware**
Files: `internal/auth/handler.go`, `internal/auth/router.go`, `internal/auth/handler_test.go`, `internal/shared/middleware/auth.go` (+ test). Routes: POST /api/v1/auth/register|login|logout|refresh, POST /auth/change-password, PUT /auth/profile, GET /auth/me.
Acceptance: handlers validate DTO (validator/v10), call service, return envelope; middleware extracts token -> sets userID+role in context; RBAC RoleRequired(ADMIN) helper.
QA happy: httptest register->login->me roundtrip 200. QA failure: missing/invalid token on protected route -> 401 envelope; staff role blocked from admin route -> 403 — evidence test.
Commit: `feat(auth): handlers, jwt middleware, rbac`.
End W3: full auth e2e via curl against dockerized API; W3 tests green.

### W4 - User module (C3, P0)
- [x] **T4.0 - SPEC-FIRST: docs/backend.md (Go implementation standards)**
References: PRD #10 (packages, interfaces, DI, error wrapping, transactions, generics), #9; user doc priority P1. Files: `docs/backend.md` (package conventions, repository interface pattern with concrete example, service error wrapping, transaction usage for inventory, DTO/validation pattern, response envelope usage, logger usage, ctx propagation).
Acceptance: doc is authoritative for all later backend todos; W7 transaction design matches it.
QA happy: W7 stock-in/out code follows backend.md transaction pattern — evidence code review. QA failure: module bypasses repo interface -> F2 flags.
Commit: `docs(backend): go implementation standards`.

- [x] **T4.1 - User service + repo (admin mgmt)**
Files: `internal/user/{model,repository,service}.go` (+ tests). List/search/paginate, get by id, update profile fields, assign role, activate/deactivate. Reuses auth models.
Acceptance: repo paginates + filters by name/email/role; service guards (cannot deactivate self).
QA happy: mock repo returns users page. QA failure: last admin self-deactivation -> ErrConflict — evidence test.
Commit: `feat(user): admin user management service`.
- [x] **T4.2 - User handlers + routes**
Files: `internal/user/handler.go`, `internal/user/router.go` (+ handler_test.go). Routes: GET /users (admin, paginated/filtered), GET /users/:id, PUT /users/:id, DELETE /users/:id (deactivate), PUT /users/:id/role.
Acceptance: admin-only via RBAC; envelope pagination meta.
QA happy: admin list users 200. QA failure: non-admin -> 403 — evidence test.
Commit: `feat(user): user admin routes`.
End W4: user admin crud green.

### W5 - Category module (C5, P0)
- [x] **T5.1 - Category service + repo (CRUD + pagination)**
Files: `internal/category/{model,repository,service}.go` (+ tests). CRUD, list with search + sort (name/created_at) + pagination.
Acceptance: repo Create/Get/Update/Delete + paginated list; service deletes only when no products reference (else ErrConflict).
QA happy: mock repo CRUD. QA failure: delete category in use -> ErrConflict — evidence test.
Commit: `feat(category): category service + repo`.
- [x] **T5.2 - Category handlers + routes**
Files: `internal/category/handler.go`, `category/router.go` (+ tests). Routes: POST/GET /categories, GET/PUT/DELETE /categories/:id, GET /categories/export (CSV).
Acceptance: CRUD 200; export returns text/csv with Content-Disposition.
QA happy: create->get->list roundtrip. QA failure: invalid body -> 400; export has header row — evidence test.
Commit: `feat(category): handlers, routes, csv export`.
End W5: category green.

### W6 - Product module (C4, P0)
- [x] **T6.1 - Product service + repo (CRUD + search/sort/filter/paginate)**
Files: `internal/product/{model,repository,service}.go` (+ tests). Filters: q (name/sku), category_id, min_price, max_price, low_stock (qty<=threshold), is_archived; sort: name/price/created_at/sku; pagination page/per_page.
Acceptance: repo builds dynamic WHERE + ORDER + LIMIT/OFFSET safely (no injection); service enforces unique SKU.
QA happy: mock returns filtered page. QA failure: SQL injection string for sort col rejected -> ErrValidation — evidence test.
Commit: `feat(product): product service + repo`.
- [x] **T6.2 - Product handlers + routes + CSV export**
Files: `internal/product/handler.go`, `router.go`, `handler_test.go`. Routes: POST/GET /products, GET/PUT/DELETE /products/:id, GET /products/export.
Acceptance: CRUD + query params + CSV export; category name included in list payload.
QA happy: create->list with filter->get roundtrip. QA failure: duplicate SKU -> 409 — evidence test.
Commit: `feat(product): handlers, routes, csv export`.
End W6: product green.

### W7 - Inventory module (C6, P0)
- [x] **T7.1 - Inventory service + repo (stock-in/out, atomic transaction)**
References: PRD #10 (transactions); #8 layering. Files: `internal/inventory/{model,repository,service}.go` (+ tests). StockIn/StockOut in a DB transaction: insert inventory_transactions + upsert inventory.Quantity atomically; reject stock-out > available.
Acceptance: concurrent-safe quantity mutation via transaction; stock-out overdraw -> ErrConflict; creates history row.
QA happy: mock/DB test stock-in then stock-out nets correct qty — evidence. QA failure: overdraw rolls back — evidence no partial row.
Commit: `feat(inventory): stock in/out with db transaction`.
- [x] **T7.2 - Inventory handlers + routes + low-stock + history + CSV export**
Files: `internal/inventory/handler.go`, `router.go`, `handler_test.go`. Routes: POST /inventory/stock-in|stock-out, GET /inventory (joined list, filter low-stock), GET /inventory/transactions (paginated, filter product/type/date), GET /inventory/export.
Acceptance: endpoints return envelope; low-stock filter works; history paginated.
QA happy: stock-in -> GET /inventory shows qty; overdraw -> 409. QA failure: invalid type -> 400 — evidence test.
Commit: `feat(inventory): routes, history, low-stock, export`.
End W7: inventory green. P0 backend core complete.

### W8 - Activity Log module (C10, P1)
- [ ] **T8.1 - ActivityLog service + repo (write + paginated/filtered read)**
References: design.md sidebar Activity Logs; Locked conventions. Files: `internal/activitylog/{model,repository,service}.go` (+ tests). Write on every auth + mutation (register, login, product/category/inventory create/update/delete, stock in/out); read endpoint filters by entity_type, entity_id, action, user_id, date range, paginated; ordered created_at desc.
Acceptance: repo inserts + filters + paginates; service logs user_id+ip+details json. Failure-safe: a logging error NEVER fails the business operation (log + continue).
QA happy: create product -> one activity_log row present. QA failure: logger down -> business op still succeeds — evidence test asserts txn committed despite log error.
Commit: `feat(activitylog): write on actions + filtered read`.

- [ ] **T8.2 - ActivityLog handlers + routes**
Files: `internal/activitylog/handler.go`, `router.go` (+ tests). Routes: GET /activity-logs (admin, RBAC).
Acceptance: returns envelope + pagination; filters apply.
QA happy: list after seed-demo shows transactions. QA failure: non-admin -> 403 — evidence test.
Commit: `feat(activitylog): admin routes`.
End W8: activity log green.

### W9 - Dashboard + Reports (C7 + C11, P1)
- [ ] **T9.1 - Dashboard service + repo (aggregates: PRD KPIs + extended)**
References: PRD #125-129; design.md #96-128; Locked conventions. Files: `internal/dashboard/{model,repository,service}.go` (+ tests). Aggregates computed on read (no cache — decision): total products (non-archived), total categories, inventory value (sum qty*unit cost of last stock-in or product.price), low stock summary (count + items), recent activities (from activity_logs), pending restock (low-stock count), warehouse health (healthy/low/critical counts), top selling (sum OUT qty per product, top N), inventory movement (STOCK_IN/STOCK_OUT/net/ending per day for N days default 30), category distribution (product count per category).
Acceptance: repo SQL aggregates correct on seeded data; service composes summary + chart payloads with exact shapes {labels:[],datasets:[]}.
QA happy: mock/DB test computes known totals from fixed seeds (assert exact numbers). QA failure: wrong aggregate math -> test fails — evidence expected-vs-actual.
Commit: `feat(dashboard): aggregate service + repo`.

- [ ] **T9.2 - Dashboard handlers + routes**
Files: `internal/dashboard/handler.go`, `router.go` (+ tests). Routes: GET /dashboard/summary, GET /dashboard/inventory-movement?days=, GET /dashboard/category-distribution, GET /dashboard/top-selling?limit=.
Acceptance: handlers validate params, return chart payload shapes for Recharts.
QA happy: summary roundtrip 200 with all KPIs. QA failure: invalid `days` -> 400 — evidence test.
Commit: `feat(dashboard): routes`.

- [ ] **T9.3 - Reports module (lightweight) + CSV export (C12)**
References: design.md Reports + Export; Locked conventions CSV. Files: `internal/report/{model,repository,service,handler,router}.go` (+ tests). Stock summary report (per-category totals, low-stock list) + CSV export via shared util.
Acceptance: GET /reports/stock-summary returns envelope; GET /reports/export returns CSV attachment.
QA happy: summary numbers match dashboard low-stock + category counts. QA failure: export uses shared util (golden CSV test) — evidence.
Commit: `feat(reports): stock summary + csv export`.
End W9: dashboard + reports green. P1 backend complete.

### W10 - Docs + Infra (C8, P3)
- [ ] **T10.0 - SPEC-FIRST: docs/deployment.md**
References: PRD Milestone 11-12; user doc priority P3. Files: `docs/deployment.md` (docker compose topology, env vars, volume strategy, Makefile targets, CI pipeline steps, local vs CI differences, secrets note for JWT_SECRET).
Acceptance: doc matches T10.2/T10.3 implementation exactly; a fresh reader can `make docker-up` from it.
QA happy: deployment.md steps reproduce a working stack — evidence docker-up. QA failure: doc omits an env var compose requires -> F1 flags.
Commit: `docs(deployment): docker + ci runbook`.

- [ ] **T10.1 - Swagger (Swaggo) + API docs**
References: PRD #15 (API doc), #17 DoD; Milestone 10/11. Files: swaggo annotations on all handlers; `docs/swagger/doc.go`; `cmd/server` registers gin-swagger at /swagger/*any; Makefile `swagger`.
Acceptance: `swag init` (in `swag init -g cmd/server/main.go -o docs/swagger`) builds docs without error; `GET /swagger/index.html` serves UI; every route documented.
QA happy: hit /swagger/index.html -> 200 HTML. QA failure: a handler missing annotations -> swag init fails or route absent from docs — evidence swagger json includes every route.
Commit: `docs(swagger): annotate and serve openapi`.

- [ ] **T10.2 - Docker backend + compose + Makefile docker targets**
References: PRD #3 (dockerized app), Milestone 11. Files: `Dockerfile` (multi-stage golang:1.24-alpine build -> alpine runtime), `docker-compose.yml` (postgres:17-alpine healthcheck + named volume, api depends_on healthy, web), `.env.example`, Makefile docker-up/down/logs.
Acceptance: `docker compose up -d db api` -> api healthy + /healthz 200 from container; DB persists across `docker compose down` (volume).
QA happy: full compose up, curl api:PORT/healthz 200. QA failure: api starts before db healthy -> retry/healthcheck handles — evidence compose logs show depends_on gating.
Commit: `feat(docker): multi-stage api image + compose`.

- [ ] **T10.3 - GitHub Actions CI**
References: PRD Milestone 12; PRD #17 (gofmt + lint pass). Files: `.github/workflows/ci.yml`. Jobs: gofmt check, go vet, golangci-lint, go test -cover with 80% gate (`-covermode=atomic -coverprofile && awk` block), rebuild + tsc strict + vite build.
Acceptance: workflow file valid YAML; runs on push/PR; coverage gate computed correctly.
QA happy: `actionlint` or YAML parse ok; (local) `go test -cover` shows >=80%. QA failure: coverage <80% -> CI step fails — evidence: gate line in workflow.
Commit: `ci(github-actions): lint, vet, test-coverage gate, builds`.

- [ ] **T10.4 - README + ER diagram + install guide**
References: PRD #15 (README, ER diagram, install guide). Files: `docs/er.md` (mermaid), `README.md` (setup, architecture #8, folder #9, seed/admin creds, security note to change Admin123!).
Acceptance: README documents prerequisites, env, `make` targets, docker run, seed credentials with a clear "change the default admin password" warning.
QA happy: README links resolve (er.md exists). QA failure: missing section -> markdown lint. 
Commit: `docs(readme): setup, architecture, er diagram`.
End W10: infra green. `make docker-up` brings the whole system up.

### W11 - Frontend foundation + design system (C9, P2/P3)
- [ ] **T11.0 - SPEC-FIRST: docs/frontend.md**
References: design.md #1-#16 (full spec); user doc priority P2. Files: `docs/frontend.md` (page inventory, component tree per design.md #15, design tokens, module-standard DataTable spec, API-consumption map from api.md, routing table, breakpoints, a11y rules).
Acceptance: every W11-W15 frontend todo implements something specified in frontend.md; token/component specs match design.md exactly.
QA happy: W14 pages match frontend.md page specs — evidence browser QA. QA failure: a page missing empty/loading/error states -> F1 flags.
Commit: `docs(frontend): design system + page specs`.

- [ ] **T11.1 - Vite scaffold + Tailwind v4 + shadcn/ui wired to Locked conventions**
References: PRD #13 stack; design.md #1,#15; Locked conventions stack compat. Files: `web/` (Vite React-TS), `web/src/index.css` (Tailwind v4 @theme), `web/components.json`, `web/src/lib/utils.ts`, base shadcn components (button, card, input, select, table, dialog, dropdown-menu, toast/sonner).
Acceptance: `npm run build` (tsc strict + vite) succeeds; BaseProvider mounts; Tailwind v4 theme variables set.
QA happy: dev server + `npm run build` clean; a shadcn Button renders. QA failure: Tailwind classes not compiled -> build error — evidence.
Commit: `feat(web): vite + tailwind v4 + shadcn base`.

- [ ] **T11.2 - Design tokens + retro-enterprise theme (design.md tokens)**
References: design.md #8 (btns 8px, icons 20px), #9 (cards 12px/24px/small shadow), #10 status colors, #12 a11y, #13 breakpoints, #14 200ms max. Files: `web/src/lib/theme.css`/index.css tokens: status colors (healthy green, info blue, warning amber, critical red, archived gray), radius tokens, spacing, shadows; 200ms transition cap; WCAG AA contrast palette.
Acceptance: token utilities exist and compile; contrast of status+text pairs meets AA (assert via style).
QA happy: design tokens used by Button/Card. QA failure: a transition >200ms or low-contrast pair -> QA flags — evidence screenshot/audit.
Commit: `feat(web): design tokens + theme`.

- [ ] **T11.3 - API client + TanStack Query + Router + auth context**
References: PRD #13 (TanStack Query, React Router); scope frontend. Files: `web/src/lib/api.ts` (fetch wrapper with base URL + token injection + 401 refresh), `web/src/lib/queryClient.ts`, `web/src/router.tsx`, `web/src/auth/store.tsx` (JWT in memory + persisted refresh), protected-route guards + role guards.
Acceptance: api client attaches Bearer, refreshes on 401 once; Router guards redirect unauthenticated to /login; role guard blocks staff from /users,/reports,/activity-logs.
QA happy: login -> guard allows dashboard, blocks staff page. QA failure: expired access -> auto-refresh -> retry succeeds (browser QA) — evidence screenshots.
Commit: `feat(web): api client, query, router, auth`.
End W11: blank-but-themed SPA with auth plumbing.

### W12 - Frontend layout + shared components
- [ ] **T12.1 - AppShell (sidebar 10 items + header) responsive**
References: design.md #2-4 (sidebar layout, 10 destinations), #5 (quick actions), #12, #13 responsive. Files: `web/src/components/layout/{AppShell,Sidebar,Header,Breadcrumb,UserMenu}.tsx`. Sidebar items exactly: Dashboard, Products, Categories, Inventory, Transactions, Users, Reports, Activity Logs, Settings, Logout. Fixed desktop, collapsible tablet (<1280), drawer mobile (<768).
Acceptance: all 10 nav destinations present; active-route highlight; logout triggers auth logout; responsive behavior verified at 3 breakpoints; keyboard-first (focus + esc closes drawer).
QA happy: browser QA at 1440/1024/375 shows correct sidebar states. QA failure: at 375 a nav item hidden with no drawer toggle -> QA flags — evidence screenshots.
Commit: `feat(web): app shell + responsive sidebar`.

- [ ] **T12.2 - DataTable + module-standard table features**
References: design.md #6 (search, filters, pagination, sorting, export, responsive table, column visibility, bulk actions, sticky header, row selection, empty/loading/error states). Files: `web/src/components/tables/DataTable.tsx` (+ subcomponents), hook-driven.
Acceptance: DataTable supports sticky header, column visibility toggle, bulk row selection, empty/loading( skeleton)/error states, toolbar slots for search/filters/sort/pagination/export-button.
QA happy: table renders 100 rows with sticky header scroll + row selection count. QA failure: empty data -> EmptyState + refetch; error -> ErrorState + retry — evidence screenshots.
Commit: `feat(web): datatable with module standards`.

- [ ] **T12.3 - Shared feedback + common components (toast, empty, skeleton, error, form field)**
References: design.md #7 (forms label+helper+inline validation, no modal alerts), #11 (toasts top-right 4s), #6 (empty/error/skeleton). Files: `web/src/components/feedback/*`, `web/src/components/common/*`, `web/src/components/forms/FormField.tsx`.
Acceptance: toast top-right auto-dismiss 4s; FormField renders label+helper+inline error; empty/error/skeleton components reusable.
QA happy: trigger toast -> auto-dismisses at ~4s. QA failure: validation error rendered inline (not modal alert) — evidence screenshots.
Commit: `feat(web): feedback + common components`.
End W12: layout + reusable components ready.

### W13 - Frontend pages: Login, Register, Profile, Settings
- [ ] **T13.1 - Login + Register pages (design.md #1 retro, RHF+Zod)**
Files: `web/src/pages/Login.tsx`, `Register.tsx` (+ forms). Login: email+password, inline validation, error toast, redirect. Register: name/email/password/confirm, confirms via auth API.
Acceptance: valid submit calls auth, stores token, redirects; invalid shows inline + toast; loading state disabled button.
QA happy: register -> login -> lands dashboard (screenshot). QA failure: wrong password -> inline error, no redirect — evidence.
Commit: `feat(web): login + register`.
- [ ] **T13.2 - Profile + Settings pages**
Files: `web/src/pages/Profile.tsx`, `Settings.tsx`. Profile: view/update name+email, change-password flow (old/new/confirm). Settings: theme toggle, display prefs (persisted localStorage).
Acceptance: update profile reflects in /auth/me; change-password enforces current; settings persist.
QA happy: edit profile -> re-fetch shows new name. QA failure: wrong current password -> inline error — evidence.
Commit: `feat(web): profile + settings`.
End W13: auth/personal pages green.

### W14 - Frontend pages: Dashboard, Products, Categories
- [ ] **T14.1 - Dashboard page (KPI cards + charts + widgets)**
References: design.md #5 (6 KPI cards w/ icon+trend, inventory movement + category distribution charts, recent activity + quick actions + low stock + top selling widgets). Files: `web/src/pages/Dashboard.tsx`, `web/src/components/dashboard/*`.
Acceptance: 6 KPI cards show metric + icon; InventoryMovement + CategoryDistribution charts (Recharts) render from API payloads; widgets (recent activity, quick actions, low-stock summary, top-selling) populate.
QA happy: with seed-demo, charts + KPIs render non-empty (screenshot). QA failure: empty DB -> cards show 0 + EmptyState widgets, no crash — evidence.
Commit: `feat(web): dashboard with charts + widgets`.
- [ ] **T14.2 - Products page (DataTable + CRUD modal + filters)**
Files: `web/src/pages/Products.tsx`, `web/src/components/forms/ProductForm.tsx`. Search q, category filter, price range, low-stock filter, sort, pagination, column visibility, bulk actions, export CSV, create/edit/delete via dialog.
Acceptance: list + filters hit API query params; CRUD roundtrips; export downloads CSV; empty/loading/error states.
QA happy: create product -> appears in filtered list (screenshot). QA failure: delete -> confirm -> removed; duplicate SKU -> toast error — evidence.
Commit: `feat(web): products page`.
- [ ] **T14.3 - Categories page**
Files: `web/src/pages/Categories.tsx`, `web/src/components/forms/CategoryForm.tsx`. CRUD + export + pagination/search.
Acceptance: CRUD roundtrip; delete-in-use shows conflict error toast.
QA happy: add category -> list updates. QA failure: delete referenced category -> 409 toast — evidence.
Commit: `feat(web): categories page`.
End W14: dashboard + product + category pages green.

### W15 - Frontend pages: Inventory, Transactions, Users, Reports, Activity Logs
- [ ] **T15.1 - Inventory page (stock-in/stock-out + low-stock)**
Files: `web/src/pages/Inventory.tsx`, `web/src/forms/StockMovementForm.tsx`. List joined product + qty; stock-in/stock-out dialog (type+sku/qty+note); low-stock filter + badge; export.
Acceptance: stock movement roundtrips and list qty updates; overdraw -> 409 toast.
QA happy: stock-out -> qty decreases; low-stock items show red badge. QA failure: overdraw rejected with toast — evidence.
Commit: `feat(web): inventory page`.
- [ ] **T15.2 - Transactions page (inventory history)**
Files: `web/src/pages/Transactions.tsx`. DataTable from /inventory/transactions: type filter, product filter, date range, pagination, export.
Acceptance: history renders from endpoint; filters apply; export downloads.
QA happy: after movements, transactions listed newest-first. QA failure: empty -> EmptyState — evidence.
Commit: `feat(web): transactions page`.
- [ ] **T15.3 - Users page (admin)**
Files: `web/src/pages/Users.tsx`. List/search/paginate users; assign role; activate/deactivate; self-protection disable.
Acceptance: admin CRUD via /users; role change reflected; cannot deactivate self.
QA happy: admin changes staff role -> reflects. QA failure: staff route guard blocks — evidence.
Commit: `feat(web): users page`.
- [ ] **T15.4 - Reports + Activity Logs pages**
Files: `web/src/pages/Reports.tsx`, `web/src/pages/ActivityLogs.tsx`. Reports: stock summary + export button. ActivityLogs: filtered table (entity/action/date), paginated.
Acceptance: both pages render from backend; guards admin.
QA happy: report numbers match dashboard; activity list shows recent — evidence. QA failure: filters empty -> EmptyState.
Commit: `feat(web): reports + activity logs pages`.
End W15: all pages green. Frontend scope complete.

### W16 - Cross-cutting + harden (P0-P3 final QA)
- [ ] **T16.1 - ComposableSeed demo data parity check**
Files: `cmd/seed/demo.go` alignment only. Verify frontend dashboard/products/inventory render correctly with seed-demo (no code change unless bug).
Acceptance: seed-demo end-to-end visual parity verified via browser.
QA happy/failure: screenshots at 3 breakpoints of Dashboard + Products + Inventory.
Commit: `test(web): demo-data parity verified`.
- [ ] **T16.2 - Accessibility + responsive audit pass (design.md #12/#13)**
Acceptance: WCAG AA via automated scanner (axe) + manual keyboard walk; no focus-loss; all breakpoints verified.
QA happy: axe 0 critical violations; tab-through works. QA failure: violation found -> fixed in this wave — evidence axe report.
Commit: `fix(web): a11y + responsive audit`.
- [ ] **T16.3 - Backend security hardening review (PRD #11)**
Acceptance: bcrypt cost 12, JWT expires, refresh rotation, secure headers, CORS allowlist, SQL injection safe (no string concat in repo WHERE/SORT), input validation everywhere.
QA happy: security checklist proxied; injection attempts return 400 (test). QA failure: any raw concat found in repo -> refactor — evidence golangci + tests.
Commit: `fix(sec): hardening audit`.
- [ ] **T16.4 - Performance sanity (NFR #6 <300ms local)**
Acceptance: representative endpoints (/products list, /dashboard/summary, /inventory) respond <300ms locally against dockerized db.
QA happy: `curl -w %{time_total}` under threshold. QA failure: over -> add missing index or query opt, re-measure — evidence timings file.
Commit: `perf: verify <300ms on hot endpoints`.
End W16: cross-cutting complete.

## Final verification wave

Runs in PARALLEL after all todos; ALL must approve before completion is declared. Evidence under `.omo/evidence/final/`.

- [ ] **F1 - Plan compliance audit**: every todo done + acceptance met; no OUT-of-scope files created; dependency matrix consistent (backend before frontend waves; DB before repos). Evidence: todo checklist diff vs this file.
- [ ] **F2 - Code quality review**: gofmt clean, golangci-lint (v2) clean, `go vet` clean, tsc strict clean, no `any`/panics/string-concat SQL, 250-LOC ceiling respected, coverage >= 80% per package gate. Evidence: lint + cover outputs.
- [ ] **F3 - Real manual QA**: `make docker-up` fresh boot; register->login->CRUD each module via UI; stock in/out; export downloads; dashboard charts render; all pages at 3 breakpoints; axe a11y report. Evidence: screenshots + browser logs in `.omo/evidence/final/qa/`.
- [ ] **F4 - Scope fidelity**: exactly PRD Phase 1 + design.md full-fidelity scope (user decision #4); no missing PRD #17 DoD item; no scope creep beyond documented additions. Evidence: cross-check matrix PRD #5/#17 x implemented.

## Commit strategy

- Conventional Commits (`feat|fix|docs|chore|test|ci|perf(scope): summary`).
- One atomic commit per todo, after its tests pass; commit line is written IN each todo above.
- Wave boundary commits: after each Wn end-line, push a wave-summary commit if the worker commits separately.
- No commit with failing tests or lint; `make pre-commit` (gofmt + lint + vet + test) gate before each commit.
- git init in T1.1; first commit there; never commit .env, .codegraph, .omo/evidence (gitignored); .omo/plans tracked for the record.

## Success criteria

Mapped to PRD #3 + #17 (Definition of Done) + design.md #16 + user decisions:

1. All CRUD (categories, products, users) + stock in/out work end-to-end. (PRD #17)
2. Authentication complete: register/login/logout/refresh/change-password/profile + JWT + RBAC. (PRD #3, #11)
3. Validation + secure headers + CORS + bcrypt + injection-safe queries. (PRD #6, #11)
4. Clean Architecture with dependency injection; handler->service->repo interface layering in every module. (PRD #3, #8)
5. PostgreSQL 17 + AutoMigrate schema for all 8 tables + seeds (roles, admin, opt-in demo). (PRD #3, #7)
6. Swagger generated + served. (PRD #3, #15)
7. Docker: multi-stage images + compose bring db+api+web up; volume-persisted DB. (PRD #3)
8. GitHub Actions CI green: fmt, lint, vet, tests, coverage >= 80%, frontend builds. (PRD #3, #17)
9. gofmt clean + golangci-lint no critical issues. (PRD #17)
10. README documents setup + architecture + ER diagram + admin creds with change warning. (PRD #15, #17)
11. Frontend: responsive retro-enterprise SPA, all pages + module standards (search/filter/sort/paginate/export/table features/empty/loading/error), WCAG AA, keyboard-first, 200ms animation cap. (design.md #16)
12. Response time <300ms on hot endpoints (local). (PRD #6)
13. Final verification wave F1-F4 all approve.