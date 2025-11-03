-- Migration 003: Procedural Memory System
-- Phase 10: Advanced Features (T202)
--
-- This migration creates tables for storing learned policies and skills

-- Create procedural_memory_policies table
CREATE TABLE IF NOT EXISTS procedural_memory_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Policy metadata
    type VARCHAR(50) NOT NULL CHECK (type IN ('entry', 'exit', 'sizing', 'risk', 'hedging', 'rebalancing')),
    name VARCHAR(200) NOT NULL,
    description TEXT,

    -- Policy definition
    conditions JSONB NOT NULL,   -- When to apply this policy
    actions JSONB NOT NULL,      -- What actions to take
    parameters JSONB,            -- Configurable parameters

    -- Performance tracking
    times_applied INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    avg_pnl FLOAT NOT NULL DEFAULT 0.0,
    total_pnl FLOAT NOT NULL DEFAULT 0.0,
    sharpe FLOAT NOT NULL DEFAULT 0.0,
    max_drawdown FLOAT NOT NULL DEFAULT 0.0,
    win_rate FLOAT NOT NULL DEFAULT 0.0 CHECK (win_rate >= 0.0 AND win_rate <= 1.0),

    -- Learning metadata
    agent_name VARCHAR(100) NOT NULL,
    symbol VARCHAR(20),          -- Associated symbol (if specific to one)
    learned_from VARCHAR(50) NOT NULL, -- 'backtest', 'live_trading', 'manual'
    source_id UUID,
    confidence FLOAT NOT NULL DEFAULT 0.5 CHECK (confidence >= 0.0 AND confidence <= 1.0),
    is_active BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL DEFAULT 0, -- Higher priority policies are evaluated first

    -- Temporal
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_applied TIMESTAMP,
    last_modified TIMESTAMP
);

-- Create procedural_memory_skills table
CREATE TABLE IF NOT EXISTS procedural_memory_skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Skill metadata
    type VARCHAR(50) NOT NULL CHECK (type IN (
        'technical_analysis', 'orderbook_analysis', 'sentiment_analysis',
        'trend_following', 'mean_reversion', 'risk_management'
    )),
    name VARCHAR(200) NOT NULL,
    description TEXT,

    -- Skill definition
    implementation JSONB NOT NULL,   -- How to execute this skill
    parameters JSONB,                -- Configurable parameters
    prerequisites JSONB,             -- Required conditions/resources

    -- Performance tracking
    times_used INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    avg_duration FLOAT NOT NULL DEFAULT 0.0,  -- Average execution duration (ms)
    avg_accuracy FLOAT NOT NULL DEFAULT 0.0 CHECK (avg_accuracy >= 0.0 AND avg_accuracy <= 1.0),

    -- Learning metadata
    agent_name VARCHAR(100) NOT NULL,
    learned_from VARCHAR(50) NOT NULL, -- 'training', 'observation', 'manual'
    source_id UUID,
    proficiency FLOAT NOT NULL DEFAULT 0.5 CHECK (proficiency >= 0.0 AND proficiency <= 1.0),
    is_active BOOLEAN NOT NULL DEFAULT true,

    -- Temporal
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used TIMESTAMP
);

-- Indexes for efficient querying

-- Policies indexes
CREATE INDEX IF NOT EXISTS idx_procedural_policies_type
    ON procedural_memory_policies (type);

CREATE INDEX IF NOT EXISTS idx_procedural_policies_agent
    ON procedural_memory_policies (agent_name);

CREATE INDEX IF NOT EXISTS idx_procedural_policies_symbol
    ON procedural_memory_policies (symbol)
    WHERE symbol IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_procedural_policies_active
    ON procedural_memory_policies (is_active, priority DESC)
    WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_procedural_policies_performance
    ON procedural_memory_policies (sharpe DESC, avg_pnl DESC)
    WHERE is_active = true AND times_applied >= 5;

CREATE INDEX IF NOT EXISTS idx_procedural_policies_created_at
    ON procedural_memory_policies (created_at DESC);

-- JSONB indexes for policy queries
CREATE INDEX IF NOT EXISTS idx_procedural_policies_conditions
    ON procedural_memory_policies USING gin (conditions);

CREATE INDEX IF NOT EXISTS idx_procedural_policies_actions
    ON procedural_memory_policies USING gin (actions);

-- Skills indexes
CREATE INDEX IF NOT EXISTS idx_procedural_skills_type
    ON procedural_memory_skills (type);

CREATE INDEX IF NOT EXISTS idx_procedural_skills_agent
    ON procedural_memory_skills (agent_name);

CREATE INDEX IF NOT EXISTS idx_procedural_skills_active
    ON procedural_memory_skills (is_active, proficiency DESC)
    WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_procedural_skills_proficiency
    ON procedural_memory_skills (proficiency DESC)
    WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_procedural_skills_created_at
    ON procedural_memory_skills (created_at DESC);

