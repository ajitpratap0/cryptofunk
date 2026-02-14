-- =============================================================================
-- Migration 017: Decision-Order Linking
-- =============================================================================
-- Adds a decision_id column to orders table for linking decisions to orders,
-- and a decision_id column to positions for linking decisions to positions.

ALTER TABLE orders ADD COLUMN IF NOT EXISTS decision_id UUID REFERENCES llm_decisions(id) ON DELETE SET NULL;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS decision_id UUID REFERENCES llm_decisions(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_orders_decision_id ON orders (decision_id) WHERE decision_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_positions_decision_id ON positions (decision_id) WHERE decision_id IS NOT NULL;

SELECT 'Migration 017: Decision-order linking columns created!' AS status;
