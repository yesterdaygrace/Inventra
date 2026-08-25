-- 000002_inventory_columns down
BEGIN;

ALTER TABLE inventory_transactions DROP COLUMN IF EXISTS reason;
ALTER TABLE inventory_transactions DROP COLUMN IF EXISTS reference_id;
ALTER TABLE inventory_transactions DROP COLUMN IF EXISTS reference_type;

ALTER TABLE inventory DROP CONSTRAINT IF EXISTS inventory_reserved_quantity_check;
ALTER TABLE inventory DROP COLUMN IF EXISTS version;
ALTER TABLE inventory DROP COLUMN IF EXISTS reserved_quantity;

COMMIT;
