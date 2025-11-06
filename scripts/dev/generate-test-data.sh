#!/usr/bin/env bash
#
# generate-test-data.sh - Generate test data for development
#
# This script populates the database with realistic test data:
# - Trading sessions
# - Positions (open and closed)
# - Orders and trades
# - Agent signals
# - LLM decisions
#
# Usage:
#   ./scripts/dev/generate-test-data.sh
#   ./scripts/dev/generate-test-data.sh --clean  # Clean first, then seed

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Database configuration
DB_HOST="${DATABASE_HOST:-localhost}"
DB_PORT="${DATABASE_PORT:-5432}"
DB_USER="${DATABASE_USER:-postgres}"
DB_PASS="${DATABASE_PASSWORD:-postgres}"
DB_NAME="${DATABASE_NAME:-cryptofunk}"

# Parse arguments
CLEAN_FIRST=false
if [[ "${1:-}" == "--clean" ]]; then
    CLEAN_FIRST=true
fi

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}Generating Test Data${NC}"
echo -e "${YELLOW}========================================${NC}"

# Optional: Clean existing data
if [[ "$CLEAN_FIRST" == true ]]; then
    echo -e "${YELLOW}Cleaning existing data...${NC}"
    PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME << SQL
TRUNCATE TABLE trades CASCADE;
TRUNCATE TABLE orders CASCADE;
TRUNCATE TABLE positions CASCADE;
TRUNCATE TABLE agent_signals CASCADE;
TRUNCATE TABLE llm_decisions CASCADE;
TRUNCATE TABLE trading_sessions CASCADE;
TRUNCATE TABLE candlesticks CASCADE;
SQL
    echo -e "${GREEN}✓ Existing data cleaned${NC}"
fi

# Generate test data
echo -e "${YELLOW}Inserting test data...${NC}"

PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME << SQL

-- Create a test trading session
INSERT INTO trading_sessions (id, mode, initial_capital, current_capital, total_pnl, win_rate, total_trades, active_positions, max_drawdown, started_at, status)
VALUES 
    ('11111111-1111-1111-1111-111111111111'::uuid, 'PAPER', 10000.00, 10450.00, 450.00, 0.65, 20, 2, 0.08, NOW() - INTERVAL '7 days', 'ACTIVE'),
    ('22222222-2222-2222-2222-222222222222'::uuid, 'PAPER', 5000.00, 5200.00, 200.00, 0.70, 10, 1, 0.05, NOW() - INTERVAL '3 days', 'ACTIVE');

-- Create some positions (mix of open and closed)
INSERT INTO positions (session_id, symbol, side, entry_price, current_price, quantity, unrealized_pnl, stop_loss, take_profit, status, opened_at)
VALUES 
    ('11111111-1111-1111-1111-111111111111'::uuid, 'BTCUSDT', 'LONG', 45000.00, 46500.00, 0.05, 75.00, 44100.00, 47250.00, 'OPEN', NOW() - INTERVAL '2 days'),
    ('11111111-1111-1111-1111-111111111111'::uuid, 'ETHUSDT', 'LONG', 3000.00, 3100.00, 0.5, 50.00, 2940.00, 3150.00, 'OPEN', NOW() - INTERVAL '1 day');

INSERT INTO positions (session_id, symbol, side, entry_price, exit_price, quantity, realized_pnl, stop_loss, take_profit, status, opened_at, closed_at)
VALUES
    ('11111111-1111-1111-1111-111111111111'::uuid, 'BTCUSDT', 'LONG', 44000.00, 45000.00, 0.1, 100.00, 43120.00, 46200.00, 'CLOSED', NOW() - INTERVAL '5 days', NOW() - INTERVAL '4 days'),
    ('11111111-1111-1111-1111-111111111111'::uuid, 'ETHUSDT', 'SHORT', 3200.00, 3100.00, 0.3, 30.00, 3264.00, 3040.00, 'CLOSED', NOW() - INTERVAL '6 days', NOW() - INTERVAL '5 days'),
    ('22222222-2222-2222-2222-222222222222'::uuid, 'BTCUSDT', 'LONG', 43000.00, 44500.00, 0.08, 120.00, 42140.00, 45150.00, 'CLOSED', NOW() - INTERVAL '3 days', NOW() - INTERVAL '2 days');

-- Create orders for the open positions
INSERT INTO orders (session_id, position_id, symbol, side, type, quantity, price, status, created_at, filled_at)
SELECT 
    session_id,
    id,
    symbol,
    side,
    'MARKET'::order_type,
    quantity,
    entry_price,
    'FILLED'::order_status,
    opened_at,
    opened_at + INTERVAL '2 seconds'
