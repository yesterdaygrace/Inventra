-- 000006_audit_fields down
BEGIN;

ALTER TABLE activity_logs
    DROP COLUMN IF EXISTS before_data,
    DROP COLUMN IF EXISTS after_data,
    DROP COLUMN IF EXISTS reason,
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS request_id;

COMMIT;