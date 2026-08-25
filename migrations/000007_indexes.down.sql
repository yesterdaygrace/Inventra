-- 000007_indexes down
BEGIN;

DROP INDEX IF EXISTS idx_users_role_id;
DROP INDEX IF EXISTS idx_activity_logs_created_at;
DROP INDEX IF EXISTS idx_activity_logs_entity;
DROP INDEX IF EXISTS idx_activity_logs_user_id;
DROP INDEX IF EXISTS idx_inventory_tx_created_at;
DROP INDEX IF EXISTS idx_inventory_tx_transfer_id;
DROP INDEX IF EXISTS idx_inventory_tx_warehouse;
DROP INDEX IF EXISTS idx_inventory_tx_product_id;
DROP INDEX IF EXISTS idx_inventory_warehouse_id;
DROP INDEX IF EXISTS idx_products_name_lower;
DROP INDEX IF EXISTS idx_products_category_id;

COMMIT;