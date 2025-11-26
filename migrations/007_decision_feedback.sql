-- =============================================================================
-- Migration 007: Decision Feedback System
-- =============================================================================
-- This migration adds user feedback capabilities for LLM decisions (T309)
-- Enables users to rate decisions and provide comments for quality improvement

-- =============================================================================
-- FEEDBACK RATING TYPE
-- =============================================================================
-- Using simple positive/negative for initial implementation
-- Can be extended to 5-star rating if needed

CREATE TYPE feedback_rating AS ENUM ('positive', 'negative');

-- =============================================================================
-- DECISION FEEDBACK TABLE
-- =============================================================================
-- Stores user feedback on individual LLM decisions

CREATE TABLE decision_feedback (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    decision_id UUID NOT NULL REFERENCES llm_decisions(id) ON DELETE CASCADE,
    user_id UUID,  -- Optional: NULL for anonymous feedback (API key only)
    rating feedback_rating NOT NULL,
    comment TEXT,  -- Optional user comment explaining the rating
    tags TEXT[],  -- Optional tags for categorization (e.g., 'wrong_direction', 'timing_issue')
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    -- Metadata for analysis
    session_id UUID REFERENCES trading_sessions(id) ON DELETE SET NULL,
    symbol VARCHAR(20),  -- Denormalized for easier querying
    decision_type VARCHAR(50),  -- Denormalized for easier querying
    agent_name VARCHAR(100),  -- Denormalized for easier querying

    -- Prevent duplicate feedback from same user on same decision
    CONSTRAINT unique_user_decision_feedback UNIQUE (decision_id, user_id)
);

-- =============================================================================
-- INDEXES
-- =============================================================================

-- Primary lookup: feedback for a specific decision
CREATE INDEX idx_decision_feedback_decision ON decision_feedback (decision_id);

-- User's feedback history
CREATE INDEX idx_decision_feedback_user ON decision_feedback (user_id) WHERE user_id IS NOT NULL;

-- Analysis: find all negative feedback for review
CREATE INDEX idx_decision_feedback_rating ON decision_feedback (rating, created_at DESC);

-- Analysis: feedback by agent (which agents get bad ratings?)
CREATE INDEX idx_decision_feedback_agent ON decision_feedback (agent_name, rating) WHERE agent_name IS NOT NULL;

-- Analysis: feedback by symbol (which symbols have decision issues?)
CREATE INDEX idx_decision_feedback_symbol ON decision_feedback (symbol, rating) WHERE symbol IS NOT NULL;

-- Analysis: feedback by decision type
CREATE INDEX idx_decision_feedback_type ON decision_feedback (decision_type, rating) WHERE decision_type IS NOT NULL;

-- Time-based analysis
CREATE INDEX idx_decision_feedback_created ON decision_feedback (created_at DESC);

-- =============================================================================
-- TRIGGER: Auto-update updated_at
-- =============================================================================

CREATE OR REPLACE FUNCTION update_decision_feedback_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER decision_feedback_updated_at
    BEFORE UPDATE ON decision_feedback
    FOR EACH ROW
    EXECUTE FUNCTION update_decision_feedback_updated_at();

-- =============================================================================
-- MATERIALIZED VIEW: Feedback Statistics
-- =============================================================================
-- Pre-computed statistics for dashboard and analysis
-- Refresh periodically (e.g., every hour) or on-demand

CREATE MATERIALIZED VIEW decision_feedback_stats AS
SELECT
    -- Overall stats
    COUNT(*) as total_feedback,
    COUNT(*) FILTER (WHERE rating = 'positive') as positive_count,
    COUNT(*) FILTER (WHERE rating = 'negative') as negative_count,
    ROUND(
        COUNT(*) FILTER (WHERE rating = 'positive')::NUMERIC /
        NULLIF(COUNT(*), 0) * 100, 2
    ) as positive_rate,

    -- By agent
    agent_name,

    -- By decision type
    decision_type,

    -- By symbol
    symbol,

    -- Time period
    DATE_TRUNC('day', created_at) as feedback_date
FROM decision_feedback
GROUP BY agent_name, decision_type, symbol, DATE_TRUNC('day', created_at);

CREATE UNIQUE INDEX idx_feedback_stats_unique
ON decision_feedback_stats (agent_name, decision_type, symbol, feedback_date);

-- =============================================================================
-- VIEW: Low-Rated Decisions for Review
-- =============================================================================
-- Decisions with multiple negative ratings that need prompt engineering review

CREATE VIEW decisions_needing_review AS
SELECT
    d.id as decision_id,
    d.agent_name,
    d.decision_type,
    d.symbol,
    d.prompt,
    d.response,
    d.confidence,
    d.outcome,
    d.outcome_pnl,
    d.created_at as decision_created_at,
    COUNT(f.id) as feedback_count,
    COUNT(*) FILTER (WHERE f.rating = 'negative') as negative_count,
    COUNT(*) FILTER (WHERE f.rating = 'positive') as positive_count,
    ARRAY_AGG(f.comment) FILTER (WHERE f.comment IS NOT NULL) as comments,
    ARRAY_AGG(DISTINCT unnest_tags.tag) FILTER (WHERE unnest_tags.tag IS NOT NULL) as all_tags
FROM llm_decisions d
JOIN decision_feedback f ON d.id = f.decision_id
LEFT JOIN LATERAL unnest(f.tags) AS unnest_tags(tag) ON true
GROUP BY d.id, d.agent_name, d.decision_type, d.symbol, d.prompt, d.response,
         d.confidence, d.outcome, d.outcome_pnl, d.created_at
HAVING COUNT(*) FILTER (WHERE f.rating = 'negative') >= 2
ORDER BY negative_count DESC, d.created_at DESC;

-- =============================================================================
-- COMMENTS
-- =============================================================================

COMMENT ON TABLE decision_feedback IS 'User feedback on LLM trading decisions for quality improvement';
COMMENT ON COLUMN decision_feedback.rating IS 'User rating: positive (helpful) or negative (not helpful)';
COMMENT ON COLUMN decision_feedback.comment IS 'Optional text explanation of the rating';
COMMENT ON COLUMN decision_feedback.tags IS 'Categorization tags like wrong_direction, timing_issue, risk_too_high';
COMMENT ON VIEW decisions_needing_review IS 'Decisions with 2+ negative ratings flagged for prompt engineering review';
COMMENT ON MATERIALIZED VIEW decision_feedback_stats IS 'Pre-computed feedback statistics - refresh with REFRESH MATERIALIZED VIEW';

-- =============================================================================
-- SEED DATA: Common feedback tags
-- =============================================================================
-- These are suggested tags that the UI can offer to users

COMMENT ON TABLE decision_feedback IS
'User feedback on LLM trading decisions.
Common tags: wrong_direction, bad_timing, risk_too_high, risk_too_low,
missed_opportunity, good_entry, good_exit, accurate_prediction,
unclear_reasoning, helpful_explanation';

-- Update statistics
ANALYZE decision_feedback;

SELECT 'Migration 007: Decision feedback system created successfully!' AS status;
