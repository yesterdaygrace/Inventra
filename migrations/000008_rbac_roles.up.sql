-- 000008_rbac_roles: add WAREHOUSE_MANAGER and VIEWER roles (PRD §41).
-- Widens BOTH roles.name CHECK constraints — the inline one from
-- migration 000001 (roles_name_check) and the GORM-tag mirror that
-- AutoMigrate creates (chk_roles_name) — then inserts the new roles.
-- Permission catalog rows and role_permissions grants are owned by the
-- seed CLI (cmd/seed), which derives them from auth.PermissionSetForRole.
BEGIN;

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_check;
ALTER TABLE roles DROP CONSTRAINT IF EXISTS chk_roles_name;
ALTER TABLE roles ADD CONSTRAINT chk_roles_name
    CHECK (name IN ('ADMIN', 'WAREHOUSE_MANAGER', 'STAFF', 'VIEWER'));

INSERT INTO roles (name)
SELECT v.name
FROM (VALUES ('WAREHOUSE_MANAGER'), ('VIEWER')) AS v(name)
WHERE NOT EXISTS (SELECT 1 FROM roles r WHERE r.name = v.name);

COMMIT;
