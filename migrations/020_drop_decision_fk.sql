-- Migration 020: Drop FK constraints on decision_id columns
--
-- The decision_id fields on orders and positions were originally FK-linked to
-- llm_decisions(id). However, orchestrator voting decisions are never inserted
-- into llm_decisions — they're a different decision pathway. This FK constraint
-- caused every LinkDecisionToOrder call to fail with a FK violation, making
-- decision traceability a silent no-op.
--
-- This migration drops the FK constraints so decision_id can store any UUID
-- (including voting-based orchestrator decision IDs) as a loose correlation
-- field. The column type and index are preserved for query efficiency.

ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_decision_id_fkey;

ALTER TABLE positions
    DROP CONSTRAINT IF EXISTS positions_decision_id_fkey;
