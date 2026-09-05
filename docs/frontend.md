# Frontend — Inventra Web

**Stack:** React 19 + TypeScript 5.7 + Vite 6 + Tailwind CSS 4 + Radix UI + TanStack Query 5 + React Router 7 + react-hook-form + Zod + Recharts
**Screens:** 12 pages (`dashboard`, `products`, `categories`, `warehouses`, `inventory`, `transactions`, `reports`, `users`, `activity`, `settings`, `login`, `register`) + responsive variants
**State:** TanStack Query (`useApiQuery`, `useList`) + shadcn-style `components/ui` kit

> All 18 screenshots below are live captures from the current `web/` build (`demo-*.png` at repo root). Open `docs/database-erd.html` for the DB, this file for the UI.

---

## Dashboard

| Live | Dark | Desktop | Mobile |
|---|---|---|---|
| ![Dashboard Live](../demo-05-dashboard-live.png) | ![Dashboard Dark](../demo-06-dashboard-dark.png) | ![Dashboard Desktop](../demo-08-dashboard-desktop.png) | ![Dashboard Mobile](../demo-07-dashboard-mobile.png) |

**What it shows:** KPIs (total products, stock value, low-stock, out-of-stock), recent ledger activity, category distribution, inventory movement. Data from `GET /dashboard/summary` + `GET /dashboard/activity` — aggregates derived from `inventory_ledger` and `activity_logs`.

---

## Products

| List | Sort & Export | Archived |
|---|---|---|
| ![Products List](../demo-09-products-list.png) | ![Products Sort Export](../demo-12-products-sort-export.png) | ![Products Archived](../demo-11-products-archived-view.png) |

**What it shows:** CRUD, search (`q` name/SKU), category/price/low-stock filters, sort, pagination, CSV export. `is_archived` soft-delete, `low_stock_threshold` per product.

---

## Categories

| Active / Inactive |
|---|
| ![Categories Inactive Badge](../demo-10-categories-inactive-badge.png) |

**What it shows:** Category list with active/inactive badges (`is_active`), product counts, deactivation guard (409 if products still reference it).

---

## Inventory

| Stock Levels | Overdraw Guard |
|---|---|
| ![Inventory](../demo-13-inventory.png) | ![Inventory Overdraw](../demo-16-inventory-overdraw.png) |

**What it shows:** Per-warehouse stock (`UNIQUE product_id+warehouse_id`), `quantity` + `reserved_quantity` + `version`, low-stock badges. Overdraw attempt → `409 INSUFFICIENT_STOCK` with no partial ledger row (atomic `FOR UPDATE`).

---

## Transactions & Reports

| Transactions | Reports |
|---|---|
| ![Transactions](../demo-14-transactions.png) | ![Reports](../demo-15-reports.png) |

**What it shows:** Append-only `inventory_ledger` history (`RECEIVE/ISSUE/TRANSFER_IN/TRANSFER_OUT/ADJUSTMENT`), `transfer_id` pairing, and `GET /reports` stock summary + CSV.

---

## Users & Activity

| Users | Activity Log | Settings |
|---|---|---|
| ![Users](../demo-17-users.png) | ![Activity](../demo-18-activity.png) | ![Settings](../demo-19-settings.png) |

**What it shows:** Admin user management (role `ADMIN`/`STAFF`, `is_active`), paginated audit log (`GET /activity-logs` — `action`, `entity_type`, `before/after`, `IP`), and settings/profile (`change-password`, `DEMO_MODE`).

---

## Responsive

| Mobile | Tablet | Desktop |
|---|---|---|
| ![Responsive Mobile](../demo-20-responsive-mobile.png) | ![Responsive Tablet](../demo-21-responsive-tablet.png) | ![Responsive Desktop](../demo-22-responsive-desktop.png) |

**What it shows:** Tailwind 4 responsive shell — `app-layout` + `command-palette` (Radix dialog/dropdown), same 12 pages adapt from 320px to 1440px.

---

## How to run

```bash
cd web
npm install
npm run dev      # Vite on http://localhost:5173
npm run build    # type-check + production bundle
npm run preview  # serve dist/
```

Backend must be running (`make docker-up` + `make seed` — API on `:8080`, Postgres `postgres:17-alpine`). See `README.md:Getting Started` and `docs/deployment.md`.

---

## Gallery — All 18 Screenshots

| # | File | Screen |
|---|---|---|
| 05 | `demo-05-dashboard-live.png` | Dashboard — live KPIs |
| 06 | `demo-06-dashboard-dark.png` | Dashboard — dark mode |
| 07 | `demo-07-dashboard-mobile.png` | Dashboard — mobile |
| 08 | `demo-08-dashboard-desktop.png` | Dashboard — desktop |
| 09 | `demo-09-products-list.png` | Products — list |
| 10 | `demo-10-categories-inactive-badge.png` | Categories — inactive badge |
| 11 | `demo-11-products-archived-view.png` | Products — archived |
| 12 | `demo-12-products-sort-export.png` | Products — sort + export |
| 13 | `demo-13-inventory.png` | Inventory — stock levels |
| 14 | `demo-14-transactions.png` | Transactions — ledger |
| 15 | `demo-15-reports.png` | Reports — summary |
| 16 | `demo-16-inventory-overdraw.png` | Inventory — overdraw guard |
| 17 | `demo-17-users.png` | Users — admin list |
| 18 | `demo-18-activity.png` | Activity — audit log |
| 19 | `demo-19-settings.png` | Settings — profile |
| 20 | `demo-20-responsive-mobile.png` | Responsive — mobile |
| 21 | `demo-21-responsive-tablet.png` | Responsive — tablet |
| 22 | `demo-22-responsive-desktop.png` | Responsive — desktop |

> Tip: open any `demo-*.png` at repo root for full-resolution.
