# API Route Contract

**Document Version:** 1.0
**Phase:** 1 — Foundation
**Date:** 2026-08-06
**Status:** SPEC-FIRST — authoritative contract for W3–W9 handler implementation

---

## 1. Base Conventions

- **Base path:** `/api/v1` for all module routes. Healthcheck: `GET /healthz`.
- **Response envelope (always):**
  ```json
  {
    "success": true,
    "message": "optional",
    "data": {},
    "pagination": { "page": 1, "per_page": 20, "total": 0, "total_pages": 0 }
  }
  ```
  - `data` is the payload; `pagination` present only on paginated list responses.
  - Error responses: `{ "success": false, "message": "<reason>" }` (no `data`, no `pagination`).
- **Content types:** requests and responses `application/json`.
- **Request ID:** every response echoes `X-Request-ID`; propagated through logging.
- **Auth:** protected routes require `Authorization: Bearer <access_token>`.

### 1.1 Error codes and HTTP mapping

| Typed error | HTTP status | Example message |
|---|---|---|
| `ErrValidation` | `400 Bad Request` | `validation failed: <details>` |
| `ErrUnauthorized` | `401 Unauthorized` | `unauthorized` |
| `ErrForbidden` | `403 Forbidden` | `forbidden` |
| `ErrNotFound` | `404 Not Found` | `not found` |
| `ErrConflict` | `409 Conflict` | `conflict` |
| `ErrInternal` (fallback) | `500 Internal Server Error` | `internal server error` |

All DTO validation uses the shared `validator` wrapper (go-playground/validator/v10).

### 1.2 Pagination

Query parameters on list endpoints: `?page=<int>&per_page=<int>` (defaults `page=1`, `per_page=20`).
Response envelope carries `pagination` object: `page`, `per_page`, `total`, `total_pages`.

---

## 2. RBAC Matrix

| Route | Public | Any authenticated | STAFF | ADMIN |
|---|---|---|---|---|
| `POST /auth/register` | ✔ | – | – | – |
| `POST /auth/login` | ✔ | – | – | – |
| `POST /auth/refresh` | ✔ (refresh token) | – | – | – |
| `POST /auth/demo` | ✔ (demo mode only) | – | – | – |
| `POST /auth/logout` | – | ✔ | ✔ | ✔ |
| `POST /auth/change-password` | – | ✔ | ✔ | ✔ |
| `PUT /auth/profile` | – | ✔ | ✔ | ✔ |
| `GET /auth/me` | – | ✔ | ✔ | ✔ |
| `GET /users` | – | – | – | ✔ |
| `GET /users/:id` | – | – | – | ✔ |
| `PUT /users/:id` | – | – | – | ✔ |
| `DELETE /users/:id` | – | – | – | ✔ |
| `PUT /users/:id/role` | – | – | – | ✔ |
| `GET /products` | ✔ | ✔ | ✔ | ✔ |
| `POST /products` | – | – | – | ✔ |
| `GET /products/:id` | ✔ | ✔ | ✔ | ✔ |
| `PUT /products/:id` | – | – | – | ✔ |
| `DELETE /products/:id` | – | – | – | ✔ |
| `GET /categories` | ✔ | ✔ | ✔ | ✔ |
| `POST /categories` | – | – | – | ✔ |
| `PUT /categories/:id` | – | – | – | ✔ |
| `DELETE /categories/:id` | – | – | – | ✔ |
| `GET /inventory` | – | ✔ | ✔ | ✔ |
| `POST /inventory/stock-in` | – | – | ✔ | ✔ |
| `POST /inventory/stock-out` | – | – | ✔ | ✔ |
| `GET /inventory/transactions` | – | ✔ | ✔ | ✔ |
| `GET /inventory/export` | – | ✔ | ✔ | ✔ |
| `GET /dashboard/summary` | – | ✔ | ✔ | ✔ |
| `GET /dashboard/activity` | – | ✔ | ✔ | ✔ |

