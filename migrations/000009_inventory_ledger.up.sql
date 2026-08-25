-- 000009_inventory_ledger: replace inventory_transactions with the
-- append-oriented ledger per PRD §15. Every stock-changing operation
-- writes a row here in the same transaction as the stock mutation.
BEGIN;

CREATE TABLE inventory_ledger (
    id               uuid           PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id       uuid           NOT NULL REFERENCES products(id),
    warehouse_id     uuid           NOT NULL REFERENCES warehouses(id),
    batch_id         uuid,          -- reserved for batch tracking (Phase 5)
    transaction_type text           NOT NULL CHECK (transaction_type IN
        ('OPENING_BALANCE', 'RECEIVE', 'ISSUE', 'TRANSFER_IN', 'TRANSFER_OUT',
         'ADJUSTMENT', 'RETURN')),
    quantity         integer        NOT NULL CHECK (quantity > 0),
    unit_cost        numeric(12,2),
    total_cost       numeric(14,2),
    reference_type   text,
    reference_id     text,
    transfer_id      uuid,          -- pairs TRANSFER_OUT with its TRANSFER_IN
    note             text,
    reason           text,
    performed_by     uuid           REFERENCES users(id),
    created_at       timestamptz    NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_product_warehouse_created
    ON inventory_ledger (product_id, warehouse_id, created_at);
CREATE INDEX idx_ledger_type ON inventory_ledger (transaction_type);
CREATE INDEX idx_ledger_transfer ON inventory_ledger (transfer_id);
CREATE INDEX idx_ledger_created ON inventory_ledger (created_at);

DROP TABLE IF EXISTS inventory_transactions;

COMMIT;
