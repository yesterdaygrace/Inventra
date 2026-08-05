# Inventra API — Go Implementation Standards

Authoritative standard for writing backend code in this repository. Every later
backend task (Categories, Products, Inventory, ActivityLog, User admin, Reports)
MUST follow this document. Deviations are flagged by the plan's verification
waves (F1/F2).

---

## 1. Package layout

Top-level module path is `inventory`. All application code lives under
`internal/`; the only public entrypoints are `cmd/`.

```
cmd/
  server/main.go          # binary entrypoint: wire config -> logger -> db -> router -> modules
  seed/{main,demo}.go     # seeding CLI
internal/
  <module>/               # one package per bounded context, e.g. auth, product, inventory
    model.go              # GORM models + domain types
    repository.go         # persistence (GORM), implements the module's Repository interface
    service.go            # business logic, consumes Repository interface, produces typed errors
    handler.go            # Gin handlers + request/response DTOs
    router.go             # RegisterRoutes(group) wiring
    *_test.go             # tests colocated with the code under test
  shared/                 # cross-cutting, dependency-light packages
    errors/               # sentinel domain errors
    response/             # unified JSON envelope
    validator/            # shared struct validator wrapper
    logger/               # zap structured logger
    config/               # env-based configuration
    database/             # connection + AutoMigrate + model registry
          middleware/     # CORS, secure headers, request ID, auth, RBAC
          router/         # engine bootstrap + healthcheck
          export/         # CSV export helper
```

Rules:

- **No package outside `internal/` may import another internal package except
  through `shared/` or its own `cmd/` layer.** Domain modules may import
  `shared/*`. `shared/*` must never import a domain module (prevents cycles).
- **Import-cycle rule:** if package A (e.g. `activitylog`) imports package B
  (e.g. `auth`) at the model level, then B must NOT import A. When a module
  needs to *use* another module's concept without a cycle, define a minimal
  decoupled struct on the consuming side (see `auth.ActivityLogEntry`) and let
  the concrete repository that "owns" persistence bridge to the other table.
- One logical concern per file; files stay small and readable.

## 2. Repository interface pattern (the spine)

Every module defines a **`Repository` interface in `service.go`** that
abstracts persistence. The concrete implementation lives in `repository.go`
and is injected into the service at construction time.

```go
// service.go
type Repository interface {
    CreateUser(u *User) error
    FindUserByEmail(email string) (*User, error)
    FindUserByID(id uuid.UUID) (*User, error)
    UpdateUser(u *User) error

    FindRoleByName(name string) (*Role, error)
    FindRoleByID(id uuid.UUID) (*Role, error)

    CreateRefreshToken(t *RefreshToken) error
    FindRefreshTokenByHash(hash string) (*RefreshToken, error)
    UpdateRefreshToken(t *RefreshToken)
    CreateActivityLog(entry ActivityLogEntry)
}
```

- Methods take/return **domain models or value objects only** — no GORM `*DB`**
  leaks into the service.
- Services test against a **mock of the interface**
  (via `testify/mock`), never against the DB.
- The concrete impl lives in `repository.go` and is **DTO-free** on the way back out:
  it owns GORM `*gorm.DB`, mapping to physical tables (e.g. `activityLogRow`) as needed.

```go
type GORMRepository struct{ db *gorm.DB }
func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db: db} }

func (r *GORMRepository) FindUserByEmail(email string) (*User, error) { /* ... */ }
```

**Mock filename**: every interface method must have a corresponding mock.**

## 3. Service definition + error wrapping

- Sentinel typed errors live in `internal/shared/errors`:

```go
var (
    ErrNotFound   = errors.New("not found")
    ErrValidation = errors.New("validation failed")
    ErrUnauthorized = errors.New("unauthorized")
    ErrForbidden  = errors.New("forbidden")
    ErrConflict   = errors.New("conflict")
    ErrInternal   = errors.New("internal error")
)
```

- Services **wrap** sentinels with `%w` to add context while preserving
  identity for `errors.Is`:

```go
// domain-level domain error wrapping a sentinel
var ErrEmailTaken = fmt.Errorf("%w: email already registered", sharederr.ErrConflict)
```

- Services return the **wrapped/typed error upward**, never `nil` on failure.
- The handler layer translates typed errors to HTTP via the shared envelope
  (section 4) — services must not emit `*gin.Context`.

```go
func (s *Service) ResetEmail(email string) error {
    if s.repo.EmailExists(email) {
        return sharederr.ErrConflict // or a domain-wrapped sentinel
    }
    ...
}
```

## 4. Response envelope + error -> status mapping

All handlers respond through `internal/shared/response`, never raw `c.JSON`.

```go
Body struct {
    Success    bool        `json:"success"`
    Message    string      `json:"message,omitempty"`
    Data       any         `json:"data,omitempty"`
    Pagination *Pagination `json:"pagination,omitempty"`
}
```

Status mapping (in `response.statusFor`):

| Squared error            | HTTP |
|--------------------------|------|
| `ErrValidation`          | 400  |
| `ErrUnauthorized`        | 401  |
| `ErrForbidden`           | 403  |
| `ErrNotFound`            | 404  |
| `ErrConflict`            | 409  |
| anything else / unknown  | 500  |

Helper calls:

```go
response.OK(c, data)                 // 200 {success:true,data}
response.Created(c, data)            // 201
response.Message(c, msg)             // 200 {success:true,message}
response.Paginated(c, data, &pg)     // 200 with pagination meta
response.Error(c, err)               // maps typed err -> status+message
```

