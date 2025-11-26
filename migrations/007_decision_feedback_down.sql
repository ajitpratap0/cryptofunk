-- =============================================================================
-- Migration 007 Down: Remove Decision Feedback System
-- =============================================================================

DROP VIEW IF EXISTS decisions_needing_review;
DROP MATERIALIZED VIEW IF EXISTS decision_feedback_stats;
DROP TRIGGER IF EXISTS decision_feedback_updated_at ON decision_feedback;
DROP FUNCTION IF EXISTS update_decision_feedback_updated_at();
DROP TABLE IF EXISTS decision_feedback;
DROP TYPE IF EXISTS feedback_rating;

SELECT 'Migration 007 down: Decision feedback system removed!' AS status;