Auth guard = presence of a valid bearer access token; role guard enforced via
`RoleRequired(ROLE)` helper middleware. Valid token with insufficient role → `403 Forbidden`.

---

## 3. Auth Module (W3)

Access token TTL `15m`; refresh TTL `168h` (7d); refresh token rotated on every use.

### POST `/api/v1/auth/register` — public
- **Request DTO:** `name` (required, min 2), `email` (required, email, unique), `password` (required, min 8).
  ```json
  { "name": "…", "email": "…", "password": "…" }
  ```
- **Response 201 (envelope.data):** `{ "id", "name", "email", "role": "STAFF", "is_active": true, "created_at", "updated_at" }`.
- **Errors:** 400 validation; 409 duplicate email.

### POST `/api/v1/auth/login` — public
- **Request:** `{ "email": "…", "password": "…" }`
  ```json
  {
    "access_token": "<jwt 15m>",
    "refresh_token": "<refresh token>",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": { "id": "…", "name": "…", "email": "…", "role": "ADMIN", "is_active": true }
  }
  ```
- **Errors:** 401 wrong credentials / inactive user; 400 validation.
- Side effects: refresh token row inserted; activity log entry (`action=LOGIN`).

### POST `/api/v1/auth/demo` — public (only when `DEMO_MODE=true`)
- **Request:** none (no JSON body).
- **Response 200 (data):** `{ "access_token", "refresh_token", "token_type": "Bearer", "expires_in": 900, "user": { "id": "…", "name": "Demo User", "email": "demo@inventory.local", "role": "STAFF", "is_active": true } }`.
- **Behaviour:** returns tokens for a `STAFF` demo user (`demo@inventory.local`), creating the account on first use. No password required.
- **Route absence:** when demo mode is off the route is not registered → `404`.
- Side effects: refresh token row inserted; activity log entry (`action=LOGIN`).

### POST `/api/v1/auth/refresh` — public (presents refresh token)
- **Request:** `{ "refresh_token": "<refresh>" }`
- **Response 200 (data):** `{ "access_token", "refresh_token", "token_type": "Bearer", "expires_in": 900 }`.
- **Rotation:** revokes presented refresh token, issues new access + new refresh.
- **Errors:** 400 validation; 401 invalid/revoked/expired refresh token.

### POST `/api/v1/auth/logout` — any authenticated
- **Request:** `{ "refresh_token": "<refresh>" }`
- **Response 200:** `{ "success": true, "message": "logged out" }`.
- **Errors:** 400 validation; 401 missing/invalid token.

### POST `/api/v1/auth/change-password` — any authenticated
- **Request:** `{ "old_password": "…", "new_password": "…" }` (both required; new min 8)
- **Response 200:** `{ "success": true, "message": "password changed" }`.
- **Errors:** 400 validation; 401 wrong `old_password`.

### PUT `/api/v1/auth/profile` — any authenticated (own profile)
- **Request:** `{ "name": "…", "email": "…", "old_password": "…" }` (`old_password` required to change `email`)
- **Response 200:** updated user object.
- **Errors:** 400 validation; 401 unauthorized; 409 duplicate email.

### GET `/api/v1/auth/me` — any authenticated
- **Response 200 (envelope):**
  ```json
  { "id": "…", "name": "…", "email": "…", "role": "ADMIN", "is_active": true, "created_at": "…", "updated_at": "…" }
  ```
- **Errors:** 401 missing/invalid token.

---

## 4. User Module (W4) — admin only

### GET `/api/v1/users` — ADMIN
- **Query:** `page`, `per_page`, `name=<substr>`, `email=<substr>`, `role=ADMIN|STAFF`, `is_active=true|false`.
- **Response 200 (paginated):** `data: [ { id, name, email, role, is_active, created_at, updated_at } ]` + `pagination`.
- **Errors:** 401 / 403.

