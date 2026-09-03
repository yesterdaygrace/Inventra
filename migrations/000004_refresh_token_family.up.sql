-- 000004_refresh_token_family: group refresh tokens into families so a
-- detected reuse (replay of a revoked token) can revoke the entire session.
BEGIN;

ALTER TABLE refresh_tokens
    ADD COLUMN family_id uuid NOT NULL DEFAULT gen_random_uuid();

CREATE INDEX idx_refresh_tokens_user_family ON refresh_tokens (user_id, family_id);

COMMIT;