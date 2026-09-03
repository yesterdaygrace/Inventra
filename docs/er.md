# Entity-Relationship Diagram

**Document Version:** 1.1 — fix-v2-gaps
**Date:** 2026-09-01
**Status:** authoritative — matches `migrations/000001…000013` and `internal/*/model.go`.
**Detail companion:** `docs/database.md` (table specs, `CHECK`/`FK`/`UNIQUE`, GORM tags) and `docs/database.md:Appendix A` (transactions, locking, validation, audit evidence).

> Visual first. Textual specs live in `database.md` — this file is the map, not the dictionary.

## Mermaid

```mermaid
erDiagram
    roles {
        uuid id PK
        text name UK "ADMIN"
        timestamptz created_at
    }
    permissions {
        uuid id PK
        text code UK
        text name
    }
    role_permissions {
        uuid role_id PK_FK
        uuid permission_id PK_FK
    }
    users {
        uuid id PK
        text name
        text email UK
        text password_hash
        uuid role_id FK
        boolean is_active
        timestamptz created_at
        timestamptz updated_at
    }
    refresh_tokens {
        uuid id PK
        uuid user_id FK
        text token_hash UK
        uuid family_id
        timestamptz expires_at
        timestamptz revoked_at
        timestamptz created_at
    }
    categories {
        uuid id PK
        text name UK
        text description
        boolean is_active
        timestamptz created_at
        timestamptz updated_at
    }
    warehouses {
        uuid id PK
        text code UK "DEFAULT"
        text name
        text description
        boolean is_active
        timestamptz created_at
        timestamptz updated_at
    }
    products {
        uuid id PK
        text name
        text sku UK
        text description
        numeric price "12p2"
        uuid category_id FK
        int low_stock_threshold "gte0"
        boolean is_archived
        timestamptz created_at
        timestamptz updated_at
    }
    inventory {
        uuid id PK
        uuid product_id FK
        uuid warehouse_id FK
        int quantity "gte0"
        int reserved_quantity "gte0"
        int version
        timestamptz updated_at
        string UK "product-warehouse"
    }
    inventory_ledger {
        uuid id PK
        uuid product_id FK
        uuid warehouse_id FK
        text transaction_type "7types"
        text direction "IN-OUT"
        int quantity "gt0"
        numeric unit_cost
        uuid transfer_id "transfer"
        uuid performed_by FK
        timestamptz created_at
    }
    inventory_reservations {
        uuid id PK
        uuid product_id FK
        uuid warehouse_id FK
        int quantity "gt0"
        text status "ACTIVE"
        timestamptz expires_at
        timestamptz created_at
    }
    inventory_adjustments {
        uuid id PK
        uuid product_id FK
        uuid warehouse_id FK
        int system_quantity
        int counted_quantity
        text status "PENDING"
        uuid requested_by FK
        timestamptz created_at
    }
    cycle_count_plans {
        uuid id PK
        uuid warehouse_id FK
        text name
        text status "OPEN"
        timestamptz created_at
    }
    cycle_count_items {
        uuid id PK
        uuid plan_id FK
        uuid product_id FK
        int system_quantity
        int counted_quantity
        timestamptz counted_at
    }
    activity_logs {
        uuid id PK
        uuid user_id FK
        text action
        text entity_type
        jsonb details
        jsonb before_data
        text ip
        timestamptz created_at
    }
    idempotency_keys {
        uuid id PK
        text key_hash UK
        uuid user_id FK
        text endpoint
        timestamptz expires_at "24h"
    }
    system_settings {
        text key PK
        text value
        timestamptz updated_at
    }

    roles ||--o{ users : has
    roles ||--o{ role_permissions : grants
    permissions ||--o{ role_permissions : granted_to
    users ||--o{ refresh_tokens : owns
    users ||--o{ activity_logs : performs
    users ||--o{ inventory_ledger : performs
    users ||--o{ inventory_adjustments : requests
    categories ||--o{ products : contains
    warehouses ||--o{ inventory : stocks
    warehouses ||--o{ inventory_ledger : locates
    warehouses ||--o{ inventory_reservations : reserves_in
    warehouses ||--o{ cycle_count_plans : counts_in
    products ||--o{ inventory : has_per_warehouse
    products ||--o{ inventory_ledger : moves
    products ||--o{ inventory_reservations : reserved
    products ||--o{ inventory_adjustments : adjusted
    products ||--o{ cycle_count_items : counted
    cycle_count_plans ||--o{ cycle_count_items : contains
    inventory_adjustments }o--|| cycle_count_items : may_create
```

