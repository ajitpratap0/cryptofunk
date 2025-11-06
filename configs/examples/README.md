# Example Configurations

This directory contains pre-configured settings for common trading scenarios. Use these as starting points for your own configuration.

## Available Configurations

### 1. Conservative Trading (`conservative.yaml`)

**Best for:** Risk-averse traders, production environments, real capital

**Key Features:**
- **Risk Level:** Very Low
- **Position Size:** Max 5% per trade
- **Stop Loss:** Tight 1.5%
- **Take Profit:** Conservative 3%
- **Confidence Required:** 80% (very high)
- **Max Positions:** 1 at a time
- **Trading Symbols:** BTC only (most liquid)

**Risk Parameters:**
```yaml
max_position_size: 0.05      # 5% max
max_daily_loss: 0.01         # 1% daily loss limit
max_drawdown: 0.05           # 5% max drawdown
default_stop_loss: 0.015     # 1.5%
default_take_profit: 0.03    # 3%
min_confidence: 0.8          # 80% required
```

**Usage:**
```bash
cp configs/examples/conservative.yaml configs/config.yaml
# Set environment variables (see below)
./bin/orchestrator --verify-keys
```

---

### 2. Aggressive Trading (`aggressive.yaml`)

**Best for:** Experienced traders, higher risk tolerance, active portfolio

**Key Features:**
- **Risk Level:** High
- **Position Size:** Up to 20% per trade
- **Stop Loss:** Wider 3%
- **Take Profit:** Aggressive 10%
- **Confidence Required:** 60% (lower threshold)
- **Max Positions:** 5 concurrent trades
- **Trading Symbols:** BTC, ETH, BNB, SOL (diversified)

**Risk Parameters:**
```yaml
max_position_size: 0.2       # 20% max
max_daily_loss: 0.05         # 5% daily loss limit
max_drawdown: 0.15           # 15% max drawdown
default_stop_loss: 0.03      # 3%
default_take_profit: 0.10    # 10%
min_confidence: 0.6          # 60% required
```

**Usage:**
```bash
cp configs/examples/aggressive.yaml configs/config.yaml
# Set environment variables (see below)
./bin/orchestrator --verify-keys
```

**⚠️ WARNING:** This configuration is high-risk. Only use with capital you can afford to lose.

---

### 3. Paper Trading (`paper-trading.yaml`)

**Best for:** Testing, learning, strategy development, debugging

**Key Features:**
- **Risk Level:** None (simulated)
- **Trading Mode:** PAPER (no real trades)
- **Realistic Simulation:** Includes slippage, partial fills
- **Full Logging:** Debug-level logs for analysis
- **No API Keys Required:** Can run without exchange credentials

**Risk Parameters:**
```yaml
max_position_size: 0.1       # 10% max
max_daily_loss: 0.02         # 2% daily loss limit
max_drawdown: 0.1            # 10% max drawdown
default_stop_loss: 0.02      # 2%
default_take_profit: 0.05    # 5%
min_confidence: 0.7          # 70% required
```

**Usage:**
```bash
cp configs/examples/paper-trading.yaml configs/config.yaml
# Minimal environment variables needed
export DATABASE_PASSWORD=postgres
docker-compose up -d
./bin/orchestrator
```

**✅ SAFE:** No real trades, perfect for learning and testing.

---

## Required Environment Variables

All configurations require these environment variables. Create a `.env` file in the project root:

### For Paper Trading (Minimal)
```bash
# Database
DATABASE_PASSWORD=postgres  # Simple password OK for local dev

# LLM (optional - Bifrost handles this)
# LLM_API_KEY=your_llm_api_key
```

### For Live Trading (Conservative/Aggressive)
```bash
# Database (PRODUCTION)
DATABASE_PASSWORD=your_strong_database_password_here

# Redis (if password-protected)
REDIS_PASSWORD=your_redis_password

# Exchange API Keys
BINANCE_API_KEY=your_binance_api_key
BINANCE_API_SECRET=your_binance_api_secret

# LLM Gateway (optional for Bifrost)
LLM_API_KEY=your_llm_api_key

# Optional overrides
DATABASE_HOST=localhost
REDIS_HOST=localhost
NATS_URL=nats://localhost:4222
LLM_ENDPOINT=http://localhost:8080/v1/chat/completions
```

