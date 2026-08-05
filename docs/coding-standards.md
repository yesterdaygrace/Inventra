# Go Coding Standards

**Document Version:** 1.0  
**Phase:** 1 — Foundation  
**Date:** 2026-08-05  
**Status:** SPEC-FIRST — governs all later implementation

---

## 1. Code Formatting

### gofmt Required

All Go code must be formatted with `gofmt`:

```bash
# Check formatting
gofmt -l .

# Format all files
gofmt -w .
```

**Acceptance:** `gofmt -l .` returns no output on clean code.

### gofumpt (Stricter Formatter)

Additionally, use `gofumpt` for stricter formatting:

```bash
gofumpt -l .
gofumpt -w .
```

**Acceptance:** Both `gofmt` and `gofumpt` produce no diffs on committed code.

---

## 2. Linting — golangci-lint v2

All code must pass `golangci-lint` with the strict v2 configuration.

### Enabled Critical Linters

| Linter | Purpose | What It Catches |
|--------|---------|-----------------|
| `errcheck` | Error checking | Unchecked errors (ignoring return values) |
| `govet` | Vet analysis | Shadowed variables, printf args, unreachable code |
| `staticcheck` | Static analysis | Dead code, unused variables, incorrect usage |
| `ineffassign` | Inefficient assignments | Assigning to variables that are never read |
| `unused` | Unused code | Unused functions, variables, imports, fields |
| `gofmt` | Formatting | Files not formatted by gofmt |
| `gofumpt` | Stricter formatting | Unnecessary code complexity |
| `revive` | Style issues | Bad practices, style violations (basics set) |

### golangci-lint Configuration

```yaml
# .golangci.yml (canonical reference)
linters-settings:
  errcheck:
    check-type-assertions: true
    check-blank: true
  govet:
    check-shadowing: true
    settings:
      printf:
        funcs:
          - (github.com/sirupsen/logrus).Infof
          - (github.com/sirupsen/logrus).Errorf
  staticcheck:
    checks: ["all"]
  ineffassign:
  unused:
    check-exported: false
  revive:
    rules:
      - name: comment-spacings
      - name: exported
      - name: errorf
      - name: if-return
      - name: var-naming

linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - ineffassign
    - unused
    - gofmt
    - gofumpt
    - revive

output:
  format: colored-line-number
  print-issued-lines: true
  print-linter-name: true
```

### Lint Command

```bash
# Local lint check
make lint  # runs: golangci-lint run

# CI lint check (GitHub Actions)
golangci-lint run
```

**Acceptance:** `golangci-lint run` completes with exit code 0, no issues.

---

## 3. go vet

All code must pass `go vet`:

```bash
go vet ./...
```

**Acceptance:** `go vet ./...` returns no output on clean code.

---

## 4. Conventional Commits

All Git commits must follow the Conventional Commits format:

```
<type>(<scope>): <summary>
```

### Type Categories

| Type | Purpose | Example |
|------|---------|---------|
| `feat` | New feature | `feat(auth): add refresh token rotation` |
| `fix` | Bug fix | `fix(user): prevent self-deactivation` |
| `docs` | Documentation | `docs(architecture): update layering diagram` |
| `chore` | Maintenance | `chore(init): scaffold module structure` |
| `test` | Test changes | `test(auth): add login failure scenario` |
| `ci` | CI/CD | `ci(github-actions): add coverage gate` |
| `perf` | Performance | `perf(dashboard): optimize aggregate query` |
| `refactor` | Code restructuring | `refactor(product): extract filter builder` |

### Scope Convention

Use module name or shared package:

- Module scopes: `auth`, `user`, `product`, `category`, `inventory`, `dashboard`, `report`, `activitylog`, `shared`
- Shared scopes: `config`, `logger`, `database`, `response`, `validator`, `errors`, `middleware`, `export`

### Full Format Example

```
feat(product): add search/sort/filter/paginate to list endpoint

- Implement dynamic WHERE clause building
- Add sort column whitelist (name, sku, price, created_at)
- Add pagination meta to response envelope
- Add CSV export via shared export util

Closes: #123
```

**Acceptance:** `git log` shows only conventional commit format.

---

## 5. File Size — 250-LOC Ceiling

### Rule

**No Go source file may exceed 250 pure lines of code.**

Measure with:

```bash
awk '!/^[[:space:]]*$/ && !/^[[:space:]]*(\/\/|#)/' <file> | wc -l
```

### When Exceeded

If a file genuinely requires >250 LOC (rare), mark with:

```go
// allow: SIZE_OK — <specific reason>
// Example: // allow: SIZE_OK — generated parser with 300+ states
```

### Refactor Trigger

When adding code that would push a file past 250 LOC:

1. Split by responsibility
2. Extract helper functions/types
3. Move related logic to separate file

**Acceptance:** Every committed file passes the 250-LOC check.

---

## 6. Type Safety — No Type Erasure

### Rule

**Go does not have `any`/`interface{}` type erasure. Do not use it as a escape hatch.**

### Prohibited Patterns

| Forbidden | Why | Correct Approach |
|-----------|-----|------------------|
| `map[string]any` | Unchecked, no compile-time safety | Typed struct with explicit fields |
| `[]interface{}` | Type loss, runtime panics | `[]string`, `[]int`, or typed slice |
| `interface{}` return | No compile-time contract | Typed return value or error |
| Type assertions `.(type)` | Runtime type checking | Generics or interfaces |
| `encoding/json.RawMessage` for domain | Delayed parsing breaks boundaries | Parse at boundary into typed value |

### Typed Struct Example

```go
// BAD
func HandleRequest(body map[string]any) error {
    name := body["name"].(string)  // Runtime panic if wrong
    age := body["age"].(int)       // Runtime panic if wrong
    // ...
}

// GOOD
type CreateUserRequest struct {
    Name string `json:"name" validate:"required,min=2,max=100"`
    Age  int    `json:"age" validate:"required,min=18,max=120"`
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    // Parse and validate at boundary
    // ...
}
```

### Exception: Generic Context Values

Context values may use string keys and `any`:

```go
// OK for context-only, never escape to domain
ctx = context.WithValue(ctx, "user_id", userID)
ctx = context.WithValue(ctx, "role", role)

// Always type-assert at consumption
userID := ctx.Value("user_id").(uuid.UUID)
```

**Acceptance:** No `any`/`interface{}` in domain types, function signatures, or return values.

---

## 7. Panic Rule

### Rule

**No `panic` in library code.**

### Allowed Usage

- `main()` entry point only (for fatal startup errors)
- Tests (for test setup failures)
- Unreachable code with `assert.False(t, true)` pattern

### Example: Startup Panic (OK in main.go)

```go
func main() {
    cfg := config.Load()
    logger := logger.New(cfg)
    
    db, err := database.Connect(cfg.Database)
    if err != nil {
        logger.Fatal("failed to connect to database", zap.Error(err))
    }
    
    // db.Ping() might panic if unrecoverable
    if err := db.Ping(); err != nil {
        panic(err)  // Only in main(), not in library code
    }
    
    // ...
}
```

### Example: Library Code — Error Return (Required)

```go
// BAD
func GetProduct(id uuid.UUID) *Product {
    var p Product
    if err := db.First(&p, id).Error; err != nil {
        panic(err)  // NEVER in library code
    }
    return &p
}

// GOOD
func (r *ProductRepository) GetByID(id uuid.UUID) (*Product, error) {
    var p Product
    if err := r.db.First(&p, id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, &errors.AppError{
                Type:    errors.ErrNotFound,
                Message: "product not found",
                Err:     err,
            }
        }
        return nil, &errors.AppError{
            Type:    errors.ErrInternal,
            Message: "failed to fetch product",
            Err:     err,
        }
    }
    return &p, nil
}
```

**Acceptance:** `golangci-lint` catches `panic` in library code.

---

## 8. TDD Discipline — Failing Test First

### Rule

**Every new feature or fix follows Red → Green → Refactor.**

### The Order

1. **Red** — Write a failing test that names the behavior. Run it. Confirm it fails for the *right reason* (not import error, not typo).
2. **Green** — Write the minimum code to make the test pass. Do not add the second case until the first passes.
3. **Refactor** — With the test green, restructure ruthlessly. The test is your safety net.

### Test Naming Convention

```go
// Structure: Test_<Behavior>_when_<Condition>
func TestProductService_Create_with_valid_input(t *testing.T) { /* ... */ }
func TestProductService_Create_with_duplicate_sku(t *testing.T) { /* ... */ }
func TestProductService_GetByID_not_found(t *testing.T) { /* ... */ }
```

### Given / When / Then in Tests

```go
func TestProductService_Create_with_valid_input(t *testing.T) {
    // Given
    svc := newProductService(mockRepo)
    input := CreateProductInput{
        Name:  "Test Product",
        SKU:   "TEST-001",
        Price: 99.99,
    }

    // When
    product, err := svc.Create(input)

    // Then
    require.NoError(t, err)
    require.NotNil(t, product)
    require.Equal(t, input.Name, product.Name)
    require.Equal(t, input.SKU, product.SKU)
}
```