### GET `/api/v1/users/:id` — ADMIN
- **Response 200:** user object.
- **Errors:** 401 / 403; 404 not found.

### PUT `/api/v1/users/:id` — ADMIN
- **Request:** `{ "name": "…", "email": "…", "is_active": true|false }`
- **Response 200:** updated user.
- **Errors:** 400 validation; 401/403; 404; 409 self-deactivation / last-admin-deactivation forbidden.

### DELETE `/api/v1/users/:id` — ADMIN (soft-deactivate; no hard delete)
- **Response 200:** `{ "success": true, "message": "user deactivated" }`.
- **Errors:** 401/403; 404; 409 cannot deactivate self / last active ADMIN.

### PUT `/api/v1/users/:id/role` — ADMIN
- **Request:** `{ "role": "ADMIN"|"STAFF" }`
- **Response 200:** updated user.
- **Errors:** 400 validation (invalid role); 401/403; 404.

---

## 5. Product Module (W6)

Product: SKU unique, price numeric(12,2), CategoryID FK, LowStockThreshold default 10, IsArchived soft archive.

### GET `/api/v1/products` — public read
- **Query:** `page`, `per_page`, `search` (name/SKU substr), `category_id`, `low_stock=true` (IsArchived=false AND quantity ≤ low_stock_threshold), `is_archived=true|false`.
- **Response 200 (paginated):** `data: [ { id, name, sku, description, price, category_id, category_name, low_stock_threshold, is_archived, stock_quantity, is_low_stock, created_at, updated_at } ]` + `pagination`.

### POST `/api/v1/products` — ADMIN
- **Request:** `{ "name": "*", "sku": "*", "description": "", "price": "* (>=0)", "category_id": "*", "low_stock_threshold": ">=0 default 10" }`
- **Response 201:** full product object.
- **Errors:** 400 validation; 404 category not found; 409 duplicate sku.

### GET `/api/v1/products/:id` — public read
- **Response 200:** product incl. `category_name`, `stock_quantity`, `is_low_stock`.
- **Errors:** 404.

### PUT `/api/v1/products/:id` — ADMIN
- **Request:** any subset of product fields.
- **Response 200:** updated product.
- **Errors:** 400 validation; 404; 409 duplicate sku; 404 category.

### DELETE `/api/v1/products/:id` — ADMIN (soft archive `IsArchived=true`)
- **Response 200:** `{ "success": true, "message": "product archived" }`.
- **Errors:** 404; 409 conflict (FK/in-use protection).

---

## 6. Category Module (W6)

Category: Name unique; deactivate semantics via `IsActive=false` (no hard delete for referenced categories).

### GET `/api/v1/categories` — public read
- **Query:** `page`, `per_page`, `name` (substr), `is_active=true|false`.
- **Response 200 (paginated):** `data: [ { id, name, description, is_active, product_count, created_at, updated_at } ]` + `pagination`.

### POST `/api/v1/categories` — ADMIN
- **Request:** `{ "name": "*", "description": "" }`
- **Response 201:** category object.
- **Errors:** 400 validation; 409 duplicate name.

### PUT `/api/v1/categories/:id` — ADMIN
- **Request:** any subset of `{ name, description, is_active }`.
- **Response 200:** updated category.
- **Errors:** 400 validation; 404; 409 duplicate name.

### DELETE `/api/v1/categories/:id` — ADMIN
- **Response 200:** `{ "success": true, "message": "category deactivated" }` (sets `IsActive=false`).
- **Errors:** 404; 409 category has active products (must archive products first).

---

## 7. Inventory Module (W6/W7)

Inventory: 1:1 with product (ProductID unique); transactions typed `IN`/`OUT`; Quantity > 0 on transactions.

### GET `/api/v1/inventory` — any authenticated
- **Query:** `page`, `per_page`, `product_id`, `low_stock=true`, `search` (product name/SKU).
- **Response 200 (paginated):** `data: [ { product_id, product_sku, product_name, quantity, updated_at } ]` + `pagination`.
  Every product is returned (left-joined), with quantity `0` when no stock row exists yet.

