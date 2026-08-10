# Entity-Relationship Diagram

**Document Version:** 1.0
**Date:** 2026-08-06
**Status:** authoritative — matches `internal/*/model.go` GORM models.

## Mermaid

```mermaid
erDiagram
    roles {
        uuid ID PK
        text Name UK "ADMIN | STAFF"
        timestamptz CreatedAt
    }

    users {
        uuid ID PK
        text Name
        text Email UK
        text PasswordHash
        uuid RoleID FK
        boolean IsActive
        timestamptz CreatedAt
        timestamptz UpdatedAt
    }

    categories {
        uuid ID PK
        text Name UK
        text Description
        timestamptz CreatedAt
        timestamptz UpdatedAt
    }

    warehouses {
        uuid ID PK
        text Code UK "e.g. DEFAULT, WH-001"
        text Name
        text Description
        boolean IsActive "default true"
        timestamptz CreatedAt
        timestamptz UpdatedAt
    }

    products {
        uuid ID PK
        text Name
        text SKU UK
        text Description
        numeric Price
        uuid CategoryID FK
        int LowStockThreshold
        boolean IsArchived
        timestamptz CreatedAt
        timestamptz UpdatedAt
    }

    inventory {
        uuid ID PK
        uuid ProductID FK
        uuid WarehouseID FK
        int Quantity
        string UniquePair "UK (ProductID, WarehouseID)"
        timestamptz UpdatedAt
    }

    inventory_transactions {
        uuid ID PK
        uuid ProductID FK
        text Type "IN | OUT"
        int Quantity
        numeric UnitCost
        text Note
        uuid UserID FK
        uuid WarehouseID FK
        uuid TransferID "nullable, groups two rows of a transfer"
        timestamptz CreatedAt
    }

    activity_logs {
        uuid ID PK
        uuid UserID FK
        text Action
        text EntityType
        text EntityID
        jsonb Details
        text IP
        timestamptz CreatedAt
    }

    roles ||--o{ users : "has"
    users ||--o{ activity_logs : "performs"
    users ||--o{ inventory_transactions : "records"
    categories ||--o{ products : "contains"
    warehouses ||--o{ inventory : "stocks"
    warehouses ||--o{ inventory_transactions : "locates"
    products ||--o{ inventory : "has per warehouse"
    products ||--o{ inventory_transactions : "moves"
```

## Relationships (verbal)

- **roles → users**: one role (`ADMIN`/`STAFF`) has many users.
- **users → activity_logs**: a user performs audit events; `user_id` is nullable
  (system/seed events carry no user).
- **users → inventory_transactions**: nullable `user_id` records who performed
  a stock movement.
- **categories → products**: one category contains many products.
- **warehouses → inventory**: a warehouse holds per-product stock rows.
- **warehouses → inventory_transactions**: each movement is scoped to a warehouse.
- **products → inventory**: one row per (product, warehouse) pair — multiple
  locations tracked via the composite unique key.
- **products → inventory_transactions**: each movement (`IN`/`OUT`) references
  the product; quantity is always `> 0` (signed by `type`).
- **inventory_transactions transfer_id**: two rows sharing a non-null `transfer_id`
  encode a warehouse-to-warehouse transfer (OUT from source, IN to destination).

## Constraints

- `inventory_transactions.type` ∈ {`IN`, `OUT`}; `inventory.quantity >= 0`.
- `inventory` composite unique: `(product_id, warehouse_id)` — one stock row per location.
- Transfers use two `inventory_transactions` rows with the same `transfer_id` (one `OUT`, one `IN`).
- `products.low_stock_threshold >= 0` (default 10).
- `roles.name` ∈ {`ADMIN`, `STAFF`}.
- Unique: `users.email`, `products.sku`, `categories.name`, `inventory.product_id`.