### Test Pyramid

| Layer | Count | Purpose | Speed Budget |
|-------|-------|---------|--------------|
| Unit | Many | Pure function correctness, happy + edges + boundaries + errors | < 10 ms |
| Integration | Some | Real adapter (DB, queue) via testcontainers | < 1 s |
| E2E Scenario | Few | Full app, real surface, observable outcome | seconds |

**Acceptance:** Every new handler/service/repository has at least one failing test committed first.

---

## 9. Error Wrapping Rule

### Rule

**Always wrap errors with context using `fmt.Errorf("...: %w", err)`.**

### Pattern

```go
// BAD
return nil, errors.New("failed to fetch product")

// GOOD
return nil, &errors.AppError{
    Type:    errors.ErrNotFound,
    Message: "product not found",
    Err:     fmt.Errorf("product repository GetByID(%s): %w", id, err),
}
```

### Typed Errors

All domain errors use the typed error system:

```go
// internal/shared/errors/errors.go
type AppError struct {
    Type    ErrorType `json:"type"`
    Message string    `json:"message"`
    Details string    `json:"details,omitempty"`
    Err     error     `json:"-"`
}

func (e *AppError) Error() string {
    return e.Message
}

func (e *AppError) Unwrap() error {
    return e.Err
}
```

### Check Errors

Use `errors.Is` and `errors.As`:

```go
// BAD
if err != nil {
    // handle generic error
}

// GOOD
if err != nil {
    var appErr *errors.AppError
    if errors.As(err, &appErr) {
        switch appErr.Type {
        case errors.ErrNotFound:
            // handle not found
        case errors.ErrConflict:
            // handle conflict
        default:
            // handle other typed errors
        }
    } else {
        // unexpected error
    }
}
```

### Error Codes Mapping

```go
var errorStatusCode = map[errors.ErrorType]int{
    errors.ErrNotFound:       http.StatusNotFound,
    errors.ErrValidation:     http.StatusBadRequest,
    errors.ErrUnauthorized:   http.StatusUnauthorized,
    errors.ErrForbidden:      http.StatusForbidden,
    errors.ErrConflict:       http.StatusConflict,
    errors.ErrInternal:       http.StatusInternalServerError,
}
```

**Acceptance:** `golangci-lint` passes; all errors wrapped with `%w`.

---

## 10. Repository Interface Pattern

### Structure

Each module defines a repository interface and a GORM concrete implementation:

```go
// internal/product/repository.go
package product

// Repository interface (no GORM dependency)
type Repository interface {
    Create(ctx context.Context, product *Product) error
    GetByID(ctx context.Context, id uuid.UUID) (*Product, error)
    GetBySKU(ctx context.Context, sku string) (*Product, error)
    List(ctx context.Context, filters ProductFilters, pagination Pagination) ([]Product, int64, error)
    Update(ctx context.Context, product *Product) error
    Delete(ctx context.Context, id uuid.UUID) error
}

// GORM implementation (concrete)
type productRepository struct {
    db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
    return &productRepository{db: db}
}

// Implement interface methods
func (r *productRepository) Create(ctx context.Context, product *Product) error {
    return r.db.Create(product).Error
}

func (r *productRepository) GetByID(ctx context.Context, id uuid.UUID) (*Product, error) {
    var p Product
    if err := r.db.First(&p, id).Error; err != nil {
        return nil, err
    }
    return &p, nil
}

// ...
```

### DI Wiring

```go
// cmd/server/main.go
productRepo := product.NewRepository(db)
productService := product.NewService(productRepo, log, cfg)
productHandler := product.NewHandler(productService, log)
```

**Acceptance:** Repository interface has no GORM imports; concrete impl is in same file or `repository_gorm.go`.

---

## 11. Context Propagation

### Rule

**Always pass `context.Context` as the first parameter.**

### Pattern

```go
// BAD
func (s *Service) GetProduct(id uuid.UUID) (*Product, error) {
    return s.repo.GetByID(id)
}

// GOOD
func (s *Service) GetProduct(ctx context.Context, id uuid.UUID) (*Product, error) {
    return s.repo.GetByID(ctx, id)
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Product, error) {
    // ...
}
```

### Logger with Context

```go
func (s *Service) GetProduct(ctx context.Context, id uuid.UUID) (*Product, error) {
    log := logger.FromContext(ctx)  // Extract from context
    log.Info("fetching product", zap.String("product_id", id.String()))
    
    return s.repo.GetByID(ctx, id)
}
```

