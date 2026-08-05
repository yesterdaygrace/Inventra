# Testing Strategy --- Enterprise Inventory Management System Phase 1

**Version:** 1.0\
**Phase:** 1\
**Last updated:** 2026-08-05

---

## 1. Overview

This document defines the testing strategy for Phase 1 of the Enterprise Inventory
Management System. It establishes the test pyramid, tooling conventions, coverage
gate, and workflow that every todo and CI pipeline must follow.

**Goal:** ≥80% coverage across the Go backend, enforced automatically.

---

## 2. Test Pyramid

| Rung | Target | Coverage | Tools | Speed budget |
|------|--------|----------|-------|--------------|
| **Repository tests** | Real PostgreSQL 17 (dockerized) | ~40% | `testing`, `testify` | <1 s each |
| **Service tests** | Mocked repositories | ~35% | `testing`, `testify/mock` | <200 ms each |
| **Handler + Middleware tests** | httptest / gin test router | ~20% | `testing`, `httptest`, `gin` | <100 ms each |
| **E2E scenarios** | Full app via docker compose | ~5% | `testing` with dockerized DB | seconds |

### 2.1 Repository Tests (Dockerized PostgreSQL 17)

- **Database:** PostgreSQL 17-alpine via `docker compose up -d db`
- **Schema:** AutoMigrate (no migrations tool); truncate between tests
- **Isolation:** Dedicated test database/schema (`DATABASE_URL_TEST`)
- **No sqlmock:** Always use real DB for repository tests
- **Pattern:**

```go
// Given: fixture data inserted via AutoMigrate + test fixture
// When: repository method called
// Then: assert result matches expected values, DB state correct
```

### 2.2 Service Tests (Mocked Repositories)

- **Tool:** `testify/mock` for repository interfaces
- **No real DB:** All DB calls mocked
- **Assert calls + return values only**
- **Pattern:**

```go
mockRepo := new(MockRepository)
mockRepo.On("FindById", mock.Anything, uuid.New()).Return(expected, nil)
service := NewService(mockRepo)
result := service.Get(id)
mockRepo.AssertExpectations(t)
```

### 2.3 Handler + Middleware Tests (httptest)

- **Router:** `gin.CreateTestContext()` + test router
- **No network:** Pure memory routing
- **Assert status code, JSON envelope shape, headers**
- **Pattern:**

```go
w := httptest.NewRecorder()
c, _ := gin.CreateTestContext(w)
c.Set("userID", adminID)
handler.GetProducts(c)
assert.Equal(t, http.StatusOK, w.Code)
```

---

## 3. TDD Workflow

**Rule:** Every new feature or bug fix follows **Red → Green → Refactor**.

### 3.1 Red (Write failing test first)

1. Write **at least two tests per behavior**:
   - **Happy path** (successful flow)
   - **Failure path** (error conditions, boundary cases)
2. Run tests — **confirm they fail for the right reason**
   - Missing function → `undefined` or compile error
   - Wrong assertion → assertion failure (not import error)
3. **Do not proceed** until tests fail correctly

### 3.2 Green (Minimal implementation)

1. Write the **smallest code change** to make the test pass
2. Resist adding second case until first passes
3. **One `When` per test** — split if multiple actions

### 3.3 Refactor (With test green)

1. Restructure with confidence (test is safety net)
2. Add edge cases if new patterns emerge
3. Keep tests fast (<10 ms unit, <1 s integration)

---

## 4. Coverage Gate Command

**Single source of truth for all CI and local verification:**

```bash
make test-cover
```

Which executes:

```bash
go test ./... -covermode=atomic -coverprofile=coverage.out && \
awk '/^total/ { if ($3+0 < 80) { print "Coverage", $3, "% is below 80% threshold"; exit 1 }}' coverage.out
```

### 4.1 What this does

1. Runs **all tests** in the project (`./...`)
2. Uses **atomic mode** for accurate branch coverage
3. Writes profile to `coverage.out`
4. Extracts total coverage percentage with `awk`
5. **Fails the step** if total coverage < 80%

### 4.2 Per-package minimum

- No individual package is gated below 80% (enforced by `./...` combined with overall gate)
- CI will fail if any package drops coverage during a change

### 4.3 Local verification

Developers run `make test-cover` **before each commit**. CI runs it on every push/PR.

---

## 5. DB Test Strategy (Repository Layer)

### 5.1 Environment Variable

```bash
DATABASE_URL_TEST=postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable
```

### 5.2 Database Lifecycle

| Phase | Action |
|-------|--------|
| **Setup** | `docker compose up -d db` (or `docker run postgres:17-alpine`) |
| **Before Suite** | Create test schema, `AutoMigrate` all models |
| **Before Each Test** | Truncate all tables (not drop/recreate) |
| **After Each Test** | No cleanup needed (truncated before next test) |
| **After Suite** | `docker compose down` (clean up via Makefile target) |

### 5.3 Truncation Pattern

```go
// internal/shared/database/testdb.go
func TruncateTables(db *gorm.DB, tables []string) error {
    for _, table := range tables {
        if err := db.Exec("TRUNCATE ? CASCADE", table).Error; err != nil {
            return err
        }
    }
    return nil
}
```

