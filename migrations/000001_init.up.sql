-- 000001_init: baseline schema matching the current GORM AutoMigrate output.
-- FK order: roles → categories → warehouses → users → products → inventory
--          → inventory_transactions → refresh_tokens → activity_logs

BEGIN;

-- 1. roles (independent parent)
CREATE TABLE roles (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        UNIQUE NOT NULL CHECK (name IN ('ADMIN', 'STAFF')),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- 2. categories (independent parent)
CREATE TABLE categories (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text        UNIQUE NOT NULL,
    description text,
    is_active   boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- 3. warehouses (independent parent)
CREATE TABLE warehouses (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    code        text        UNIQUE NOT NULL,
    name        text        NOT NULL,
    description text,
    is_active   boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- 4. users (depends on roles)
CREATE TABLE users (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name          text        NOT NULL,
    email         text        UNIQUE NOT NULL,
    password_hash text        NOT NULL,
    role_id       uuid        NOT NULL REFERENCES roles(id),
    is_active     boolean     NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- 5. products (depends on categories)
CREATE TABLE products (
    id                  uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    name                text          NOT NULL,
    sku                 text          UNIQUE NOT NULL,
    description         text,
    price               numeric(12,2) NOT NULL,
    category_id         uuid          NOT NULL REFERENCES categories(id),
    low_stock_threshold integer       NOT NULL DEFAULT 10 CHECK (low_stock_threshold >= 0),
    is_archived         boolean       NOT NULL DEFAULT false,
    created_at          timestamptz   NOT NULL DEFAULT now(),
    updated_at          timestamptz   NOT NULL DEFAULT now()
);

-- 6. inventory (depends on products, warehouses)
CREATE TABLE inventory (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  uuid        NOT NULL REFERENCES products(id),
    warehouse_id uuid       NOT NULL REFERENCES warehouses(id),
    quantity    integer     NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (product_id, warehouse_id)
);

-- 7. inventory_transactions (depends on products; user_id/warehouse_id nullable FKs)
CREATE TABLE inventory_transactions (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id   uuid        NOT NULL REFERENCES products(id),
    type         text        NOT NULL CHECK (type IN ('IN', 'OUT')),
    quantity     integer     NOT NULL CHECK (quantity > 0),
    unit_cost    numeric(12,2),
    note         text,
    user_id      uuid        REFERENCES users(id),
    warehouse_id uuid        REFERENCES warehouses(id),
    transfer_id  uuid,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- 8. refresh_tokens (depends on users)
CREATE TABLE refresh_tokens (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users(id),
    token_hash text        UNIQUE NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- 9. activity_logs (depends on users)
CREATE TABLE activity_logs (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        REFERENCES users(id),
    action      text        NOT NULL,
    entity_type text        NOT NULL,
    entity_id   text,
    details     jsonb,
    ip          text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

COMMIT;