**Acceptance:** All repository and service methods have `ctx context.Context` as first parameter.

---

## 12. Testing Tooling — Testify

### Imports

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

### assert vs require

- `assert` — continues test on failure (for soft checks)
- `require` — fails test immediately (for hard dependencies)

### Example: Service Test

```go
func TestProductService_Create_with_valid_input(t *testing.T) {
    // Mock setup
    mockRepo := &mocks.Repository{}
    mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*product.Product")).
        Return(nil).Once()
    mockRepo.On("GetBySKU", mock.Anything, "TEST-001").
        Return(nil, errors.New("not found")).Once()

    svc := NewService(mockRepo, log, cfg)

    // Run
    product, err := svc.Create(context.Background(), CreateProductInput{
        Name:  "Test Product",
        SKU:   "TEST-001",
        Price: 99.99,
    })

    // Verify
    require.NoError(t, err)
    assert.NotNil(t, product)
    assert.Equal(t, "Test Product", product.Name)
    assert.Equal(t, "TEST-001", product.SKU)
    assert.True(t, product.ID.Valid())

    // Assert mock was called
    mockRepo.AssertExpectations(t)
}
```

### Example: Repository Test (Dockerized)

```go
func TestProductRepository_Create(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping DB test in short mode")
    }

    // Given: Dockerized DB (testcontainers)
    db := setupTestDB(t)
    repo := NewRepository(db)

    // When
    product := &Product{
        Name:  "Test Product",
        SKU:   "TEST-001",
        Price: 99.99,
    }
    err := repo.Create(context.Background(), product)

    // Then
    require.NoError(t, err)
    assert.NotEmpty(t, product.ID)

    // Verify record exists
    var count int64
    db.Model(&Product{}).Where("sku = ?", "TEST-001").Count(&count)
    assert.Equal(t, int64(1), count)
}
```

**Acceptance:** All tests use testify; repo tests against dockerized DB.

---

## 13. Handler Validation Pattern

### validator/v10 Wrapper

```go
// internal/shared/validator/validator.go
package validator

import "github.com/go-playground/validator/v10"

type Validator struct {
    v *validator.Validate
}

func New() *Validator {
    return &Validator{
        v: validator.New(),
    }
}

func (v *Validator) Validate(obj interface{}) error {
    if err := v.v.Struct(obj); err != nil {
        return &errors.AppError{
            Type:    errors.ErrValidation,
            Message: "validation failed",
            Details: err.Error(),
        }
    }
    return nil
}
```

### Handler Usage

```go
func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    log := logger.FromContext(ctx)

    var req CreateProductRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        response.WriteError(w, &errors.AppError{
            Type:    errors.ErrValidation,
            Message: "invalid JSON",
        })
        return
    }

    // Validate with validator/v10
    if err := h.validator.Validate(req); err != nil {
        response.WriteError(w, err)
        return
    }

    // ... rest of handler
}
```

**Acceptance:** All handlers validate input DTOs with validator/v10.

---

## 14. Response Envelope Usage

### Pattern

```go
// internal/shared/response/response.go
type Envelope struct {
    Success bool        `json:"success"`
    Message string      `json:"message,omitempty"`
    Data    interface{} `json:"data,omitempty"`
    Error   *AppError   `json:"error,omitempty"`
}

type Pagination struct {
    Page      int `json:"page"`
    PerPage   int `json:"per_page"`
    Total     int `json:"total"`
    TotalPages int `json:"total_pages"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}, pagination *Pagination) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    
    envelope := Envelope{
        Success: true,
        Data:    data,
    }
    if pagination != nil {
        envelope.Pagination = pagination
    }
    
    json.NewEncoder(w).Encode(envelope)
}
```

### Handler Usage

```go
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    log := logger.FromContext(ctx)

    // Parse query params
    filters, pagination, err := parseProductFilters(r)
    if err != nil {
        response.WriteError(w, err)
        return
    }

    // Service call
    products, total, err := h.service.List(ctx, filters, pagination)
    if err != nil {
        response.WriteError(w, err)
        return
    }

    // Write response
    response.WriteJSON(w, http.StatusOK, products, &pagination)
}
```

**Acceptance:** All handler responses use the envelope; never raw JSON.

---

## 15. Logger Usage

### Pattern

```go
// Log at decision points, never in helpers
func (s *Service) CreateProduct(ctx context.Context, input CreateProductInput) (*Product, error) {
    log := logger.FromContext(ctx)

    // Log at decision point
    log.Info("creating product",
        zap.String("name", input.Name),
        zap.String("sku", input.SKU),
        zap.Float64("price", input.Price),
    )

    // ...
}
```

### Log Levels

| Level | Use Case |
|-------|----------|
| `Debug` | Detailed tracing, development only |
| `Info` | Normal operations, decision points |
| `Warn` | Unexpected but recoverable |
| `Error` | Errors (wrapped with `zap.Error(err)`) |

### Context-Aware Logger

```go
// logger/logger.go
func FromContext(ctx context.Context) *zap.Logger {
    if log, ok := ctx.Value("logger").(*zap.Logger); ok {
        return log
    }
    return defaultLogger
}

