# Security Design

**Document Version:** 1.0
**Phase:** 1 — Foundation
**Date:** 2026-08-06
**Status:** SPEC-FIRST — governs all authentication/authorization implementation

---

## 1. Authentication

### 1.1 Password hashing

- **Algorithm:** bcrypt via `golang.org/x/crypto/bcrypt`.
- **Cost:** `12` (locked by config `BCRYPT_COST`, default `12`).
- The plaintext password is never stored; only the bcrypt hash with baked-in salt.
- `bcrypt.CompareHashAndPassword` for verification; constant-time behavior built-in.

### 1.2 Token strategy (JWT access + rotating refresh)

| Property | Access token | Refresh token |
|---|---|---|
| Format | JWT (HS256) | 40-byte random, base64-encoded |
| TTL | `15m` (JWT_ACCESS_TTL) | `168h` / 7 days (JWT_REFRESH_TTL) |
| Storage | client (in-memory / Authorization header) | DB: `refresh_tokens` (SHA-256 hash only) |
| Rotation | – | Every use: revoke presented, issue new |

- **Access token claims:** `sub` (user id), `role`, `exp` (15m), `iat`, `iss` (`inventory-api`), `aud` (`inventory`).
- **Refresh token lifecycle:** on login a refresh token is generated, its SHA-256 hash stored in
  `refresh_tokens` (columns: `token_hash` unique, `user_id` FK, `expires_at`, `revoked_at`, `created_at`).
  On `/auth/refresh` the presented token is matched by hash, must be unexpired + unrevoked, then rotated.
  On `/auth/logout` the refresh token is revoked.
- **JWT secret:** `JWT_SECRET` config is **required** at load time; `config.Load()` returns
  `MissingRequiredError` if absent. Production uses a strong random secret via env var.

### 1.3 Token verification flow

1. Client sends `Authorization: Bearer <access_token>`.
2. JWT middleware verifies signature (HS256, `JWT_SECRET`), exp, claims.
3. On success, sets `user_id` and `role` in Gin context.
4. Route-level `RoleRequired(role)` middleware gates admin-only endpoints.

### 1.4 Demo auto-login (DEMO_MODE) — security notes

- `DEMO_MODE=true` exposes `POST /api/v1/auth/demo`, which returns a real token pair
  for a `STAFF` demo user (`demo@inventory.local`) **without any password**.
- **It is a passwordless identities-the-user door.** Anyone who can reach the endpoint can
  authenticate as the demo STAFF account. It is **development/demo only** — it must never be
  enabled in a shared or production environment.
- The demo account holds `STAFF`, not `ADMIN`, so the blast radius is limited to read
  endpoints and stock movements — no user management, no audit-log read.
- The demo password hash is a fresh random bcrypt value; the account has **no known credential**,
  so normal `POST /auth/login` cannot be used against it.
- Every demo login still writes an activity log entry (`action=LOGIN`) for auditability.
- Guardrail: the route is only registered (reachable) while `DEMO_MODE=true`; the flag defaults `false`.

---

## 2. Authorization (RBAC)

- **Roles:** `ADMIN`, `STAFF` (enforced by `roles.name` check constraint and DTO validation).
- **Matrix:** see `docs/api.md` §2. Every route declares its guard (public / any-auth / STAFF / ADMIN).
- **Enforcement:** centralized JWT middleware resolves role; `RoleRequired(ROLE)` returns
  `403 Forbidden` when the caller's role is insufficient.
- Default registered users receive `STAFF`; only ADMIN can assign roles (`PUT /users/:id/role`).

---

## 3. Input Validation & Injection Safety

- **Validation:** all request DTOs validated via the shared `validator` wrapper
  (go-playground/validator/v10). Invalid input → `400 ErrValidation`.
- **SQL injection:** all DB access goes through GORM parameterized queries
  (`.Where("email = ?", email)`); never string-concatenate user input into SQL.
- **Output/escape:** JSON serialization only; front-end escapes rendered content.

---

## 4. Secure Headers

Applied by `SecureHeaders()` middleware on every response:

| Header | Value |
|---|---|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Content-Security-Policy` | `default-src 'self'` |
| `Referrer-Policy` | `no-referrer` |
| `Strict-Transport-Security` (prod) | `max-age=63072000` (HSTS, when `APP_ENV=production`) |

---

## 5. CORS

- **Allowlist:** configured `CORS_ORIGINS` (comma-separated). Default fallback: `http://localhost:5173`.
- **Allowed methods:** `GET, POST, PUT, PATCH, DELETE, OPTIONS`.
- **Allowed headers:** `Origin, Content-Type, Authorization`.
- `AllowCredentials: true`; `MaxAge` `12h`.
- CSRF risk mitigated by bearer-token + CORS allowlist design (no auth cookies).

---

## 6. Rate Limiting

- In-memory **per-IP token bucket** middleware at the middleware layer
  (`internal/shared/middleware/ratelimit.go`) applied to the public auth
  endpoints `/auth/login`, `/auth/refresh`, `/auth/register`, and `/auth/demo`
  to mitigate brute-force and token stuffing.
- Default budgets (requests/minute per IP): login **10**, refresh **30**,
  register **5**, demo **5** — configurable via `LOGIN_RATE_LIMIT_RPM`,
  `REFRESH_RATE_LIMIT_RPM`, `REGISTER_RATE_LIMIT_RPM`, `DEMO_RATE_LIMIT_RPM`.
- Exceeded budget returns `429 Too Many Requests` with a `Retry-After: 60`
  header; buckets idle longer than 1 minute are evicted to bound memory.
- No external store is used (single-instance modular monolith; Redis deferred
  until multi-instance deployment is required).

---

## 7. Activity Logging / Audit Trails

- Audit-relevant actions (`LOGIN`, `REGISTER`, `LOGOUT`, `CHANGE_PASSWORD`, CRUD, stock in/out)
  write to `activity_logs` via `internal/activitylog` (fire-and-forget; logging failure must not
  fail the business operation).
- Contains `action`, `entity_type`, `entity_id`, `details` (JSONB), `ip`, `user_id`, `created_at`.

---

## 8. Threat model summary

| Threat | Mitigation |
|---|---|
| Credential brute force | bcrypt cost 12 + rate limiting on auth endpoints |
| Token theft / replay | short access TTL (15m); refresh rotation; refresh stored hashed in DB |
| DB dump exposes passwords | only bcrypt hashes stored |
| DB dump exposes refresh tokens | only SHA-256 hashes of refresh tokens stored |
| Stolen refresh token reuse | revoke on use (rotation) makes replay detectable |
| Forged/upgraded JWT | HS256 signature verified against required `JWT_SECRET` |
| SQL injection | GORM parameterized queries only |
| XSS / clickjacking | CSP + X-Frame-Options DENY + JSON-only responses |
| CSRF | bearer-token auth (no cookies), CORS allowlist |
| Privilege escalation | RBAC middleware + role claim; STAFF cannot reach ADMIN routes |

---

## 9. Config reference (locked)

| Config key | Default | Notes |
|---|---|---|
| `JWT_ACCESS_TTL` | `15m` | access token TTL |
| `JWT_REFRESH_TTL` | `168h` | refresh token TTL (7 days) |
| `BCRYPT_COST` | `12` | bcrypt work factor |
| `LOW_STOCK_THRESHOLD` | `10` | product low-stock threshold default |
| `JWT_SECRET` | *(required)* | HS256 signing secret; load fails if missing |
| `CORS_ORIGINS` | `http://localhost:5173` | comma-separated allowlist |
| `PORT` | `8080` | HTTP listen port |
| `DB_SSLMODE` | `disable` | dev default |