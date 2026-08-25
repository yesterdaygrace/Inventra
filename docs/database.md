# Database Specification — Enterprise Inventory Management System

**Status**: Approved  
**Version**: 1.0  
**Phase**: 1 — Foundation  
**Last Updated**: 2026-08-05  

## Overview

This document defines the database schema for the Enterprise Inventory Management System Phase 1. The schema implements the 8 core tables from PRD #7, following locked conventions and enterprise software practices. This specification is **authoritative** — all Go model implementations (W2.1-W2.3) must match it exactly.

## Schema Management

- **Migration strategy**: GORM AutoMigrate (no versioned migrations)
- **Migrations directory**: `migrations/` reserved for seed SQL and documentation only
- **Database**: PostgreSQL 17 (via `gorm.io/driver/postgres` pgx driver)
- **Primary keys**: UUID v4 (`google/uuid`)

## Entity Relationship Diagram

```mermaid
erDiagram
    roles {
        uuid id PK
        string name UK "ADMIN, STAFF"
        timestamp created_at
    }

    users {
        uuid id PK
        string name
        string email UK
        string password_hash "bcrypt"
        uuid role_id FK
        bool is_active "default true"
        timestamp created_at
        timestamp updated_at
    }

    refresh_tokens {
        uuid id PK
        uuid user_id FK
        string token_hash UK "sha256 hex"
        timestamp expires_at
        timestamp revoked_at NULL
        timestamp created_at
    }

    categories {
        uuid id PK
        string name UK
        string description NULL
        timestamp created_at
        timestamp updated_at
    }

    warehouses {
        uuid id PK
        string code UK
        string name
        string description NULL
        bool is_active "default true (soft-delete)"
        timestamp created_at
        timestamp updated_at
    }

    products {
        uuid id PK
        string name
        string sku UK
        string description NULL
        numeric price "numeric(12,2)"
        uuid category_id FK
        int low_stock_threshold "default 10"
        bool is_archived "default false (soft-delete)"
        timestamp created_at
        timestamp updated_at
    }

    inventory {
        uuid id PK
        uuid product_id FK
        uuid warehouse_id FK
        int quantity ">= 0"
        timestamp updated_at
        string unique_pair "UK (product_id, warehouse_id)"
    }

    inventory_transactions {
        uuid id PK
        uuid product_id FK
        string type "IN, OUT"
        int quantity "> 0"
        numeric unit_cost NULL "numeric(12,2)"
        string note NULL
        uuid user_id NULL FK
        uuid warehouse_id NULL FK
        uuid transfer_id NULL "pairs two rows of a transfer"
        timestamp created_at
    }

    activity_logs {
        uuid id PK
        uuid user_id NULL FK
        string action
        string entity_type
        string entity_id NULL
        jsonb details NULL
        string ip NULL
        timestamp created_at
    }

    roles ||--o{ users : "one-to-many"
    users ||--o{ refresh_tokens : "one-to-many"
    users ||--o{ inventory_transactions : "one-to-many"
    users ||--o{ activity_logs : "one-to-many"
    categories ||--o{ products : "one-to-many"
    warehouses ||--o{ inventory : "one-to-many"
    warehouses ||--o{ inventory_transactions : "one-to-many"
    products ||--o{ inventory : "one-to-many (per warehouse)"
    products ||--o{ inventory_transactions : "one-to-many"
```

## Table Specifications

### 1. `roles`

| Column | Type | Nullable | Default | Constraints | Description |
|--------|------|----------|---------|-------------|-------------|
| `id` | `uuid` | NO | `gen_random_uuid()` | PRIMARY KEY | Role identifier |
| `name` | `text` | NO | | UNIQUE, CHECK (`name` IN ('ADMIN', 'STAFF')) | Role name (ADMIN/STAFF) |
| `created_at` | `timestamp` | NO | `CURRENT_TIMESTAMP` | | Creation timestamp |

**Notes**:
- Two predefined roles seeded: `ADMIN` (full system access), `STAFF` (limited access)
- RBAC matrix enforced by middleware

### 2. `users`

