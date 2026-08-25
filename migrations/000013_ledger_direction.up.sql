-- 000013_ledger_direction: disambiguate movement direction for entries
-- whose type alone cannot carry it (an ADJUSTMENT may correct up or down).
-- Balance = SUM(direction='OUT' ? -quantity : quantity).
BEGIN;

ALTER TABLE inventory_ledger ADD COLUMN direction text;
UPDATE inventory_ledger SET direction = CASE
    WHEN transaction_type IN ('ISSUE', 'TRANSFER_OUT') THEN 'OUT'
    ELSE 'IN' END;
ALTER TABLE inventory_ledger ALTER COLUMN direction SET NOT NULL;
ALTER TABLE inventory_ledger ADD CONSTRAINT ledger_direction_check
    CHECK (direction IN ('IN', 'OUT'));

COMMIT;