`Pagination` metadata:

```go
type Pagination struct {
    Page       int   `json:"page"`
    PerPage    int   `json:"per_page"`
    Total      int64 `json:"total"`
    TotalPages int   `json:"total_pages"`
}
```

## 5. DTO + validation pattern

- Request structs are **unexported** in the handler file, tagged with
  `binding:"..."`, and validated via the shared validator wrapper
  (`internal/shared/validator`, wrapping `go-playground/validator/v10`):

```go
type registerRequest struct {
    Name     string `json:"name"     binding:"required,min=2"`
    Email    string `json:"email"    binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
}
```

- Validation is always invoked as `v.Validate(&req)`; on error the service/handler
  returns `ErrValidation` wrapped so the map goes to 400.
- Handlers **never** parse/seed persistence; they bind -> validate -> call service
  -> map the result to a `*Envelope`/`*Error`.
- Response DTOs (e.g. `LoginEnvelope`, `userResponse`) conceal internal columns
  (never leak password hashes or refresh tokens).

## 6. Service layer

```go
type Service struct {
    repo   Repository
    tokens *TokenManager
    cost   int
}
func NewService(repo Repository, tokens *TokenManager, bcryptCost int) *Service
```

- **Services are the only place that encodes business rules** (e.g. RBAC
  guards, JWT TTL, low-stock thresholds) — not handlers, not the repo.
- Services take domain fetches (by ID/username/etc.) as UUIDs and return domain
  results or typed errors. Handlers translate.
- Consider a `New*` constructor that receives its own Repository so tests can
  inject mocks without touching the real DB.

## 7. Transactions for inventory (W7)

Stock-in / stock-out flows must mutate **whole** in a DB transaction so a
failure never leaves partial rows. Since the repository wraps a `*gorm.DB`,
implement atomic mutations as repository methods that begin a transaction
internally (or accept a `Tx`), and roll back on any error:

```go
func (r *GORMRepository) StockMovement(txFn func(tx *gorm.DB) error) error {
    return r.db.Transaction(func(tx *gorm.DB) error { return txFn(tx) })
}
```

- Within a movement: insert an `inventory_transactions` history row **and**
  upsert `inventory.Quantity` in the same transaction.
- Reject overdraw (moving out more than available) with a conflict/409 **before
  writing** — the transaction must roll back so no partial row persists.
- Covers behavior is verified by `development`-side QA (mock/DB test: stock-in
  then stock-out nets correct qty; overdraw test leaves no partial row).

## 8. Context propagation & request IDs

- Attach cross-cutting values to `*gin.Context` via typed helpers in
  `shared/middleware`:

  - `middleware.RequestID()` — stores UUID under `RequestIDKey` and echoes it
    in the `X-Request-ID` response header.
  - `middleware.Auth(parser)` — extracts a JWT, verifies it, and puts the
    authenticated **user id** and **role** into the Gin context.
  - `middleware.RoleRequired(ADMIN)` — checks the role stored by Auth and
    returns 403 if the caller lacks it.

- Handlers read context values via the helper getters
  (`middleware.UserIDFromContext(c, key)` etc.) rather than `c.Get` directly.
- DB reads/writes pass `ctx` through the service -> repository call chain
  (GORM supports `.WithContext(ctx)`) so cancellation and request IDs propagate.

## 9. Logging

- `shared/logger.New(cfg)` returns a `*zap.Logger` and wires the configured
  `LOG_LEVEL`. In dev it uses a human console encoder; in production a JSON
  encoder — both include timestamps and caller info.
- Modules **never construct their own logger**; they accept one (or construct
  at wiring time) and log via `log.Info(msg, zap.String("email",...))`.
- `cmd/server` uses a single logger instance built in `main` and passed down;
  failures are `log.Fatal` at boot only.

## 10. Module wiring / Dependency Injection (in `cmd/server/main.go`)

Modular construction is done top-down, explicitly, in `main.go`:

```go
r := router.New(cfg)
authRepo := auth.NewGORMRepository(db)
tm := auth.NewTokenManager(auth.TokenManagerConfig{Secret: cfg.JWTSecret, ...})
svc := auth.NewService(authRepo, tm, cfg.BCryptCost)
h := auth.NewHandler(svc, validator.New())
auth.RegisterRoutes(r.Group("/api/v1"), h)
```

- Each module exposes a `RegisterRoutes(group *gin.RouterGroup, deps ...)` so
  mounting is a one-liner.
- `shared/router.New(cfg)` already applies `CORS`, `SecureHeaders`, `RequestID`,
  and a `/healthz` probe; module routes are layered on top of it.

## 11. Conventions checklist

- Repository interface lives next to its service; concrete impl in `repo.go`.
- Services return sentinel-wrapped domain errors; no `*gin.Context` anywhere
  except handlers.
- Handlers go through `response` helpers; no raw `c.JSON`.
- DTOs validated via `shared/validator`; structural/domain rules validated in
  service, not handler.
- Login/report/CSV export set HTTP headers explicitly (e.g.
  `c.Header("Content-Type","text/csv")`, `Content-Disposition`).
- Tests sit colocated, are deterministic (`-p 1` when run), and use
  `testify/mock` for repository seams. No test imports another module's test.

## 12. Generic helpers

Prefer repository methods that do the SQL over closed loops in services; keep
any typed generics (e.g. `paged[T]`, `upsert[T]`) centralized in `shared/`
when they are reused by >1 module. A single module should inline its own, only.