| Column | Type | Nullable | Default | Constraints | Description |
|--------|------|----------|---------|-------------|-------------|
| `id` | `uuid` | NO | `gen_random_uuid()` | PRIMARY KEY | User identifier |
| `name` | `text` | NO | | | Full name |
| `email` | `text` | NO | | UNIQUE | Email address (login) |
| `password_hash` | `text` | NO | | | bcrypt hash (cost 12) |
| `role_id` | `uuid` | NO | | FOREIGN KEY (`roles.id`) | User role |
| `is_active` | `boolean` | NO | `true` | | Account status |
| `created_at` | `timestamp` | NO | `CURRENT_TIMESTAMP` | | Creation timestamp |
| `updated_at` | `timestamp` | YES | `CURRENT_TIMESTAMP` | | Last update timestamp |

**Notes**:
- Default admin seeded: `admin@inventory.local` / `Admin123!` (change required)
- `is_active = false` prevents login but retains data integrity

### 3. `refresh_tokens`

| Column | Type | Nullable | Default | Constraints | Description |
|--------|------|----------|---------|-------------|-------------|
| `id` | `uuid` | NO | `gen_random_uuid()` | PRIMARY KEY | Token identifier |
| `user_id` | `uuid` | NO | | FOREIGN KEY (`users.id`) | Associated user |
| `token_hash` | `text` | NO | | UNIQUE | SHA-256 hash of refresh token |
| `expires_at` | `timestamp` | NO | | | Expiration time (7 days) |
| `revoked_at` | `timestamp` | YES | | | Revocation timestamp |
| `created_at` | `timestamp` | NO | `CURRENT_TIMESTAMP` | | Creation timestamp |

**Notes**:
- Implements refresh token rotation
- `revoked_at` marks tokens revoked before expiration
- Old tokens cleaned via scheduled job (future phase)

### 4. `categories`

| Column | Type | Nullable | Default | Constraints | Description |
|--------|------|----------|---------|-------------|-------------|
| `id` | `uuid` | NO | `gen_random_uuid()` | PRIMARY KEY | Category identifier |
| `name` | `text` | NO | | UNIQUE | Category name |
| `description` | `text` | YES | | | Optional description |
| `created_at` | `timestamp` | NO | `CURRENT_TIMESTAMP` | | Creation timestamp |
| `updated_at` | `timestamp` | YES | `CURRENT_TIMESTAMP` | | Last update timestamp |

**Notes**:
- Cannot delete if referenced by products (FK constraint)
- Used for product organization and dashboard reporting

### 5. `products`

| Column | Type | Nullable | Default | Constraints | Description |
|--------|------|----------|---------|-------------|-------------|
| `id` | `uuid` | NO | `gen_random_uuid()` | PRIMARY KEY | Product identifier |
| `name` | `text` | NO | | | Product name |
| `sku` | `text` | NO | | UNIQUE | Stock Keeping Unit |
| `description` | `text` | YES | | | Optional description |
| `price` | `numeric(12,2)` | NO | | | Unit price (decimal) |
| `category_id` | `uuid` | NO | | FOREIGN KEY (`categories.id`) | Product category |
| `low_stock_threshold` | `integer` | NO | `10` | CHECK (`low_stock_threshold` >= 0) | Low stock warning level |
| `is_archived` | `boolean` | NO | `false` | | Soft delete flag |
| `created_at` | `timestamp` | NO | `CURRENT_TIMESTAMP` | | Creation timestamp |
| `updated_at` | `timestamp` | YES | `CURRENT_TIMESTAMP` | | Last update timestamp |

**Notes**:
- `is_archived = true` marks soft-deleted products (excluded from active listings)
- SKU must be unique across all products
- Price stored as decimal for financial accuracy

### 6. `warehouses`

| Column | Type | Nullable | Default | Constraints | Description |
|--------|------|----------|---------|-------------|-------------|
| `id` | `uuid` | NO | `gen_random_uuid()` | PRIMARY KEY | Warehouse identifier |
| `code` | `text` | NO | | UNIQUE | Warehouse code (short, human-readable, e.g. DEFAULT, WH-001) |
| `name` | `text` | NO | | | Display name |
| `description` | `text` | YES | | | Optional description |
| `is_active` | `boolean` | NO | `true` | | Soft-deactivate flag |
| `created_at` | `timestamp` | NO | `CURRENT_TIMESTAMP` | | Creation timestamp |
| `updated_at` | `timestamp` | YES | `CURRENT_TIMESTAMP` | | Last update timestamp |

**Notes**:
- A `DEFAULT` warehouse is seeded by `cmd/seed` and used as the fallback for stock movements that omit a warehouse
- Soft-deactivate only; hard delete blocked while inventory rows reference the warehouse

