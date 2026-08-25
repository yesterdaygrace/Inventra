-- 000002_inventory_columns: add reserved_quantity, version to inventory;
-- add reference_type, reference_id, reason to inventory_transactions.
BEGIN;

ALTER TABLE inventory
    ADD COLUMN reserved_quantity integer NOT NULL DEFAULT 0,
    ADD COLUMN version          integer NOT NULL DEFAULT 0;

ALTER TABLE inventory
    ADD CONSTRAINT inventory_reserved_quantity_check CHECK (reserved_quantity >= 0);

ALTER TABLE inventory_transactions
    ADD COLUMN reference_type text,
    ADD COLUMN reference_id   text,
    ADD COLUMN reason         text;

COMMIT;