## Relationships (verbal)

- **roles → users** — one role has many users; `roles.name` is `CHECK IN ('ADMIN','WAREHOUSE_MANAGER','STAFF','VIEWER')` (`000008`).
- **roles ↔ permissions** — many-to-many via `role_permissions`; catalog in `auth.PermissionSetForRole` mirrored by seed.
- **users → refresh_tokens** — one user has many families; `token_hash` unique, `family_id` groups rotations for reuse detection (`000004`).
- **users → activity_logs / inventory_ledger** — `user_id` nullable (system/seed) vs `performed_by` nullable.
- **categories → products** — `products.category_id` FK, cannot delete a category with active products (409).
- **warehouses → inventory / ledger / reservations / cycle plans** — per-warehouse scoping; `DEFAULT` warehouse seeded as fallback.
- **products → inventory** — **one row per `(product_id, warehouse_id)`** via composite unique `idx_inventory_product_warehouse` (`000001:68`).
- **products → inventory_ledger** — append-only movements; `transfer_id` non-null groups exactly two rows (`TRANSFER_OUT` + `TRANSFER_IN`).
- **inventory ↔ inventory_reservations** — `available = inventory.quantity − SUM(active)`; reservations lazily expire inside `FOR UPDATE` lock.
- **inventory → inventory_adjustments / cycle_count_items** — `system_quantity` snapshots `inventory.quantity` at count time; variances file an adjustment.
- **cycle_count_plans → cycle_count_items** — one plan has many items; `counted_quantity` nullable until counted.
- **idempotency_keys** — `key_hash = SHA256(user+route+header)` unique; scoped per user+route, TTL 24h (`000003`).

## Constraints (summary — full inventory in `database.md:Appendix A3`)

- `inventory.quantity >= 0`, `reserved_quantity >= 0`, `low_stock_threshold >= 0` (`000001:66`, `000002:10`)
- `inventory_ledger.quantity > 0`, `transaction_type IN (7)`, `direction IN ('IN','OUT')` (`000013:12`)
- `inventory_reservations.quantity > 0`, `status IN ('ACTIVE','RELEASED','CONSUMED','EXPIRED')` (`000010`)
- `inventory_adjustments.counted_quantity >= 0`, `status IN ('PENDING','APPROVED','REJECTED')`
- `cycle_count_plans.status IN ('OPEN','COMPLETED')`, `system_quantity >= 0`, `counted_quantity >= 0` nullable (`000012:11`)
- `UNIQUE`: `users.email`, `products.sku`, `categories.name`, `warehouses.code`, `refresh_tokens.token_hash`, `idempotency_keys.key_hash`, `(inventory product_id+warehouse_id)`
- `FK`: all `*_id` columns reference their parent `id` with `REFERENCES`; `ON DELETE` is restrictive — deactivate, never hard-delete in-use rows.

## Notes on integrity

- **Ledger is append-only** — never `UPDATE`/`DELETE`; corrections are new `ADJUSTMENT` rows; balance is derived on read via window function (`SUM(CASE WHEN direction='OUT' THEN -quantity ELSE quantity END) OVER (PARTITION BY product_id, warehouse_id)`).
- **Migrations are versioned** (`000001_init` → `000013_ledger_direction`) via `golang-migrate`; `make migrate-up` is production path, `DB_AUTOMIGRATE` is dev-only. See `database.md:Schema Management`.
- **Indexes:** `idx_inventory_product_warehouse`, `idx_inventory_ledger_product_created`, `idx_activity_logs_created`, `idx_reservations_status_expires` — see `database.md:Indexes and Constraints`.