### 7. `inventory`

| Column | Type | Nullable | Default | Constraints | Description |
|--------|------|----------|---------|-------------|-------------|
| `id` | `uuid` | NO | `gen_random_uuid()` | PRIMARY KEY | Inventory record identifier |
| `product_id` | `uuid` | NO | | UNIQUE (`product_id, warehouse_id`), FOREIGN KEY (`products.id`) | Associated product |
| `warehouse_id` | `uuid` | NO | | UNIQUE (`product_id, warehouse_id`) | Associated warehouse |
| `quantity` | `integer` | NO | `0` | CHECK (`quantity` >= 0) | Current stock quantity in this warehouse |
| `reserved_quantity` | `integer` | NO | `0` | CHECK (`reserved_quantity` >= 0) | Quantity reserved for pending operations (0 until the reservation flow lands) |
| `version` | `integer` | NO | `0` | | Optimistic-lock version, bumped on every movement |
| `updated_at` | `timestamp` | YES | `CURRENT_TIMESTAMP` | | Last stock update |

**Notes**:
- One row per (product, warehouse) pair — the same product can be tracked across multiple locations
- The legacy single-warehouse unique constraint on `product_id` alone was replaced by the composite key; `cmd/seed` backfills existing rows to the `DEFAULT` warehouse
- Quantity updated atomically via transactions
- `version` increments on every stock-in/stock-out/transfer; `reserved_quantity` is always `0` until the reservation flow ships (fix.md §8 availability semantics only)

### 8. `inventory_transactions`

| Column | Type | Nullable | Default | Constraints | Description |
|--------|------|----------|---------|-------------|-------------|
| `id` | `uuid` | NO | `gen_random_uuid()` | PRIMARY KEY | Transaction identifier |
| `product_id` | `uuid` | NO | | FOREIGN KEY (`products.id`) | Associated product |
| `type` | `text` | NO | | CHECK (`type` IN ('IN', 'OUT')) | Transaction type (IN=stock in, OUT=stock out) |
| `quantity` | `integer` | NO | | CHECK (`quantity` > 0) | Quantity moved |
| `unit_cost` | `numeric(12,2)` | YES | | | Unit cost at time of transaction |
| `note` | `text` | YES | | | Optional note |
| `user_id` | `uuid` | YES | | FOREIGN KEY (`users.id`) | User who performed transaction |
| `warehouse_id` | `uuid` | YES | | | Warehouse where this movement occurred |
| `transfer_id` | `uuid` | YES | | | Groups two rows of a warehouse transfer (OUT + IN) sharing one UUID |
| `reference_type` | `text` | YES | | | Optional external reference type (e.g., "PURCHASE_ORDER") |
| `reference_id` | `text` | YES | | | Optional external reference identifier (e.g., "PO-00123") |
| `reason` | `text` | YES | | | Optional movement reason |
| `created_at` | `timestamp` | NO | `CURRENT_TIMESTAMP` | | Creation timestamp |

**Notes**:
- Complete audit trail of all inventory movements
- Two rows sharing the same `transfer_id` encode a warehouse-to-warehouse transfer (OUT from source, IN to destination)
- `unit_cost` nullable for OUT transactions (use last IN cost or product price)
- `reference_type`/`reference_id`/`reason` are NULL when the caller omits them — no behavior change for today's clients
- Indexed for dashboard reporting and history queries

### 9. `activity_logs`

| Column | Type | Nullable | Default | Constraints | Description |
|--------|------|----------|---------|-------------|-------------|
| `id` | `uuid` | NO | `gen_random_uuid()` | PRIMARY KEY | Log entry identifier |
| `user_id` | `uuid` | YES | | FOREIGN KEY (`users.id`) | User who performed action |
| `action` | `text` | NO | | | Action type (e.g., "LOGIN", "CREATE_PRODUCT") |
| `entity_type` | `text` | NO | | | Entity type (e.g., "product", "category") |
| `entity_id` | `text` | YES | | | Entity identifier (UUID as text) |
| `details` | `jsonb` | YES | | | Additional context as JSON |
| `ip` | `text` | YES | | | Client IP address |
| `before_data` | `jsonb` | YES | | | Pre-mutation state (e.g., `{"quantity":10}` on stock-in) |
| `after_data` | `jsonb` | YES | | | Post-mutation state (e.g., `{"quantity":15}`) |
| `reason` | `text` | YES | | | Optional mutation reason |
| `user_agent` | `text` | YES | | | Request `User-Agent` header |
| `request_id` | `text` | YES | | | Request ID (`X-Request-ID`, server-generated if absent) |
| `created_at` | `timestamp` | NO | `CURRENT_TIMESTAMP` | | Creation timestamp |

