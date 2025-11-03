-- Migration 002: Semantic Memory System
-- Phase 10: Advanced Features (T201)
--
-- This migration creates the semantic_memory table for storing knowledge items
-- with vector embeddings for semantic similarity search.

-- Create semantic_memory table
CREATE TABLE IF NOT EXISTS semantic_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Knowledge metadata
    type VARCHAR(50) NOT NULL CHECK (type IN ('fact', 'pattern', 'experience', 'strategy', 'risk')),
    content TEXT NOT NULL,
    embedding vector(1536),  -- 1536-dim embeddings (OpenAI/custom)
    confidence FLOAT NOT NULL DEFAULT 0.5 CHECK (confidence >= 0.0 AND confidence <= 1.0),
    importance FLOAT NOT NULL DEFAULT 0.5 CHECK (importance >= 0.0 AND importance <= 1.0),
    access_count INTEGER NOT NULL DEFAULT 0,

    -- Provenance
    source VARCHAR(100) NOT NULL,  -- 'llm_decision', 'manual', 'backtest', 'pattern_extraction'
    source_id UUID,                 -- ID of source (decision ID, backtest ID, etc.)
    agent_name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20),             -- Associated symbol (if applicable)
    context JSONB,                  -- Additional context (market conditions, etc.)

    -- Validation tracking
    validation_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    last_validated TIMESTAMP,

    -- Temporal
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP,  -- Optional expiration for time-sensitive knowledge

    -- Foreign key constraint to llm_decisions if source is a decision
    CONSTRAINT fk_source_decision
        FOREIGN KEY (source_id)
        REFERENCES llm_decisions(id)
        ON DELETE SET NULL
);

-- Indexes for efficient querying

-- Vector similarity search using cosine distance
CREATE INDEX IF NOT EXISTS idx_semantic_memory_embedding
    ON semantic_memory
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- Query by type
CREATE INDEX IF NOT EXISTS idx_semantic_memory_type
    ON semantic_memory (type);

-- Query by agent
CREATE INDEX IF NOT EXISTS idx_semantic_memory_agent
    ON semantic_memory (agent_name);

-- Query by symbol
CREATE INDEX IF NOT EXISTS idx_semantic_memory_symbol
    ON semantic_memory (symbol)
    WHERE symbol IS NOT NULL;

-- Query by source
CREATE INDEX IF NOT EXISTS idx_semantic_memory_source
    ON semantic_memory (source, source_id);

-- Query by importance (for retrieving most relevant knowledge)
CREATE INDEX IF NOT EXISTS idx_semantic_memory_importance
    ON semantic_memory (importance DESC, confidence DESC);

-- Query by recency
CREATE INDEX IF NOT EXISTS idx_semantic_memory_created_at
    ON semantic_memory (created_at DESC);

-- Query by expiration (for pruning)
CREATE INDEX IF NOT EXISTS idx_semantic_memory_expires_at
    ON semantic_memory (expires_at)
    WHERE expires_at IS NOT NULL;

-- Composite index for common queries
CREATE INDEX IF NOT EXISTS idx_semantic_memory_type_agent
    ON semantic_memory (type, agent_name, created_at DESC);

-- JSONB index for context queries
CREATE INDEX IF NOT EXISTS idx_semantic_memory_context
    ON semantic_memory USING gin (context);

-- Comments for documentation
COMMENT ON TABLE semantic_memory IS 'Stores agent knowledge items with vector embeddings for semantic similarity search';
COMMENT ON COLUMN semantic_memory.embedding IS '1536-dimensional vector embedding for semantic similarity search using cosine distance';
COMMENT ON COLUMN semantic_memory.confidence IS 'Confidence level in this knowledge (0.0 to 1.0)';
COMMENT ON COLUMN semantic_memory.importance IS 'Importance of this knowledge for retrieval prioritization (0.0 to 1.0)';
COMMENT ON COLUMN semantic_memory.context IS 'JSONB context including market conditions, indicators, and other metadata';
COMMENT ON COLUMN semantic_memory.validation_count IS 'Number of times this knowledge has been validated through use';
COMMENT ON COLUMN semantic_memory.success_count IS 'Number of successful validations';
COMMENT ON COLUMN semantic_memory.failure_count IS 'Number of failed validations';
COMMENT ON COLUMN semantic_memory.expires_at IS 'Optional expiration timestamp for time-sensitive knowledge';

-- Insert sample knowledge items for testing
INSERT INTO semantic_memory (
    type, content, confidence, importance, source, agent_name, context, created_at
) VALUES
    (
        'pattern',
        'When RSI exceeds 70 and volume decreases, price typically corrects within 24-48 hours',
        0.85,
        0.9,
        'pattern_extraction',
        'technical-agent',
        '{"indicators": {"rsi_threshold": 70, "volume_trend": "decreasing"}, "timeframe": "24-48h"}'::jsonb,
        NOW() - INTERVAL '10 days'
    ),
    (
        'risk',
        'Maximum drawdown exceeding 15% indicates strategy failure and requires intervention',
        0.95,
        1.0,
        'manual',
        'risk-agent',
        '{"threshold": 0.15, "action": "halt_trading"}'::jsonb,
        NOW() - INTERVAL '5 days'
    ),
    (
        'strategy',
        'Mean reversion strategies perform better in ranging markets (ATR < 2% of price)',
        0.75,
        0.8,
        'backtest',
        'reversion-agent',
        '{"market_condition": "ranging", "atr_threshold": 0.02}'::jsonb,
        NOW() - INTERVAL '15 days'
    ),
    (
        'experience',
        'Stop losses at 2% provide better risk-reward ratio than 5% for volatile assets',
        0.8,
        0.85,
        'llm_decision',
        'risk-agent',
        '{"stop_loss_tight": 0.02, "stop_loss_wide": 0.05, "asset_type": "volatile"}'::jsonb,
        NOW() - INTERVAL '7 days'
    ),
    (
        'fact',
        'BTC price exhibits increased volatility 30 days before and after halving events',
        0.9,
        0.7,
        'manual',
        'technical-agent',
        '{"event": "halving", "volatility_window": "±30 days"}'::jsonb,
        NOW() - INTERVAL '20 days'
    )
ON CONFLICT (id) DO NOTHING;
