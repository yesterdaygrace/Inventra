-- 000012_cycle_counts: warehouse-scoped count plans with explicit SKU
-- lists (PRD §24). Item system quantities are snapshotted at creation;
-- counting a variance files an adjustment request through the §23 workflow.
BEGIN;

CREATE TABLE cycle_count_plans (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    warehouse_id uuid      NOT NULL REFERENCES warehouses(id),
    name       text        NOT NULL,
    status     text        NOT NULL DEFAULT 'OPEN'
               CHECK (status IN ('OPEN', 'COMPLETED')),
    created_by uuid        REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE cycle_count_items (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id          uuid        NOT NULL REFERENCES cycle_count_plans(id) ON DELETE CASCADE,
    product_id       uuid        NOT NULL REFERENCES products(id),
    system_quantity  integer     NOT NULL CHECK (system_quantity >= 0),
    counted_quantity integer     CHECK (counted_quantity >= 0),
    counted_by       uuid        REFERENCES users(id),
    counted_at       timestamptz,
    adjustment_id    uuid        REFERENCES inventory_adjustments(id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (plan_id, product_id)
);

CREATE INDEX idx_count_items_plan ON cycle_count_items (plan_id);
CREATE INDEX idx_count_plans_status ON cycle_count_plans (status);

COMMIT;
