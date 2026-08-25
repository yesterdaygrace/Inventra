-- Restore the legacy transactions table and drop the ledger.
-- Ledger rows are not migrated back; this exists only to satisfy
-- golang-migrate's down path.
BEGIN;

CREATE TABLE inventory_transactions (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id     uuid        NOT NULL REFERENCES products(id),
    type           text        NOT NULL CHECK (type IN ('IN', 'OUT')),
    quantity       integer     NOT NULL CHECK (quantity > 0),
    unit_cost      numeric(12,2),
    note           text,
    user_id        uuid        REFERENCES users(id),
    warehouse_id   uuid        REFERENCES warehouses(id),
    transfer_id    uuid,
    reference_type text,
    reference_id   text,
    reason         text,
    created_at     timestamptz NOT NULL DEFAULT now()
);

DROP TABLE inventory_ledger;

COMMIT;
