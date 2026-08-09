# Wave 3 Verification — Operations: Inventory + Transactions + Reports

## Scope
Per `.omo/plans/frontend-waves.md` Wave 3: stock-in/out forms, transactions
history, low-stock indicator; reports (stock summary + CSV export). Built
against the live `:8090` demo backend.

## Frontend changes
- `web/src/types/api.ts` — added `StockSummary`, `CategorySummary`,
  `ReportLowStockItem` (matching the report read-model).
- `web/src/lib/api.ts` — added `reportApi` (`summary`, `exportCsv`,
  `exportLowStockCsv`) and `inventoryApi.exportCsv` (both via the shared
  `downloadCsv` helper with 401-refresh); `StockSummary` type import.
- `web/src/lib/query.ts` — added `listKeys.reports`.
- `web/src/components/inventory/stock-movement-dialog.tsx` — RHF+zod dialog
  for stock in/out: product select (from active products), quantity (>0),
  optional unit cost + note; surfaces API errors (e.g. overdraw 409) inline.
- `web/src/pages/inventory.tsx` — stock-level list (product/sku, qty,
  in/out-of-stock badge), search + low-stock filter, pagination, Export CSV,
  role-gated Stock In / Stock Out (header buttons + per-row menu); invalidates
  inventory/products/transactions/dashboard/reports after a movement.
- `web/src/pages/transactions.tsx` — movement history (item, date, IN/OUT
  badge, qty, unit cost, note) with product + type filters and pagination.
- `web/src/pages/reports.tsx` — KPIs (total products, total stock value),
  stock-by-category table, low-stock items table, Export summary + Export
  low-stock CSV buttons.
- `web/src/App.tsx` — `/inventory`, `/transactions`, `/reports` now render the
  real pages (placeholders removed).

## Build
- `npx tsc -b --noEmit` clean; `npm run build` green (2387 modules). Chunk-size
  warning pre-existing.

## Live verification (Playwright, admin, against live API)
- **Inventory**: list renders seeded stock (27in Monitor qty 100). Stock in
  +5 with unit cost → qty 100→105, success toast. Export button present.
  Low-stock filter toggles (0 rows = correctly none low after seed). Per-row
  Actions menu → Stock in/out opens the dialog pre-selected.
- **Overdraw 409**: stock-out 999999 on a stocked item → backend 409 surfaced
  inside the dialog as an inline alert (no crash, no duplicate toast).
- **Transactions**: new movement recorded — "27in Monitor, In, 5, $12.50";
  product filter + type filter present; selecting OUT sends
  `?type=OUT` (verified via network) and correctly returns the filtered set.
- **Reports**: KPI cards + stock-by-category table (real category names/
  values) + low-stock table render; `stock-summary-2026-08-07.csv` and
  `low-stock-2026-08-07.csv` both download.
- Console: only the expected 401 refresh noise; no page errors.

## Evidence
- `demo-13-inventory.png`, `demo-14-transactions.png`, `demo-15-reports.png`,
  `demo-16-inventory-overdraw.png`.

## Notes
- Backend unchanged this wave (no API bugs surfaced; endpoints already
  contract-correct, incl. the export-per-page fix from Wave 2 covering
  inventory/report CSV exports via the same paginated List path where used).
- The overdraw 409 message string from the backend is the generic
  `"conflict"`; surfaced verbatim in the dialog — acceptable, no backend
  change needed.
