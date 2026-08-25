-- 000004_refresh_token_family down
BEGIN;

DROP INDEX IF EXISTS idx_refresh_tokens_user_family;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS family_id;

COMMIT;