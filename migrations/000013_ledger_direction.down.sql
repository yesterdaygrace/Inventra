BEGIN;

ALTER TABLE inventory_ledger DROP CONSTRAINT IF EXISTS ledger_direction_check;
ALTER TABLE inventory_ledger DROP COLUMN IF EXISTS direction;

COMMIT;