**Notes**:
- System-wide audit log
- `details` stores structured JSON for extensibility
- `before_data`/`after_data` capture real pre/post quantities for stock operations
- **Append-only**: the table has no update/delete path anywhere in the codebase;
  the repository surface is pinned to `Create` + `List` only
- Used for security monitoring and user activity tracking

## Low-Stock Semantics

A product is considered **low stock** when (in a given warehouse, when filtered):

1. **Inventory condition**: `SUM(inventory.quantity) <= products.low_stock_threshold` (aggregated across warehouses unless a `warehouse_id` filter is applied)
2. **Product status**: `products.is_archived = false`

**Configuration**:
- **Per-product threshold**: `products.low_stock_threshold` (default: 10)
- **Global default**: `LOW_STOCK_THRESHOLD` environment variable (default: 10)
- **Dashboard indicator**: Shows count and list of low-stock products

## Indexes and Constraints

| Table | Index/Constraint | Type | Purpose |
|-------|-----------------|------|---------|
| `roles` | `roles_name_key` | UNIQUE | Enforce unique role names |
| `users` | `users_email_key` | UNIQUE | Enforce unique email addresses |
| `users` | `users_role_id_fkey` | FOREIGN KEY | Reference `roles.id` |
| `refresh_tokens` | `refresh_tokens_token_hash_key` | UNIQUE | Prevent token reuse |
| `refresh_tokens` | `refresh_tokens_user_id_fkey` | FOREIGN KEY | Reference `users.id` |
| `categories` | `categories_name_key` | UNIQUE | Enforce unique category names |
| `warehouses` | `warehouses_code_key` | UNIQUE | Enforce unique warehouse codes |
| `products` | `products_sku_key` | UNIQUE | Enforce unique SKUs |
| `products` | `products_category_id_fkey` | FOREIGN KEY | Reference `categories.id` |
| `inventory` | `idx_inventory_product_warehouse` | UNIQUE (`product_id`, `warehouse_id`) | One inventory record per product per warehouse |
| `inventory` | `inventory_product_id_fkey` | FOREIGN KEY | Reference `products.id` |
| `inventory_transactions` | `idx_inventory_transactions_product_created` | INDEX (`product_id`, `created_at` DESC) | Fast history queries |
| `inventory_transactions` | `inventory_transactions_product_id_fkey` | FOREIGN KEY | Reference `products.id` |
| `inventory_transactions` | `inventory_transactions_user_id_fkey` | FOREIGN KEY | Reference `users.id` |
| `activity_logs` | `idx_activity_logs_created` | INDEX (`created_at` DESC) | Fast recent activity queries |
| `activity_logs` | `activity_logs_user_id_fkey` | FOREIGN KEY | Reference `users.id` |

## Decimal Handling Strategy

**Phase 1 decision**: Use Go `float64` for `price` and `unit_cost` columns.

**Rationale**:
- Simplicity for Phase 1 learning project
- GORM maps `numeric(12,2)` to `float64` by default
- Sufficient precision for currency values (2 decimal places)
- Known tradeoff: floating-point rounding for financial calculations

**Future consideration**: Migrate to `pgtype.Numeric` or decimal library in Phase 2 for production financial accuracy.

## Model Mapping (Go Struct Fields)

This section defines the exact Go struct fields for W2 model implementation.

### Role Model
```go
type Role struct {
    ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Name      string    `gorm:"type:text;unique;check:name IN ('ADMIN', 'STAFF')"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
}
```

### User Model
```go
type User struct {
    ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Name         string    `gorm:"type:text;not null"`
    Email        string    `gorm:"type:text;unique;not null"`
    PasswordHash string    `gorm:"type:text;not null"`
    RoleID       uuid.UUID `gorm:"type:uuid;not null"`
    Role         Role      `gorm:"foreignKey:RoleID"`
    IsActive     bool      `gorm:"not null;default:true"`
    CreatedAt    time.Time `gorm:"autoCreateTime"`
    UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}