---

## 6. Testify Conventions

### 6.1 Assert vs Require

| Use | When |
|-----|------|
| `assert.*` | Continue test after failure (non-fatal) |
| `require.*` | Stop test immediately (fatal, e.g., setup failure) |

**Rule:** Use `require` for setup failures, `assert` for behavior assertions.

### 6.2 Mocking with testify/mock

**Repository interface pattern:**

```go
type ProductRepository interface {
    Create(ctx context.Context, p *Product) error
    FindByID(ctx context.Context, id uuid.UUID) (*Product, error)
    List(ctx context.Context, filters Filters, pagination Pagination) ([]Product, int64, error)
}
```

**Mock generation (manual for simplicity):**

```go
type MockProductRepository struct {
    mock.Mock
}

func (m *MockProductRepository) Create(ctx context.Context, p *Product) error {
    args := m.Called(ctx, p)
    return args.Error(0)
}
// ... other methods
```

### 6.3 Table-Driven Tests

Preferred for multiple input/output variations:

```go
func TestValidateProduct(t *testing.T) {
    tests := []struct {
        name    string
        product Product
        wantErr bool
    }{
        {"valid", Product{Name: "Widget", Price: 10.0}, false},
        {"empty name", Product{Price: 10.0}, true},
        {"negative price", Product{Name: "Widget", Price: -5}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateProduct(tt.product)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateProduct() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## 7. Evidence Conventions

Every todo must record execution evidence in `.omo/evidence/`.

### 7.1 Required Evidence

| Todo type | Evidence path | Content |
|-----------|--------------|---------|
| Test run | `.omo/evidence/t1.0b/tests.log` | Full test output (stdout/stderr) |
| Coverage run | `.omo/evidence/t1.0b/coverage/coverage.out` | Raw coverage profile |
| Coverage run | `.omo/evidence/t1.0b/coverage/percent.txt` | Total coverage percentage |
| Coverage run | `.omo/evidence/t1.0b/coverage/report.html` | HTML coverage report (optional) |
| Failing test | `.omo/evidence/tX.X/failed-test-*.png` | Screenshots for E2E/DB tests if applicable |

### 7.2 Evidence Template

```bash
# Example for T1.0b
mkdir -p .omo/evidence/t1.0b/coverage

# Run tests, capture output
go test ./... -v -coverprofile=.omo/evidence/t1.0b/coverage/coverage.out 2>&1 | tee .omo/evidence/t1.0b/tests.log

# Compute and record percentage
go tool cover -func=.omo/evidence/t1.0b/coverage/coverage.out | grep total | awk '{print $3}' > .omo/evidence/t1.0b/coverage/percent.txt

# Optional HTML report
go tool cover -html=.omo/evidence/t1.0b/coverage/coverage.out -o .omo/evidence/t1.0b/coverage/report.html
```

---

## 8. Frontend Testing Policy (Phase 1)

**Explicit decision (D2):** No frontend unit-test suite in Phase 1.

### 8.1 What is NOT required

- No Vitest/Jest unit tests for React components
- No RTL (React Testing Library) assertions
- No snapshot tests for UI

### 8.2 What IS required

- **Agent-executed browser QA** via `webapp-testing` skill per page wave
- Verify:
  - Login flow
  - CRUD flows (create, read, update, delete)
  - Empty/loading/error states
  - Responsive behavior at `<768`, `768-1279`, `>=1280`
  - Keyboard navigation
  - Toast behavior and timing

### 8.3 Browser QA checklist per page

- [ ] Page loads (HTTP 200)
- [ ] Auth guard redirects unauthenticated to `/login`
- [ ] Role guard blocks unauthorized navigation
- [ ] Form validation shows inline errors
- [ ] Empty state renders when no data
- [ ] Loading skeleton appears during fetch
- [ ] Error state shows on failure (e.g., 404)
- [ ] CRUD roundtrip: create → list shows item
- [ ] Export CSV downloads correctly
- [ ] Keyboard focus moves logically (Tab order)
- [ ] No focus trap or loss
- [ ] Toast auto-dismisses after ~4s
- [ ] Responsive breakpoints render correctly (sidebar hidden/collapsed/drawer)

---

## 9. Makefile Targets

| Target | Command | Purpose |
|--------|---------|---------|
| `test` | `go test ./... -v` | Run all tests, verbose output |
| `test-cover` | (gate command above) | Enforce ≥80% coverage |
| `test-race` | `go test -race ./...` | Detect data races |
| `lint` | `golangci-lint run` | Static analysis |
| `format` | `gofmt -l .` | Check formatting |

---

## 10. Exclusions

The following are **NOT** covered by this strategy (deferred to Phase 2+):

- Load/performance testing (beyond <300ms manual check)
- Security fuzzing
- UI regression testing (Screenshots)
- End-to-end test automation via Playwright (browser QA is manual per wave)

---

## 11. References

- PRD §14 (Testing)
- PRD §17 (Definition of Done)
- Decision D2 (TDD + ≥80% coverage)
- Locked conventions (dockerized-Postgres repo tests, no sqlmock)
