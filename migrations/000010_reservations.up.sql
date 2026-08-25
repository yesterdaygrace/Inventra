-- 000010_reservations: stock reservations per PRD §21. A reservation
-- answers "how much stock is actually available?" — Available = On Hand −
-- Reserved. Expiry is lazy: rows flip to EXPIRED when first observed past
-- expires_at, releasing their quantity in the observing transaction.
BEGIN;

CREATE TABLE inventory_reservations (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id     uuid        NOT NULL REFERENCES products(id),
    warehouse_id   uuid        NOT NULL REFERENCES warehouses(id),
    quantity       integer     NOT NULL CHECK (quantity > 0),
    reference_type text        NOT NULL,
    reference_id   text        NOT NULL,
    status         text        NOT NULL DEFAULT 'ACTIVE'
                   CHECK (status IN ('ACTIVE', 'RELEASED', 'CONSUMED', 'EXPIRED')),
    expires_at     timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_reservations_product_warehouse_active
    ON inventory_reservations (product_id, warehouse_id) WHERE status = 'ACTIVE';
CREATE INDEX idx_reservations_status ON inventory_reservations (status);

COMMIT;