```

### RefreshToken Model
```go
type RefreshToken struct {
    ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    UserID    uuid.UUID `gorm:"type:uuid;not null"`
    User      User      `gorm:"foreignKey:UserID"`
    TokenHash string    `gorm:"type:text;unique;not null"`
    ExpiresAt time.Time `gorm:"not null"`
    RevokedAt *time.Time
    CreatedAt time.Time `gorm:"autoCreateTime"`
}
```

### Category Model
```go
type Category struct {
    ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Name        string    `gorm:"type:text;unique;not null"`
    Description *string   `gorm:"type:text"`
    CreatedAt   time.Time `gorm:"autoCreateTime"`
    UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}
```

### Warehouse Model
```go
type Warehouse struct {
    ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Code        string    `gorm:"type:text;unique;not null"`
    Name        string    `gorm:"type:text;not null"`
    Description *string   `gorm:"type:text"`
    IsActive    bool      `gorm:"not null;default:true"`
    CreatedAt   time.Time `gorm:"autoCreateTime"`
    UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}
```

### Product Model
```go
type Product struct {
    ID                uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Name              string    `gorm:"type:text;not null"`
    SKU               string    `gorm:"type:text;unique;not null"`
    Description       *string   `gorm:"type:text"`
    Price             float64   `gorm:"type:numeric(12,2);not null"`
    CategoryID        uuid.UUID `gorm:"type:uuid;not null"`
    Category          Category  `gorm:"foreignKey:CategoryID"`
    LowStockThreshold int       `gorm:"not null;default:10;check:low_stock_threshold >= 0"`
    IsArchived        bool      `gorm:"not null;default:false"`
    CreatedAt         time.Time `gorm:"autoCreateTime"`
    UpdatedAt         time.Time `gorm:"autoUpdateTime"`
}
```

### Inventory Model
```go
type Inventory struct {
    ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    ProductID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_inventory_product_warehouse"`
    Product     Product   `gorm:"foreignKey:ProductID"`
    WarehouseID uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_inventory_product_warehouse"`
    Quantity    int       `gorm:"not null;default:0;check:quantity >= 0"`
    UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}
```

### InventoryTransaction Model
```go
type InventoryTransaction struct {
    ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    ProductID   uuid.UUID  `gorm:"type:uuid;not null"`
    Product     Product    `gorm:"foreignKey:ProductID"`
    Type        string     `gorm:"type:text;not null;check:type IN ('IN', 'OUT')"`
    Quantity    int        `gorm:"not null;check:quantity > 0"`
    UnitCost    *float64   `gorm:"type:numeric(12,2)"`
    Note        *string    `gorm:"type:text"`
    UserID      *uuid.UUID `gorm:"type:uuid"`
    User        *User      `gorm:"foreignKey:UserID"`
    WarehouseID *uuid.UUID `gorm:"type:uuid"`
    TransferID  *uuid.UUID `gorm:"type:uuid"`
    CreatedAt   time.Time  `gorm:"autoCreateTime"`
}
```

### ActivityLog Model
```go
type ActivityLog struct {
    ID         uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    UserID     *uuid.UUID      `gorm:"type:uuid"`
    User       *User           `gorm:"foreignKey:UserID"`
    Action     string          `gorm:"type:text;not null"`
    EntityType string          `gorm:"type:text;not null"`
    EntityID   *string         `gorm:"type:text"`
    Details    *datatypes.JSON `gorm:"type:jsonb"`
    IP         *string         `gorm:"type:text"`
    CreatedAt  time.Time       `gorm:"autoCreateTime"`
}
```

## Schema Management

Schema is owned by versioned SQL migrations in `migrations/`, applied via
`make migrate-up` (golang-migrate). GORM AutoMigrate remains available for
local development only (`DB_AUTOMIGRATE=true` default) — never in production
(`DB_AUTOMIGRATE=false` via docker-compose).

## Validation

All W2 model implementations (T2.1-T2.3) must:
1. Match column names, types, and constraints exactly as specified
2. Implement GORM tags as shown in Model Mapping section
3. Pass AutoMigrate tests against PostgreSQL 17
4. Enforce all unique and foreign key constraints at database level
5. Support low-stock semantics query as defined

---

**Next**: Implementation of Go models in W2.1-W2.3 must reference this specification as authoritative source.