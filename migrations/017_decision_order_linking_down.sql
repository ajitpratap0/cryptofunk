DROP INDEX IF EXISTS idx_orders_decision_id;
DROP INDEX IF EXISTS idx_positions_decision_id;
ALTER TABLE orders DROP COLUMN IF EXISTS decision_id;
ALTER TABLE positions DROP COLUMN IF EXISTS decision_id;
