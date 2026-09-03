-- 000005_rbac: permission catalog + role→permission mapping.
BEGIN;

CREATE TABLE permissions (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    code        text        UNIQUE NOT NULL,
    description text
);

CREATE TABLE role_permissions (
    role_id       uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

COMMIT;