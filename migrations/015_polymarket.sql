-- Polymarket paper trading tables

-- =============================================================================
-- POLYMARKET MARKETS
-- =============================================================================

CREATE TABLE polymarket_markets (
    id TEXT PRIMARY KEY,
    question TEXT NOT NULL,
    category TEXT,
    yes_price DECIMAL(10, 4),
    no_price DECIMAL(10, 4),
    volume DECIMAL(20, 2),
    end_date TIMESTAMPTZ,
    active BOOLEAN DEFAULT TRUE,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_polymarket_markets_active ON polymarket_markets (active) WHERE active = TRUE;
CREATE INDEX idx_polymarket_markets_category ON polymarket_markets (category);

-- =============================================================================
-- POLYMARKET POSITIONS
-- =============================================================================

CREATE TABLE polymarket_positions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES trading_sessions(id) ON DELETE CASCADE,
    market_id TEXT REFERENCES polymarket_markets(id) ON DELETE CASCADE,
    side TEXT NOT NULL,
    shares DECIMAL(20, 8) NOT NULL DEFAULT 0,
    avg_price DECIMAL(10, 8) NOT NULL DEFAULT 0,
    cost_basis DECIMAL(20, 8) NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'OPEN',
    opened_at TIMESTAMPTZ DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    realized_pnl DECIMAL(20, 8),
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_polymarket_positions_session ON polymarket_positions (session_id);
CREATE INDEX idx_polymarket_positions_market ON polymarket_positions (market_id);
CREATE INDEX idx_polymarket_positions_status ON polymarket_positions (status) WHERE status = 'OPEN';

-- =============================================================================
-- POLYMARKET TRADES
-- =============================================================================

CREATE TABLE polymarket_trades (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    position_id UUID REFERENCES polymarket_positions(id) ON DELETE CASCADE,
    market_id TEXT REFERENCES polymarket_markets(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    side TEXT NOT NULL,
    amount DECIMAL(20, 8) NOT NULL,
    price DECIMAL(10, 8) NOT NULL,
    shares DECIMAL(20, 8) NOT NULL,
    timestamp TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_polymarket_trades_position ON polymarket_trades (position_id);
CREATE INDEX idx_polymarket_trades_market ON polymarket_trades (market_id);
CREATE INDEX idx_polymarket_trades_timestamp ON polymarket_trades (timestamp DESC);

-- =============================================================================
-- POLYMARKET PREDICTIONS
-- =============================================================================

CREATE TABLE polymarket_predictions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    market_id TEXT REFERENCES polymarket_markets(id) ON DELETE CASCADE,
    predicted_prob DECIMAL(10, 8),
    market_price DECIMAL(10, 8),
    edge DECIMAL(10, 8),
    confidence DECIMAL(10, 8),
    category TEXT,
    reasoning TEXT,
    outcome DECIMAL(10, 8),
    resolved BOOLEAN DEFAULT FALSE,
    pnl DECIMAL(20, 8),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_polymarket_predictions_market ON polymarket_predictions (market_id);
CREATE INDEX idx_polymarket_predictions_resolved ON polymarket_predictions (resolved);
CREATE INDEX idx_polymarket_predictions_created ON polymarket_predictions (created_at DESC);

-- =============================================================================
-- TRIGGERS
-- =============================================================================

CREATE TRIGGER update_polymarket_positions_updated_at BEFORE UPDATE ON polymarket_positions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

SELECT 'Polymarket tables created successfully!' AS status;