---

## Quick Start Guide

### 1. Choose a Configuration

```bash
# For testing (RECOMMENDED TO START)
cp configs/examples/paper-trading.yaml configs/config.yaml

# For conservative live trading
cp configs/examples/conservative.yaml configs/config.yaml

# For aggressive live trading
cp configs/examples/aggressive.yaml configs/config.yaml
```

### 2. Set Environment Variables

```bash
# Copy example env file
cp .env.example .env

# Edit with your values
nano .env  # or vim, code, etc.
```

### 3. Verify Configuration

```bash
# Check config is valid
./bin/orchestrator --verify-keys

# Should output:
# ✅ All API keys and configuration verified successfully
# System is ready to start
```

### 4. Start Trading

```bash
# Start infrastructure
docker-compose up -d

# Start orchestrator
./bin/orchestrator

# In another terminal, start agents
./bin/technical-agent &
./bin/trend-agent &
./bin/risk-agent &
```

---

## Customization

Feel free to modify these configurations for your needs:

### Adjust Risk Tolerance

```yaml
risk:
  max_position_size: 0.15      # Your comfort level (0.01 to 0.5)
  max_daily_loss: 0.03         # Daily risk tolerance
  max_drawdown: 0.10           # Maximum portfolio decline
  min_confidence: 0.75         # Signal quality requirement
```

### Add More Trading Pairs

```yaml
trading:
  symbols:
    - "BTCUSDT"
    - "ETHUSDT"
    - "ADAUSDT"   # Add your preferred pairs
    - "MATICUSDT"
```

### Tune LLM Behavior

```yaml
llm:
  temperature: 0.7    # 0.0 = deterministic, 1.0 = creative
  timeout: 30000      # ms to wait for LLM response
  max_tokens: 2000    # Response length
```

---

## Risk Comparison Table

| Parameter              | Conservative | Aggressive | Paper Trading |
|------------------------|--------------|------------|---------------|
| Max Position Size      | 5%           | 20%        | 10%           |
| Max Daily Loss         | 1%           | 5%         | 2%            |
| Max Drawdown           | 5%           | 15%        | 10%           |
| Stop Loss              | 1.5%         | 3%         | 2%            |
| Take Profit            | 3%           | 10%        | 5%            |
| Min Confidence         | 80%          | 60%        | 70%           |
| Max Concurrent Trades  | 1            | 5          | 3             |
| Trading Pairs          | 1 (BTC)      | 4          | 3             |
| LLM Approval Required  | Yes          | No         | Yes           |

---

## Safety Checklist

Before using conservative or aggressive configs in production:

- [ ] Test with paper trading configuration first
- [ ] Verify all environment variables are set
- [ ] Run `./bin/orchestrator --verify-keys` successfully
- [ ] Start with MINIMAL capital to test live trading
- [ ] Monitor first few trades closely
- [ ] Set up Grafana dashboards for monitoring
- [ ] Understand the risk parameters
- [ ] Have a plan for when to stop trading
- [ ] Keep secure backups of API keys
- [ ] Use testnet mode initially if available

---

## Troubleshooting

### Configuration Validation Errors

```bash
# Check what's wrong
./bin/orchestrator 2>&1 | grep "Configuration validation failed"

# Common issues:
# - Missing environment variables
# - Invalid risk parameters (outside 0-1 range)
# - Weak passwords in production
# - Testnet enabled in production environment
```

### Paper Trading Not Working

```bash
# Ensure mode is set correctly
grep "mode:" configs/config.yaml
# Should show: mode: "paper"

# Check TRADING_MODE env var
echo $TRADING_MODE  # Should be empty or "paper"
```

### Live Trading Not Executing

```bash
# Verify API keys
./bin/orchestrator --verify-keys

# Check exchange configuration
grep -A 5 "exchanges:" configs/config.yaml

# Verify testnet is disabled for production
grep "testnet:" configs/config.yaml
# Should show: testnet: false
```

---

## Support

- **Documentation:** See `CLAUDE.md` and `README.md`
- **Issues:** https://github.com/yourusername/cryptofunk/issues
- **Configuration Guide:** `docs/CONFIGURATION.md` (if available)

---

## License

Same as parent project. See LICENSE file in repository root.