func WithContext(ctx context.Context, log *zap.Logger) context.Context {
    return context.WithValue(ctx, "logger", log)
}
```

**Acceptance:** Logs are structured (key-value), not prose; no `fmt.Printf` for production logging.

---

## 16. Database Transactions

### Pattern

```go
func (s *Service) StockIn(ctx context.Context, input StockInInput) error {
    log := logger.FromContext(ctx)

    return s.db.Transaction(func(tx *gorm.DB) error {
        // 1. Insert inventory transaction
        transaction := &InventoryTransaction{
            ProductID: input.ProductID,
            Type:      TransactionTypeStockIn,
            Quantity:  input.Quantity,
            UnitCost:  input.UnitCost,
            Note:      input.Note,
            UserID:    input.UserID,
        }
        if err := tx.Create(transaction).Error; err != nil {
            log.Error("failed to create inventory transaction",
                zap.Error(err),
                zap.String("product_id", input.ProductID.String()),
            )
            return fmt.Errorf("create inventory transaction: %w", err)
        }

        // 2. Update inventory quantity (upsert)
        inventory := &Inventory{
            ProductID: input.ProductID,
        }
        if err := tx.FirstOrCreate(inventory, Inventory{ProductID: input.ProductID}).Error; err != nil {
            return fmt.Errorf("find or create inventory: %w", err)
        }

        inventory.Quantity += input.Quantity
        if err := tx.Save(inventory).Error; err != nil {
            return fmt.Errorf("update inventory quantity: %w", err)
        }

        // Transaction commits if no error returned
        return nil
    })
}
```

### Transaction Rollback

If any step returns an error, GORM automatically rolls back.

**Acceptance:** Stock movements use transactions; no partial writes.

---

## 17. CI Gate

### Command

```bash
# Local pre-commit gate
make pre-commit

# CI gate (GitHub Actions)
go test -race -shuffle=on -count=1 ./...
```

### Coverage Gate

```bash
# Coverage calculation
go test -race -shuffle=on -count=1 -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total: | awk '{print $NF}' | tr -d '%'
```

### CI Workflow Gates

```yaml
# .github/workflows/ci.yml
jobs:
  test:
    run: |
      go test -race -shuffle=on -count=1 ./...
  
  coverage:
    run: |
      go test -race -shuffle=on -count=1 -covermode=atomic -coverprofile=coverage.out ./...
      COVERAGE=$(go tool cover -func=coverage.out | grep total: | awk '{print $NF}' | tr -d '%')
      if [ "$(echo "$COVERAGE < 80" | bc -l)" -eq 1 ]; then
        echo "Coverage $COVERAGE% is below 80%"
        exit 1
      fi
```

**Acceptance:** CI fails if coverage < 80% per package.

---

## 18. Reference: Complete Checklist

Before committing code, verify:

- [ ] `gofmt -l .` returns no files
- [ ] `gofumpt -l .` returns no files
- [ ] `golangci-lint run` exits 0, no issues
- [ ] `go vet ./...` returns no issues
- [ ] `go test -race -shuffle=on -count=1 ./...` passes
- [ ] Coverage ≥ 80% per package
- [ ] No `any`/`interface{}` in domain types
- [ ] No `panic` in library code
- [ ] All errors wrapped with `%w`
- [ ] All handlers validate with validator/v10
- [ ] All responses use envelope
- [ ] All service/repository methods have `ctx` first
- [ ] All new code has failing test first (Red)
- [ ] Committed with conventional commit format
- [ ] File ≤ 250 pure LOC (unless SIZE_OK marked)
- [ ] Tests use testify (`assert`/`require`)

---

*This document is SPEC-FIRST. All code in Phase 1 must conform to these standards. Later todos reference these rules in their acceptance criteria.*
