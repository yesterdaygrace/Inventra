# Wave 2 Verification — Backend DELETE alignment + Products/Categories UI

## Scope
Per `docs/api.md`, product `DELETE` must be a **soft archive** and category
`DELETE` must be a **soft deactivate** (rows preserved for audit/reporting),
plus category list must expose `is_active` and `product_count`. Backend was
aligned to the documented contract; the Wave 2 Products/Categories UI was
verified against the live backend.

## Backend changes
- `internal/product/repository.go` — `Delete` now `UPDATE is_archived = true`
  (soft archive) instead of `DELETE`; undocumented-not-found behavior preserved.
- `internal/product/handler.go` — message `"product archived"`.
- `internal/category/model.go` — added `IsActive` (persisted) and read-only
  `ProductCount`; hot-automigrated by GORM (no SQL migration, no `is_active`
  breakage).
- `internal/category/repository.go` — `Delete` now sets `is_active=false`; `List`
  adds `product_count` subquery and an optional `is_active` filter;
  `CountProductsFor` counts only non-archived products (allows deactivating a
  category that only has archived products).
- `internal/category/service.go` — `Update` accepts optional `is_active`.
- `internal/category/handler.go` — envelope now returns `is_active` +
  `product_count`; list accepts `is_active=true|false`; update accepts
  `is_active`; message `"category deactivated"`.
- `internal/dashboard/repository.go` — `CountCategories` and
  `CategoryDistribution` count only active categories.

## Test changes
- Product/category repo "Delete" tests assert the row *persists* flagged
  archived/inactive and that a bogus id still yields `ErrNotFound`; category
  service `Update` call sites updated for the new signature.
- `go build ./...` clean; `go test -p 1 ./...` green (run serially; parallel
  `make test` races because all DB packages share one live DB and drop each
  other's tables — pre-existing and unrelated to these changes).

## Live verification (curl against restarted `:8090` demo server)
- Product archive: create → `DELETE` →
  `{"success":true,"message":"product archived"}`; product still in DB
  (`is_archived=true`); excluded from `?is_archived=false` list.
- Category deactivate: create → `DELETE` →
  `{"success":true,"message":"category deactivated"}`; category still in DB
  (`is_active=false`); included in `?is_active=false` filter.
- Category list payload now carries `is_active` and `product_count`
  (e.g. `Electronics: is_active true, product_count 5`).

## Live UI verification (Playwright at :5173, admin flow)
- Products default list excludes archived products; the **Archived** toggle
  reveals them with an "Archived" badge — matches the new soft-archive backend.
- Categories page shows deactivated category persisting with an "Inactive"
  badge and `product_count` (0) — soft-deactivate round trip intact.
- No console errors on either page (only the standard React DevTools notice).

## Evidence
- `demo-10-categories-inactive-badge.png` — deactivated category with Inactive badge.
- `demo-11-products-archived-view.png` — archived product visible under Archived + search.
## Wave 2 completion pass (export + sort)

Added the remaining Wave 2 checklist items to the Products/Categories UI:

### Frontend changes
- `web/src/lib/api.ts` — new `downloadCsv()` (authenticated CSV fetch with
  401 refresh; errors surfaced via `ApiError`); `productApi.exportCsv()` and
  `categoryApi.exportCsv()`; `ProductQuery` now includes optional `sort`.
- `web/src/pages/products.tsx` — **sort** control (`Name`, `Price ×2`,
  `Newest/Oldest`) wired to the documented `sort` param; **Export CSV** button
  (any authenticated role, per backend read-route RBAC).
- `web/src/pages/categories.tsx` — **Export CSV** button + success/error toast.

### Backend bug found + fixed (plan guardrail — API bug discovered via UI test)
- **Issue:** both export handlers called `ListQuery{PerPage: 1000}`, but the
  list repositories clamp `PerPage` to ≤100, so exports silently dropped rows
  beyond the first page (e.g. the last product, Wireless Mouse). Category repo
  clamped to 20, so it was worse.
- **Fix**: product and category `Export` handlers now page through the catalog
  (up to 100/page) until the full `total` is collected.

### Tests
- Added `TestExportPaginatesBeyondCap` (product + category handler tests)
  asserting all rows beyond the cap are exported.
- `go build ./...` clean; product/category/export tests pass.

### Live verification
- Product **sort**: default Name A–Z; Price high→low → Standing Desk $499.99;
  Name Z–A → Wireless Mouse. Sort control present.
- Product **export**: downloads `products-2026-08-07.csv` (toast shown); CSV now
  contains the full catalog (incl. Wireless Mouse previously dropped), verified
  after the backend fix via curl (all-rows export).
- Category **export**: downloads `categories-2026-08-07.csv` + success toast.
- No console errors on Products/Categories pages.

### Evidence
- `demo-12-products-sort-export.png` — Products page showing Sort + Export controls.
