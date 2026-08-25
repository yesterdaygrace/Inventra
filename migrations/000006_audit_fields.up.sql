-- 000006_audit_fields: enrich activity_logs with before/after data and
-- request context (user_agent, request_id) plus an optional reason.
BEGIN;

ALTER TABLE activity_logs
    ADD COLUMN before_data jsonb,
    ADD COLUMN after_data  jsonb,
    ADD COLUMN reason      text,
    ADD COLUMN user_agent  text,
    ADD COLUMN request_id  text;

COMMIT;