-- Remove the expanded roles. Cascades clear their role_permissions grants.
BEGIN;

DELETE FROM roles WHERE name IN ('WAREHOUSE_MANAGER', 'VIEWER');

COMMIT;