-- JSONB index for skill implementation queries
CREATE INDEX IF NOT EXISTS idx_procedural_skills_implementation
    ON procedural_memory_skills USING gin (implementation);

-- Comments for documentation
COMMENT ON TABLE procedural_memory_policies IS 'Stores learned trading policies and rules';
COMMENT ON COLUMN procedural_memory_policies.conditions IS 'JSONB conditions defining when to apply this policy';
COMMENT ON COLUMN procedural_memory_policies.actions IS 'JSONB actions to take when policy is triggered';
COMMENT ON COLUMN procedural_memory_policies.sharpe IS 'Sharpe ratio measuring risk-adjusted returns';
COMMENT ON COLUMN procedural_memory_policies.priority IS 'Priority for policy evaluation (higher = evaluated first)';

COMMENT ON TABLE procedural_memory_skills IS 'Stores agent skills and capabilities';
COMMENT ON COLUMN procedural_memory_skills.implementation IS 'JSONB describing how to execute this skill';
COMMENT ON COLUMN procedural_memory_skills.proficiency IS 'Agent proficiency level in this skill (0.0 to 1.0)';
COMMENT ON COLUMN procedural_memory_skills.avg_duration IS 'Average execution duration in milliseconds';

-- Insert sample policies for testing
INSERT INTO procedural_memory_policies (
    type, name, description, conditions, actions, parameters,
    agent_name, learned_from, confidence, priority, created_at
) VALUES
    (
        'entry',
        'Trend Following Entry',
        'Enter long position when EMA crossover occurs with RSI confirmation',
        '{"ema_fast_above_slow": true, "rsi": {"min": 40, "max": 70}, "volume_ratio": {"min": 1.2}}'::jsonb,
        '{"action": "enter_long", "position_size": "kelly_criterion", "stop_loss": 0.02}'::jsonb,
        '{"ema_fast": 12, "ema_slow": 26, "rsi_period": 14}'::jsonb,
        'trend-agent',
        'backtest',
        0.85,
        10,
        NOW() - INTERVAL '30 days'
    ),
    (
        'exit',
        'Profit Target Exit',
        'Exit position when profit target is reached or trailing stop is hit',
        '{"profit_target_reached": true}'::jsonb,
        '{"action": "exit_position", "exit_type": "limit_order"}'::jsonb,
        '{"profit_target": 0.05, "trailing_stop": 0.02}'::jsonb,
        'trend-agent',
        'backtest',
        0.9,
        15,
        NOW() - INTERVAL '25 days'
    ),
    (
        'risk',
        'Drawdown Circuit Breaker',
        'Halt trading when drawdown exceeds threshold',
        '{"drawdown": {"min": 0.15}}'::jsonb,
        '{"action": "halt_trading", "notify": true}'::jsonb,
        '{"max_drawdown": 0.15, "cooldown_hours": 24}'::jsonb,
        'risk-agent',
        'manual',
        0.95,
        100,
        NOW() - INTERVAL '20 days'
    ),
    (
        'sizing',
        'Kelly Criterion Position Sizing',
        'Calculate optimal position size using Kelly Criterion',
        '{"strategy": "kelly"}'::jsonb,
        '{"action": "calculate_position_size", "method": "kelly_criterion"}'::jsonb,
        '{"kelly_fraction": 0.25, "max_position": 0.1}'::jsonb,
        'risk-agent',
        'backtest',
        0.8,
        20,
        NOW() - INTERVAL '15 days'
    )
ON CONFLICT (id) DO NOTHING;

-- Insert sample skills for testing
INSERT INTO procedural_memory_skills (
    type, name, description, implementation, parameters,
    agent_name, learned_from, proficiency, created_at
) VALUES
    (
        'technical_analysis',
        'RSI Divergence Detection',
        'Detect bullish and bearish RSI divergences',
        '{"steps": ["calculate_rsi", "identify_price_peaks", "identify_rsi_peaks", "compare_peaks"]}'::jsonb,
        '{"rsi_period": 14, "lookback_bars": 50}'::jsonb,
        'technical-agent',
        'training',
        0.85,
        NOW() - INTERVAL '60 days'
    ),
    (
        'orderbook_analysis',
        'Support/Resistance Detection',
        'Identify support and resistance levels from order book',
        '{"steps": ["fetch_orderbook", "cluster_orders", "calculate_liquidity", "identify_levels"]}'::jsonb,
        '{"depth": 20, "cluster_threshold": 0.005}'::jsonb,
        'orderbook-agent',
        'training',
        0.75,
        NOW() - INTERVAL '50 days'
    ),
    (
        'risk_management',
        'VaR Calculation',
        'Calculate Value at Risk using historical simulation',
        '{"steps": ["fetch_historical_returns", "sort_returns", "calculate_percentile"]}'::jsonb,
        '{"confidence_level": 0.95, "lookback_days": 30}'::jsonb,
        'risk-agent',
        'training',
        0.9,
        NOW() - INTERVAL '40 days'
    )
ON CONFLICT (id) DO NOTHING;
