-- 000007_indexes: justified indexes for hot query paths (FK joins,
-- name search, timestamp ordering, and filter columns). Every index here was
-- verified absent via pg_indexes before being added; unique/FK-guard indexes
-- that already existed (products.sku, warehouses.code, users.email,
-- refresh_tokens.token_hash, inventory (product_id, warehouse_id), roles.name,
-- categories.name, permissions.code) are NOT duplicated.
BEGIN;

CREATE INDEX IF NOT EXISTS idx_products_category_id     ON products (category_id);
CREATE INDEX IF NOT EXISTS idx_products_name_lower      ON products (LOWER(name));

CREATE INDEX IF NOT EXISTS idx_inventory_warehouse_id   ON inventory (warehouse_id);

CREATE INDEX IF NOT EXISTS idx_inventory_tx_product_id  ON inventory_transactions (product_id);
CREATE INDEX IF NOT EXISTS idx_inventory_tx_warehouse   ON inventory_transactions (warehouse_id);
CREATE INDEX IF NOT EXISTS idx_inventory_tx_transfer_id ON inventory_transactions (transfer_id);
CREATE INDEX IF NOT EXISTS idx_inventory_tx_created_at  ON inventory_transactions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_activity_logs_user_id    ON activity_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_activity_logs_entity     ON activity_logs (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_activity_logs_created_at ON activity_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_users_role_id            ON users (role_id);

COMMIT;