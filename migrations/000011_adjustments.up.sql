-- 000011_adjustments: stock adjustment requests with an approval workflow
-- (PRD §23). No direct stock edits: every correction is a request that
-- either auto-approves under the value threshold or waits for a manager.
BEGIN;

CREATE TABLE system_settings (
    key        text        PRIMARY KEY,
    value      text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO system_settings (key, value) VALUES
    ('adjustment_approval_threshold', '500')
ON CONFLICT (key) DO NOTHING;

CREATE TABLE inventory_adjustments (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id       uuid        NOT NULL REFERENCES products(id),
    warehouse_id     uuid        NOT NULL REFERENCES warehouses(id),
    system_quantity  integer     NOT NULL CHECK (system_quantity >= 0),
    counted_quantity integer     NOT NULL CHECK (counted_quantity >= 0),
    reason           text        NOT NULL CHECK (reason IN
        ('COUNT_VARIANCE', 'DAMAGE', 'THEFT', 'EXPIRED_STOCK',
         'SUPPLIER_SHORTAGE', 'SYSTEM_CORRECTION', 'OTHER')),
    note             text,
    status           text        NOT NULL DEFAULT 'PENDING'
                     CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    requested_by     uuid        REFERENCES users(id),
    reviewed_by      uuid        REFERENCES users(id),
    reviewed_at      timestamptz,
    applied_value    numeric(14,2),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_adjustments_status ON inventory_adjustments (status);
CREATE INDEX idx_adjustments_product_warehouse
    ON inventory_adjustments (product_id, warehouse_id);

COMMIT;
