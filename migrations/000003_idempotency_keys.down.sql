-- 000003_idempotency_keys down
BEGIN;

DROP TABLE IF EXISTS idempotency_keys CASCADE;

COMMIT;
