-- 000001_init down: drop in reverse FK order (children before parents).
BEGIN;

DROP TABLE IF EXISTS activity_logs      CASCADE;
DROP TABLE IF EXISTS refresh_tokens     CASCADE;
DROP TABLE IF EXISTS inventory_transactions CASCADE;
DROP TABLE IF EXISTS inventory          CASCADE;
DROP TABLE IF EXISTS products           CASCADE;
DROP TABLE IF EXISTS users              CASCADE;
DROP TABLE IF EXISTS warehouses         CASCADE;
DROP TABLE IF EXISTS categories         CASCADE;
DROP TABLE IF EXISTS roles              CASCADE;

COMMIT;
