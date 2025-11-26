-- CryptoFunk Initial Database Schema
-- TimescaleDB Extension for time-series data
-- pgvector Extension for semantic search

-- =============================================================================
-- EXTENSIONS
-- =============================================================================

-- Enable TimescaleDB extension (requires timescale/timescaledb Docker image)
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- Enable pgvector extension for semantic search
CREATE EXTENSION IF NOT EXISTS vector;

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================================================
-- ENUMS
-- =============================================================================

CREATE TYPE order_side AS ENUM ('BUY', 'SELL');
CREATE TYPE order_type AS ENUM ('MARKET', 'LIMIT', 'STOP_LOSS', 'STOP_LOSS_LIMIT', 'TAKE_PROFIT', 'TAKE_PROFIT_LIMIT');
CREATE TYPE order_status AS ENUM ('NEW', 'PARTIALLY_FILLED', 'FILLED', 'CANCELED', 'REJECTED', 'EXPIRED');
CREATE TYPE position_side AS ENUM ('LONG', 'SHORT', 'FLAT');
CREATE TYPE signal_type AS ENUM ('BUY', 'SELL', 'HOLD');
CREATE TYPE trading_mode AS ENUM ('PAPER', 'LIVE');
CREATE TYPE agent_state_type AS ENUM ('STARTING', 'RUNNING', 'STOPPED', 'ERROR');

-- =============================================================================
-- CANDLESTICK DATA (TimescaleDB Hypertable)
-- =============================================================================

CREATE TABLE candlesticks (
    id BIGSERIAL,
    symbol VARCHAR(20) NOT NULL,
    exchange VARCHAR(50) NOT NULL DEFAULT 'binance',
    interval VARCHAR(10) NOT NULL,  -- 1m, 5m, 15m, 1h, 4h, 1d
    open_time TIMESTAMPTZ NOT NULL,
    close_time TIMESTAMPTZ NOT NULL,
    open DECIMAL(20, 8) NOT NULL,
    high DECIMAL(20, 8) NOT NULL,
    low DECIMAL(20, 8) NOT NULL,
    close DECIMAL(20, 8) NOT NULL,
    volume DECIMAL(30, 8) NOT NULL,
    quote_volume DECIMAL(30, 8),
    trades_count INTEGER,
    taker_buy_base_volume DECIMAL(30, 8),
    taker_buy_quote_volume DECIMAL(30, 8),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (symbol, exchange, interval, open_time)
);

-- Convert to TimescaleDB hypertable
SELECT create_hypertable('candlesticks', 'open_time', 
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Create indexes for candlesticks
CREATE INDEX idx_candlesticks_symbol_time ON candlesticks (symbol, open_time DESC);
CREATE INDEX idx_candlesticks_exchange_symbol ON candlesticks (exchange, symbol, open_time DESC);

-- Enable compression (compress data older than 7 days)
ALTER TABLE candlesticks SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'symbol, exchange, interval'
);

SELECT add_compression_policy('candlesticks', INTERVAL '7 days', if_not_exists => TRUE);

-- =============================================================================
-- TRADING SESSIONS
-- =============================================================================

CREATE TABLE trading_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    mode trading_mode NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    exchange VARCHAR(50) NOT NULL DEFAULT 'binance',
    started_at TIMESTAMPTZ DEFAULT NOW(),
    stopped_at TIMESTAMPTZ,
    initial_capital DECIMAL(20, 8) NOT NULL,
    final_capital DECIMAL(20, 8),
    total_trades INTEGER DEFAULT 0,
    winning_trades INTEGER DEFAULT 0,
    losing_trades INTEGER DEFAULT 0,
    total_pnl DECIMAL(20, 8) DEFAULT 0,
    max_drawdown DECIMAL(20, 8) DEFAULT 0,
    sharpe_ratio DECIMAL(10, 4),
    config JSONB,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_trading_sessions_symbol ON trading_sessions (symbol, started_at DESC);
CREATE INDEX idx_trading_sessions_mode ON trading_sessions (mode, started_at DESC);

-- =============================================================================
-- POSITIONS
-- =============================================================================

