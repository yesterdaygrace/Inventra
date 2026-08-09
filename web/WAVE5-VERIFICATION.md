# Wave 5 Verification — Full regression + polish

Per `.omo/plans/frontend-waves.md` Wave 5: end-to-end UI test of all
features; responsive QA at 1280 / 768 / <768; keyboard nav, WCAG AA spot
check, toast timing, animation budget ≤200ms. All checks below were run live
against `:5173` (Vite) + `:8090` (demo backend) via Playwright.

## 1. Full E2E regression (admin)
All nine pages render with their real data and **no console errors** (only
the expected 401 refresh noise filtered out):

| Page | Heading | Content |
|---|---|---|
| `/` | Dashboard | 10 cards (KPIs/charts), no table |
| `/products` | Products | catalog table, filters, sort, pagination |
| `/categories` | Categories | categories table with active/product_count |
| `/inventory` | Inventory | stock-level table, search, low-stock, export |
| `/transactions` | Transactions | movement history, product/type filters |
| `/reports` | Reports | KPI cards + category table + low-stock table |
| `/users` | Users | user table, role/status filters |
| `/activity` | Activity Log | audit events + action/entity filters |
| `/settings` | Settings | profile + change-password cards |

Functional interactions already verified in Waves 2–4 (archive/deactivate,
stock in/out incl. 409, exports, sort, role change, profile save) remain
green on this pass.

## 2. RBAC (STAFF regression)
- Sidebar: **Users + Activity Log hidden** for STAFF; Settings present.
- Products/Categories: no Add buttons (admin-only writes); Export present.
- Inventory: Stock in/out **visible** (STAFF is an operator).
- Direct URL to `/users` as STAFF → **redirected to /** (frontend guard).

## 3. Responsive QA (no horizontal overflow on any page)
Checked every shared page at 480 / 768 / 1280: `scrollWidth ≤ innerWidth` on
**all** pages — no horizontal scroll anywhere.

## 4. Keyboard nav + WCAG AA spot check (products page)
- 23 tab stops in logical order (nav → user menu → palette → content →
  filters → pagination).
- **Every stop has an accessible name** (aria-label/placeholder/text) —
  `noNameCount: 0`.
- **Every stop has a visible focus ring** (`focus-visible` box-shadow) —
  `withRing: 23/23`.
- Form inputs use `<Label htmlFor>` associations; decorative icons use
  `aria-hidden` (shadcn defaults).

## 5. Toast timing + animation budget
- Toast auto-dismiss: `TOAST_REMOVE_DELAY = 4000` ms in
  `src/hooks/use-toast.ts` — matches the 4s target.
- Max CSS transition duration measured on page: **200 ms**; max keyframe
  animation: **0 ms** → within the ≤200 ms budget.

## Evidence
- `demo-20-responsive-mobile.png` (dashboard @ 480)
- `demo-21-responsive-tablet.png` (products @ 768)
- `demo-22-responsive-desktop.png` (inventory @ 1440)

## Verdict
Wave 5 checks **PASS**: all features demoable end-to-end through the UI, RBAC
enforced at both route and nav level, responsive at all three breakpoints,
keyboard accessible with visible focus, 4 s toasts, and animations within
budget. The Inventra frontend is complete per the wave plan.