FROM positions
WHERE status = 'OPEN';

-- Create trades for filled orders
INSERT INTO trades (order_id, symbol, side, quantity, price, fee, fee_currency, executed_at)
SELECT
    o.id,
    o.symbol,
    o.side,
    o.quantity,
    o.price,
    o.quantity * o.price * 0.001, -- 0.1% fee
    'USDT',
    o.filled_at
FROM orders o
WHERE o.status = 'FILLED';

-- Create agent signals
INSERT INTO agent_signals (session_id, agent_type, signal_type, symbol, confidence, reasoning, created_at)
VALUES
    ('11111111-1111-1111-1111-111111111111'::uuid, 'technical', 'BUY', 'BTCUSDT', 0.85, 'RSI oversold (28), MACD crossover bullish, strong support at 45k', NOW() - INTERVAL '2 hours'),
    ('11111111-1111-1111-1111-111111111111'::uuid, 'trend', 'BUY', 'BTCUSDT', 0.78, 'Strong uptrend, 20-day MA crossed above 50-day MA', NOW() - INTERVAL '2 hours'),
    ('11111111-1111-1111-1111-111111111111'::uuid, 'risk', 'HOLD', 'BTCUSDT', 0.90, 'Position size within limits, no circuit breakers triggered', NOW() - INTERVAL '2 hours'),
    ('11111111-1111-1111-1111-111111111111'::uuid, 'technical', 'SELL', 'ETHUSDT', 0.72, 'RSI overbought (75), resistance at 3200', NOW() - INTERVAL '1 hour'),
    ('22222222-2222-2222-2222-222222222222'::uuid, 'technical', 'BUY', 'ETHUSDT', 0.80, 'Bollinger Band squeeze, volatility breakout imminent', NOW() - INTERVAL '30 minutes');

-- Create LLM decisions (with synthetic embeddings)
INSERT INTO llm_decisions (session_id, agent_type, decision_type, prompt, response, model_used, confidence, reasoning, metadata, created_at)
VALUES
    ('11111111-1111-1111-1111-111111111111'::uuid, 'technical', 'TRADE_SIGNAL', 'Analyze BTC technical indicators', 'Strong buy signal based on multiple indicators', 'claude-sonnet-4', 0.85, 'Multiple bullish signals: RSI oversold, MACD crossover, support level', '{"rsi": 28, "macd": "bullish", "support": 45000}', NOW() - INTERVAL '2 hours'),
    ('11111111-1111-1111-1111-111111111111'::uuid, 'risk', 'RISK_ASSESSMENT', 'Evaluate portfolio risk', 'Risk within acceptable limits', 'claude-sonnet-4', 0.90, 'Current drawdown 8%, position sizes compliant', '{"drawdown": 0.08, "max_positions": 2}', NOW() - INTERVAL '1 hour'),
    ('22222222-2222-2222-2222-222222222222'::uuid, 'trend', 'MARKET_ANALYSIS', 'Analyze ETH trend', 'Bullish trend continuation expected', 'gpt-4-turbo', 0.75, 'Uptrend intact, higher highs and higher lows', '{"trend": "bullish", "strength": 0.75}', NOW() - INTERVAL '30 minutes');

-- Create some candlestick data for BTC
INSERT INTO candlesticks (symbol, interval, open_time, close_time, open, high, low, close, volume, trades)
SELECT
    'BTCUSDT',
    '1h',
    timestamp,
    timestamp + INTERVAL '1 hour',
    45000 + (random() * 1000 - 500)::numeric,  -- open
    45000 + (random() * 1500)::numeric,         -- high
    45000 - (random() * 1500)::numeric,         -- low
    45000 + (random() * 1000 - 500)::numeric,   -- close
    (random() * 1000000)::numeric,               -- volume
    (random() * 10000)::int                      -- trades
FROM generate_series(
    NOW() - INTERVAL '7 days',
    NOW(),
    INTERVAL '1 hour'
) AS timestamp;

SQL

echo -e "${GREEN}✓ Test data generated successfully${NC}"
echo ""
echo "Generated:"
echo "  - 2 trading sessions"
echo "  - 5 positions (2 open, 3 closed)"
echo "  - Multiple orders and trades"
echo "  - 5 agent signals"
echo "  - 3 LLM decisions"
echo "  - 168 hours of BTC candlestick data"
echo ""
echo "Verify with:"
echo "  psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c 'SELECT * FROM trading_sessions;'"
echo "  psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c 'SELECT * FROM positions;'"
echo ""
