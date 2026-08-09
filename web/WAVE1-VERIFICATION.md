# Wave 1 Verification — Foundation + Auth + Dashboard

**Status: PASSED** (all live checks via Playwright against running app + backend)

## Build
- `npm run build` green (tsc + vite, ~4.4s). Only warning: chunk-size (expected, code-splitting planned for later waves).

## Live functional checks
| Check | Result |
|---|---|
| Login page renders, 0 console errors | ✅ |
| Admin login (`admin@inventory.local` / `Admin123!`) → dashboard | ✅ |
| Dashboard live data | ✅ 23 products, $222,380.00, 5 categories, 1 low-stock, activity list |
| Ctrl+K command palette open / search / navigate | ✅ (navigated to /products) |
| Logout → /login | ✅ |
| Demo login (`POST /auth/demo`) → STAFF, reduced nav | ✅ |
| STAFF hitting /users → redirected to / (RBAC) | ✅ |
| Authenticated hitting /login → redirected to / (RequireGuest) | ✅ |
| Dark theme toggle → `.dark` class + persisted to localStorage | ✅ |
| Sidebar collapse 240→64px and restore | ✅ |
| Register flow → creates account + auto-login (STAFF) | ✅ |
| Empty-form Zod validation errors render | ✅ |
| Wrong password → `unauthorized` alert (401 expected resource error only) | ✅ |
| Token refresh on 401 | implemented (deduped refresh-once, `inventra:unauthorized` event) |

## Responsive (design.md: desktop ≥1280 / tablet 768–1279 drawer / mobile <768)
| Viewport | Fixed sidebar | Hamburger | Toggle | Result |
|---|---|---|---|---|
| 1440×900 | ✅ 240px | hidden | visible | ✅ |
| 1024×900 | hidden | visible | hidden | ✅ drawer pattern |
| 480×860 | hidden | visible | hidden | ✅ |
| Drawer opens with all 9 nav links + overlay | — | — | — | ✅ |

## Screenshots (repo root)
- `demo-05-dashboard-live.png` — light, full page
- `demo-06-dashboard-dark.png` — dark
- `demo-07-dashboard-mobile.png` — 480px mobile
- `demo-08-dashboard-desktop.png` — 1440px desktop

## Notes
- Breakpoint bug found & fixed during verification: layout used `lg:` (1024px) for the fixed sidebar; changed to `xl:` (1280px) per design.md. Verified at 1024 (drawer) and 1440 (fixed sidebar).
- Only console error across all checks: expected 401 resource-load from the deliberate wrong-password test.
- Vite dev on :5173 (PID 529059), backend :8090.
