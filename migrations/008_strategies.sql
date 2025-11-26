-- Migration: 008_strategies.sql
-- Description: Create strategies table for persistent strategy storage
-- Created: 2025-11-26

-- Create strategies table for persisting trading strategy configurations
CREATE TABLE IF NOT EXISTS strategies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    schema_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    config JSONB NOT NULL,
    is_active BOOLEAN DEFAULT FALSE,
    author VARCHAR(255),
    version VARCHAR(20) DEFAULT '1.0.0',
    tags TEXT[],
    source VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure only one active strategy at a time
    CONSTRAINT strategies_single_active EXCLUDE USING btree (is_active WITH =) WHERE (is_active = TRUE)
);

-- Create indexes for efficient querying
CREATE INDEX idx_strategies_name ON strategies(name);
CREATE INDEX idx_strategies_is_active ON strategies(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_strategies_created_at ON strategies(created_at DESC);
CREATE INDEX idx_strategies_updated_at ON strategies(updated_at DESC);
CREATE INDEX idx_strategies_tags ON strategies USING GIN(tags);
CREATE INDEX idx_strategies_schema_version ON strategies(schema_version);

-- Index for searching config JSONB
CREATE INDEX idx_strategies_config ON strategies USING GIN(config jsonb_path_ops);

-- Add comments
COMMENT ON TABLE strategies IS 'Persisted trading strategy configurations';
COMMENT ON COLUMN strategies.id IS 'Unique identifier for the strategy';
COMMENT ON COLUMN strategies.name IS 'Human-readable strategy name';
COMMENT ON COLUMN strategies.description IS 'Strategy description';
COMMENT ON COLUMN strategies.schema_version IS 'Schema version for compatibility checking';
COMMENT ON COLUMN strategies.config IS 'Full strategy configuration as JSONB';
COMMENT ON COLUMN strategies.is_active IS 'Whether this is the currently active strategy';
COMMENT ON COLUMN strategies.author IS 'Strategy author/creator';
COMMENT ON COLUMN strategies.version IS 'Strategy version number';
COMMENT ON COLUMN strategies.tags IS 'Tags for categorization';
COMMENT ON COLUMN strategies.source IS 'Where the strategy was imported from';
COMMENT ON COLUMN strategies.created_at IS 'When the strategy was created';
COMMENT ON COLUMN strategies.updated_at IS 'When the strategy was last updated';

-- Create strategy_history table for versioning
CREATE TABLE IF NOT EXISTS strategy_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    strategy_id UUID NOT NULL REFERENCES strategies(id) ON DELETE CASCADE,
    config JSONB NOT NULL,
    version VARCHAR(20),
    changed_by VARCHAR(255),
    change_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_strategy_history_strategy_id ON strategy_history(strategy_id);
CREATE INDEX idx_strategy_history_created_at ON strategy_history(created_at DESC);

COMMENT ON TABLE strategy_history IS 'Version history for strategies';
COMMENT ON COLUMN strategy_history.strategy_id IS 'Reference to the parent strategy';
COMMENT ON COLUMN strategy_history.config IS 'Strategy configuration at this version';
COMMENT ON COLUMN strategy_history.changed_by IS 'User who made the change';
COMMENT ON COLUMN strategy_history.change_reason IS 'Reason for the change';