### POST `/api/v1/inventory/stock-in` — STAFF / ADMIN
- **Request:** `{ "product_id": "*", "quantity": ">0 (required)", "unit_cost": ">=0 optional", "note": "optional" }`
- **Response 200:** `{ product_id, quantity, updated_at }`. Committed atomically with the history row.

### POST `/api/v1/inventory/stock-out` — STAFF / ADMIN
- **Request:** `{ "product_id": "*", "quantity": ">0 (required)", "unit_cost": ">=0 optional", "note": "optional" }`
- **Response 200:** `{ product_id, quantity, updated_at }`.
- **Errors:** 400 validation; 409 insufficient stock (quantity would go below 0), rolled back with no partial history row.

### GET `/api/v1/inventory/transactions` — any authenticated
- **Query:** `page`, `per_page`, `product_id`, `type=IN|OUT`.
- **Response 200 (paginated):** `data: [ { id, product_id, product_sku, product_name, type, quantity, unit_cost, note, user_id, created_at } ]` + `pagination`.

### GET `/api/v1/inventory/export` — any authenticated
- **Response 200:** CSV download (`attachment; filename=inventory_<ts>.csv`) with columns `product_id,sku,name,quantity,updated_at`.

---

## 8. Dashboard Module (W9)

### GET `/api/v1/dashboard/summary` — any authenticated
- **Response 200 (data):**
  ```json
  {
    "total_products": 0,
    "total_categories": 0,
    "total_stock_value": 0.0,
    "low_stock_count": 0,
    "out_of_stock_count": 0,
    "recent_transactions": [ { "id", "product_name", "type", "quantity", "created_at" } ]
  }
  ```

### GET `/api/v1/dashboard/activity` — any authenticated
- **Query:** `page`, `per_page`.
- **Response 200 (paginated):** `data: [ { id, user_name, action, entity_type, entity_id, details, ip, created_at } ]` (from `activity_logs`) + `pagination`.

---

## 9. Representative payloads

### Login
```
POST /api/v1/auth/login
{ "email": "admin@inventory.local", "password": "Admin123!" }
→ 200
{
  "success": true,
  "data": {
    "access_token": "eyJ…",
    "refresh_token": "9f2a…",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": { "id": "…", "name": "System Administrator", "email": "admin@inventory.local", "role": "ADMIN", "is_active": true }
  }
}
```

### Products list
```
GET /api/v1/products?page=1&per_page=20&low_stock=true
→ 200
{
  "success": true,
  "data": [ { "id": "…", "name": "Wireless Mouse", "sku": "ELEC-001", "price": 24.99, "category_id": "…", "category_name": "Electronics", "stock_quantity": 100, "is_low_stock": false } ],
  "pagination": { "page": 1, "per_page": 20, "total": 1, "total_pages": 1 }
}
```

### Create product
```
POST /api/v1/products   (Authorization: Bearer <admin>)
{ "name": "Laptop", "sku": "LAP-001", "price": 999.99, "category_id": "…", "low_stock_threshold": 5 }
→ 201
{ "success": true, "data": { "id": "…", "name": "Laptop", "sku": "LAP-001", "price": 999.99, "category_id": "…", "category_name": "Electronics", "low_stock_threshold": 5, "is_archived": false, "stock_quantity": 0 } }
```

### Stock in
```
POST /api/v1/inventory/transactions  (Authorization: Bearer <staff>)
{ "product_id": "…", "type": "IN", "quantity": 50, "unit_cost": 80.0, "note": "restock" }
→ 201
{ "success": true, "data": { "id": "…", "product_id": "…", "type": "IN", "quantity": 50, "unit_cost": 80.0, "note": "restock", "created_at": "…" } }
```

### Error example
```
POST /api/v1/auth/login  (bad password)
→ 401
{ "success": false, "message": "unauthorized" }
```
