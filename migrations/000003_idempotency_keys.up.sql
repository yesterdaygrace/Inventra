-- 000003_idempotency_keys: stores 2xx responses for replayable stock write routes.
BEGIN;

CREATE TABLE idempotency_keys (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash        text        NOT NULL,
    user_id         uuid        NOT NULL REFERENCES users(id),
    endpoint        text        NOT NULL,
    request_hash    text        NOT NULL,
    response_status integer     NOT NULL,
    response_body   text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    CONSTRAINT uni_idempotency_keys_key_hash UNIQUE (key_hash)
);

CREATE INDEX idx_idempotency_keys_created_at ON idempotency_keys (created_at);

COMMIT;