CREATE TABLE positions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES trading_sessions(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    exchange VARCHAR(50) NOT NULL DEFAULT 'binance',
    side position_side NOT NULL,
    entry_price DECIMAL(20, 8) NOT NULL,
    exit_price DECIMAL(20, 8),
    quantity DECIMAL(30, 8) NOT NULL,
    entry_time TIMESTAMPTZ DEFAULT NOW(),
    exit_time TIMESTAMPTZ,
    stop_loss DECIMAL(20, 8),
    take_profit DECIMAL(20, 8),
    realized_pnl DECIMAL(20, 8),
    unrealized_pnl DECIMAL(20, 8),
    fees DECIMAL(20, 8) DEFAULT 0,
    entry_reason TEXT,
    exit_reason TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_positions_session ON positions (session_id, entry_time DESC);
CREATE INDEX idx_positions_symbol ON positions (symbol, entry_time DESC);
CREATE INDEX idx_positions_open ON positions (exit_time) WHERE exit_time IS NULL;

-- =============================================================================
-- ORDERS
-- =============================================================================

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES trading_sessions(id) ON DELETE CASCADE,
    position_id UUID REFERENCES positions(id) ON DELETE SET NULL,
    exchange_order_id VARCHAR(100),
    symbol VARCHAR(20) NOT NULL,
    exchange VARCHAR(50) NOT NULL DEFAULT 'binance',
    side order_side NOT NULL,
    type order_type NOT NULL,
    status order_status NOT NULL,
    price DECIMAL(20, 8),
    stop_price DECIMAL(20, 8),
    quantity DECIMAL(30, 8) NOT NULL,
    executed_quantity DECIMAL(30, 8) DEFAULT 0,
    executed_quote_quantity DECIMAL(30, 8) DEFAULT 0,
    time_in_force VARCHAR(10),
    placed_at TIMESTAMPTZ DEFAULT NOW(),
    filled_at TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ,
    error_message TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_orders_session ON orders (session_id, placed_at DESC);
CREATE INDEX idx_orders_symbol ON orders (symbol, placed_at DESC);
CREATE INDEX idx_orders_status ON orders (status, placed_at DESC);
CREATE INDEX idx_orders_exchange_id ON orders (exchange_order_id) WHERE exchange_order_id IS NOT NULL;

-- =============================================================================
-- TRADES (Fills)
-- =============================================================================

CREATE TABLE trades (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID REFERENCES orders(id) ON DELETE CASCADE,
    exchange_trade_id VARCHAR(100),
    symbol VARCHAR(20) NOT NULL,
    exchange VARCHAR(50) NOT NULL DEFAULT 'binance',
    side order_side NOT NULL,
    price DECIMAL(20, 8) NOT NULL,
    quantity DECIMAL(30, 8) NOT NULL,
    quote_quantity DECIMAL(30, 8) NOT NULL,
    commission DECIMAL(20, 8) DEFAULT 0,
    commission_asset VARCHAR(10),
    executed_at TIMESTAMPTZ NOT NULL,
    is_maker BOOLEAN DEFAULT FALSE,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_trades_order ON trades (order_id, executed_at DESC);
CREATE INDEX idx_trades_symbol ON trades (symbol, executed_at DESC);
CREATE INDEX idx_trades_exchange_id ON trades (exchange_trade_id) WHERE exchange_trade_id IS NOT NULL;

-- =============================================================================
-- AGENT SIGNALS
-- =============================================================================

CREATE TABLE agent_signals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES trading_sessions(id) ON DELETE CASCADE,
    agent_name VARCHAR(100) NOT NULL,
    agent_type VARCHAR(50) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    signal signal_type NOT NULL,
    confidence DECIMAL(5, 4) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    reasoning TEXT,
    context JSONB,
    indicators JSONB,
    llm_prompt TEXT,
    llm_response TEXT,
    llm_model VARCHAR(100),
    llm_tokens_used INTEGER,
    processing_time_ms INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_agent_signals_session ON agent_signals (session_id, created_at DESC);
CREATE INDEX idx_agent_signals_symbol ON agent_signals (symbol, created_at DESC);
CREATE INDEX idx_agent_signals_agent ON agent_signals (agent_name, created_at DESC);

-- =============================================================================
-- AGENT STATUS
-- =============================================================================

CREATE TABLE agent_status (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_name VARCHAR(100) UNIQUE NOT NULL,
    agent_type VARCHAR(50) NOT NULL,
    status agent_state_type NOT NULL,
    pid INTEGER,
    started_at TIMESTAMPTZ,
    last_heartbeat TIMESTAMPTZ,
    total_signals INTEGER DEFAULT 0,
    avg_confidence DECIMAL(5, 4),
    error_count INTEGER DEFAULT 0,
    last_error TEXT,
    config JSONB,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_agent_status_type ON agent_status (agent_type, status);

-- =============================================================================
-- LLM DECISIONS (Explainability & Learning)
-- =============================================================================

CREATE TABLE llm_decisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES trading_sessions(id) ON DELETE CASCADE,
    decision_type VARCHAR(50) NOT NULL,  -- 'signal', 'risk_approval', 'position_sizing', etc.
    symbol VARCHAR(20) NOT NULL,
    prompt TEXT NOT NULL,
    prompt_embedding vector(1536),  -- For semantic search (OpenAI embeddings)
    response TEXT NOT NULL,
    model VARCHAR(100) NOT NULL,
    tokens_used INTEGER,
    latency_ms INTEGER,
    cost_usd DECIMAL(10, 6),
    cached BOOLEAN DEFAULT FALSE,
    outcome VARCHAR(50),  -- 'profitable', 'loss', 'pending', etc.
    outcome_pnl DECIMAL(20, 8),
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_llm_decisions_session ON llm_decisions (session_id, created_at DESC);
CREATE INDEX idx_llm_decisions_symbol ON llm_decisions (symbol, created_at DESC);
CREATE INDEX idx_llm_decisions_type ON llm_decisions (decision_type, created_at DESC);
CREATE INDEX idx_llm_decisions_outcome ON llm_decisions (outcome) WHERE outcome IS NOT NULL;

-- Vector similarity search index
CREATE INDEX idx_llm_decisions_embedding ON llm_decisions USING ivfflat (prompt_embedding vector_cosine_ops)
WITH (lists = 100);

-- =============================================================================
-- PERFORMANCE METRICS
-- =============================================================================

CREATE TABLE performance_metrics (
    id UUID DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES trading_sessions(id) ON DELETE CASCADE,
    metric_time TIMESTAMPTZ NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    total_value DECIMAL(20, 8) NOT NULL,
    cash_balance DECIMAL(20, 8) NOT NULL,
    position_value DECIMAL(20, 8) NOT NULL,
    unrealized_pnl DECIMAL(20, 8),
    realized_pnl DECIMAL(20, 8),
    total_pnl DECIMAL(20, 8),
    win_rate DECIMAL(5, 4),
    sharpe_ratio DECIMAL(10, 4),
    max_drawdown DECIMAL(20, 8),
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (id, metric_time)
);

-- Convert to hypertable
SELECT create_hypertable('performance_metrics', 'metric_time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

CREATE INDEX idx_performance_metrics_session ON performance_metrics (session_id, metric_time DESC);

-- =============================================================================
-- MARKET DATA CACHE
-- =============================================================================

CREATE TABLE market_data_cache (
    symbol VARCHAR(20) NOT NULL,
    exchange VARCHAR(50) NOT NULL DEFAULT 'binance',
    data_type VARCHAR(50) NOT NULL,  -- 'ticker', 'orderbook', 'trades'
    data JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (symbol, exchange, data_type)
);

CREATE INDEX idx_market_data_cache_expires ON market_data_cache (expires_at);

-- Cleanup old cache entries (TTL)
CREATE OR REPLACE FUNCTION cleanup_market_data_cache()
RETURNS void AS $$
BEGIN
    DELETE FROM market_data_cache WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- TRIGGERS
-- =============================================================================

-- Update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_trading_sessions_updated_at BEFORE UPDATE ON trading_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_positions_updated_at BEFORE UPDATE ON positions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agent_status_updated_at BEFORE UPDATE ON agent_status
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- VIEWS
-- =============================================================================

-- Active positions view
CREATE VIEW active_positions AS
SELECT * FROM positions
WHERE exit_time IS NULL
ORDER BY entry_time DESC;

-- Open orders view
CREATE VIEW open_orders AS
SELECT * FROM orders
WHERE status IN ('NEW', 'PARTIALLY_FILLED')
ORDER BY placed_at DESC;

-- Daily performance summary
CREATE VIEW daily_performance AS
SELECT 
    DATE(metric_time) as date,
    session_id,
    symbol,
    MAX(total_value) as final_value,
    MIN(total_value) as min_value,
    MAX(total_value) - MIN(total_value) as intraday_change,
    SUM(realized_pnl) as total_realized_pnl
FROM performance_metrics
GROUP BY DATE(metric_time), session_id, symbol
ORDER BY date DESC;

-- =============================================================================
-- INITIAL DATA
-- =============================================================================

-- Insert default agent status entries (will be updated by agents)
INSERT INTO agent_status (agent_name, agent_type, status) VALUES
    ('technical-analyst', 'analysis', 'STOPPED'),
    ('orderbook-analyst', 'analysis', 'STOPPED'),
    ('sentiment-analyst', 'analysis', 'STOPPED'),
    ('trend-follower', 'strategy', 'STOPPED'),
    ('mean-reversion', 'strategy', 'STOPPED'),
    ('risk-manager', 'risk', 'STOPPED')
ON CONFLICT (agent_name) DO NOTHING;

-- =============================================================================
-- COMMENTS
-- =============================================================================

COMMENT ON TABLE candlesticks IS 'OHLCV candlestick data (TimescaleDB hypertable)';
COMMENT ON TABLE trading_sessions IS 'Trading sessions with performance metrics';
COMMENT ON TABLE positions IS 'Trading positions (long/short)';
COMMENT ON TABLE orders IS 'Exchange orders (buy/sell)';
COMMENT ON TABLE trades IS 'Executed trades (order fills)';
COMMENT ON TABLE agent_signals IS 'Agent trading signals with LLM reasoning';
COMMENT ON TABLE agent_status IS 'Real-time agent health and status';
COMMENT ON TABLE llm_decisions IS 'LLM decision history for explainability and learning';
COMMENT ON TABLE performance_metrics IS 'Time-series performance metrics (TimescaleDB)';

-- =============================================================================
-- GRANTS
-- =============================================================================

-- Grant appropriate permissions (adjust for production)
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO postgres;

-- =============================================================================
-- COMPLETED
-- =============================================================================
-- Note: schema_version table is managed by the migration system (internal/db/migrate.go)
-- Do not create schema_version here as the migrator handles it automatically.

SELECT 'Schema created successfully!' AS status;
