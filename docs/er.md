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
        uuid ProductID FK UK
        int Quantity
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
    products ||--|| inventory : "has"
    products ||--o{ inventory_transactions : "moves"
```

## Relationships (verbal)

- **roles → users**: one role (`ADMIN`/`STAFF`) has many users.
- **users → activity_logs**: a user performs audit events; `user_id` is nullable
  (system/seed events carry no user).
- **users → inventory_transactions**: nullable `user_id` records who performed
  a stock movement.
- **categories → products**: one category contains many products.
- **products → inventory**: one-to-one current stock row per product (the
  `inventory` row holds the live `quantity`).
- **products → inventory_transactions**: each movement (`IN`/`OUT`) references
  the product; quantity is always `> 0` (signed by `type`).

## Constraints

- `inventory_transactions.type` ∈ {`IN`, `OUT`}; `inventory.quantity >= 0`.
- `products.low_stock_threshold >= 0` (default 10).
- `roles.name` ∈ {`ADMIN`, `STAFF`}.
- Unique: `users.email`, `products.sku`, `categories.name`, `inventory.product_id`.