# CryptoFunk Example Configurations

This directory contains example configurations for different trading strategies and use cases. Each configuration is pre-tuned for a specific risk profile and trading style.

## Available Configurations

### 1. Paper Trading (`paper-trading.yaml`)

**Use Case**: Testing, development, strategy validation, learning

**Risk Level**: None (simulated trading only)

**Key Features**:
- NO REAL MONEY - All trades are simulated
- Mock exchange with realistic slippage and latency
- Moderate risk parameters for realistic testing
- Debug logging for detailed analysis
- Separate database for isolation

**Perfect For**:
- First-time users learning the system
- Strategy development and validation
- Testing new agent configurations
- CI/CD integration testing
- Risk-free experimentation

**Quick Start**:
```bash
# Copy paper trading config
cp configs/examples/paper-trading.yaml configs/config.yaml

# Start infrastructure
task docker-up

# Run migrations
task db-migrate

# Start paper trading
task run-paper
```

---

### 2. Conservative (`conservative.yaml`)

**Use Case**: Risk-averse traders, long-term investors, live trading beginners

**Risk Level**: Low

**Expected Returns**: 5-15% annual

**Key Features**:
- Small position sizes (5% max)
- Tight stop losses (2%)
- High confidence threshold (80%)
- Limited concurrent positions (2 max)
- Focus on major pairs (BTC, ETH only)
- LLM approval required for all trades

**Risk Parameters**:
- Max Position Size: 5%
- Max Daily Loss: 1%
- Max Drawdown: 5%
- Stop Loss: 2%
- Take Profit: 5% (2.5:1 risk/reward)
- Min Confidence: 80%

**Perfect For**:
- First live trading deployment
- Capital preservation focus
- Long-term steady growth
- Minimal monitoring required
- Risk-averse investors

**Quick Start**:
```bash
# Copy conservative config
cp configs/examples/conservative.yaml configs/config.yaml

# Set environment variables (REQUIRED for live trading)
export BINANCE_API_KEY="your_api_key"
export BINANCE_API_SECRET="your_api_secret"
export DATABASE_PASSWORD="your_secure_password"
export REDIS_PASSWORD="your_secure_password"

# Verify configuration and API keys
./bin/orchestrator --verify-keys

# Start trading (if verification passes)
task run-orchestrator
```

---

### 3. Aggressive (`aggressive.yaml`)

**Use Case**: Risk-tolerant traders, experienced users, short-term trading

**Risk Level**: High

**Expected Returns**: 30-100%+ annual (with higher volatility)

**Key Features**:
- Large position sizes (20% max)
- Multiple concurrent positions (5 max)
- Lower confidence threshold (60%)
- Wider stops for volatility (5%)
- Higher profit targets (15%)
- Diversified across 8+ trading pairs

**Risk Parameters**:
- Max Position Size: 20%
- Max Daily Loss: 5%
- Max Drawdown: 15%
- Stop Loss: 5%
- Take Profit: 15% (3:1 risk/reward)
- Min Confidence: 60%

**Trading Pairs**:
BTC/USDT, ETH/USDT, BNB/USDT, SOL/USDT, ADA/USDT, XRP/USDT, MATIC/USDT, AVAX/USDT

**Perfect For**:
- Experienced traders
- Higher risk tolerance
- Active trading and monitoring
- Capital you can afford to lose
- Seeking higher returns

**⚠️ Risk Warning**:
- High volatility exposure
- Potential for significant losses
- Requires active monitoring
- Not suitable for risk-averse investors
- **Test thoroughly in paper mode first**
- **Never invest more than you can afford to lose**

**Quick Start**:
```bash
# IMPORTANT: Test in paper mode FIRST
cp configs/examples/aggressive.yaml configs/config.yaml.test
# Change trading.mode to "paper" in config.yaml.test
# Run paper trading for 2-4 weeks to validate strategy

# When ready for live trading
cp configs/examples/aggressive.yaml configs/config.yaml

# Set environment variables
export BINANCE_API_KEY="your_api_key"
export BINANCE_API_SECRET="your_api_secret"

# Verify configuration
./bin/orchestrator --verify-keys

# Start with small capital first!
# Edit config.yaml and set initial_capital to 1-5% of intended amount
task run-orchestrator
```

---

## Configuration Comparison

| Feature | Paper Trading | Conservative | Aggressive |
|---------|--------------|--------------|------------|
| **Risk Level** | None | Low | High |
| **Expected Return** | N/A | 5-15% | 30-100%+ |
| **Max Position Size** | 10% | 5% | 20% |
| **Max Daily Loss** | 3% | 1% | 5% |
| **Max Drawdown** | 10% | 5% | 15% |
| **Stop Loss** | 3% | 2% | 5% |
| **Take Profit** | 8% | 5% | 15% |
| **Min Confidence** | 65% | 80% | 60% |
| **Max Positions** | 3 | 2 | 5 |
| **Trading Pairs** | 5 | 2 | 8+ |
| **LLM Approval** | Optional | Required | Required |
| **Monitoring** | Optional | Minimal | Active |
| **Real Money** | ❌ No | ✅ Yes | ✅ Yes |

---

## Choosing the Right Configuration

### Start with Paper Trading

**ALWAYS start with paper trading**, regardless of experience level:

1. Copy `paper-trading.yaml` to `configs/config.yaml`
2. Run for at least 2-4 weeks
3. Monitor performance, drawdown, and trade quality
4. Verify strategy works as expected
5. Review all agent decisions and LLM reasoning

### Transition to Conservative

After successful paper trading, start live trading conservatively:

1. Use `conservative.yaml` as base
2. Start with small capital (10-20% of intended amount)
3. Monitor daily for first week
4. Verify risk management works correctly
5. Gradually increase capital as confidence builds

### Consider Aggressive (Optional)

Only move to aggressive configuration if:

- ✅ At least 3+ months successful conservative trading
- ✅ Proven positive returns with acceptable drawdown
- ✅ Deep understanding of system behavior
- ✅ Comfortable with high volatility
- ✅ Capital you can afford to lose
- ✅ Active monitoring capability

---

## Customizing Configurations

All configurations can be customized to fit your needs:

### Common Customizations

**Add/Remove Trading Pairs**:
```yaml
trading:
  symbols:
    - "BTC/USDT"
    - "ETH/USDT"
    # Add more pairs as needed
```

**Adjust Risk Parameters**:
```yaml
risk:
  max_position_size: 0.10  # 10% per position
  max_daily_loss: 0.02     # 2% daily loss limit
  max_drawdown: 0.08       # 8% max drawdown
  default_stop_loss: 0.03  # 3% stop loss
  default_take_profit: 0.09 # 9% take profit
```

**Change Confidence Threshold**:
```yaml
risk:
  min_confidence: 0.75  # Require 75% confidence
```

**Adjust LLM Temperature**:
```yaml
llm:
  temperature: 0.6  # Lower = more conservative, Higher = more aggressive
```

### Environment Variables

All configurations support environment variable overrides:

**Required for Live Trading**:
```bash
export BINANCE_API_KEY="your_api_key"
export BINANCE_API_SECRET="your_api_secret"
export DATABASE_PASSWORD="your_secure_password"
export REDIS_PASSWORD="your_secure_password"
```

**Optional**:
```bash
export DATABASE_HOST="postgres.example.com"
export REDIS_HOST="redis.example.com"
export NATS_URL="nats://nats.example.com:4222"
export LLM_ENDPOINT="http://bifrost.example.com:8080/v1/chat/completions"
```

---

## Security Best Practices

### Paper Trading
- ✅ Simple passwords acceptable for local development
- ✅ Can disable SSL for local PostgreSQL
- ✅ API keys not required (mock exchange)

### Live Trading (Conservative/Aggressive)
- ⚠️ **NEVER** commit API keys or passwords to git
- ⚠️ Use strong, unique passwords (12+ characters)
- ⚠️ Enable SSL for all services (`ssl_mode: require`)
- ⚠️ Store secrets in environment variables or HashiCorp Vault
- ⚠️ Rotate API keys and passwords regularly
- ⚠️ Use API key restrictions (IP whitelist, permissions)
- ⚠️ Enable 2FA on exchange account
- ⚠️ Monitor for unusual activity

### Verification Before Production

Always verify configuration before starting:

```bash
# Validate configuration and API keys
./bin/orchestrator --verify-keys

# Expected output for valid config:
# ✅ All API keys and configuration verified successfully
```

If verification fails, fix issues before starting the orchestrator.

---

## Monitoring and Alerts

All configurations include monitoring via Prometheus and Grafana:

**Access Monitoring**:
- Grafana UI: http://localhost:3000 (default: admin / your_password)
- Prometheus: http://localhost:9090
- Orchestrator Health: http://localhost:8081/health
- API Health: http://localhost:8080/health

**Key Metrics to Monitor**:
- Total P&L and returns
- Current drawdown
- Win rate and risk/reward ratio
- Number of positions
- Agent confidence levels
- Circuit breaker status
- API latency and errors

**Set Up Alerts**:
Configure Grafana alerts for:
- Drawdown exceeding 50% of max threshold
- High number of consecutive losses (5+)
- Circuit breaker activation
- Service health check failures
- High API error rates

---

## Troubleshooting

### Paper Trading Issues

**Problem**: "Cannot connect to exchange"
- **Solution**: Paper trading uses mock exchange, no real connection needed. Check logs for actual error.

**Problem**: "No trades being executed"
- **Solution**: Lower confidence threshold or check agent signals in database.

### Live Trading Issues

**Problem**: "API key verification failed"
- **Solution**: Run `./bin/orchestrator --verify-keys` to diagnose API key issues.

**Problem**: "Order rejected by exchange"
- **Solution**: Check account balance, trading permissions, and API key restrictions.

**Problem**: "Circuit breaker activated"
- **Solution**: System auto-halted due to max drawdown or volatility. Review trades and adjust risk parameters.

### Configuration Issues

**Problem**: "Configuration validation failed"
- **Solution**: Read error messages carefully. Each error shows field name and specific issue.

**Problem**: "Weak password detected"
- **Solution**: Use strong passwords (12+ chars, mixed case, numbers, symbols) for production.

---

## Migration Path

### Phase 1: Paper Trading (2-4 weeks)
```bash
cp configs/examples/paper-trading.yaml configs/config.yaml
task run-paper
# Monitor and validate strategy
```

### Phase 2: Conservative Live (1-3 months)
```bash
cp configs/examples/conservative.yaml configs/config.yaml
# Set environment variables with real API keys
./bin/orchestrator --verify-keys
# Start with 10-20% of intended capital
task run-orchestrator
```

### Phase 3: Scale Conservative (3-6 months)
```bash
# Gradually increase capital to 100%
# Continue monitoring and optimization
```

### Phase 4: Aggressive (Optional, 6+ months)
```bash
# Only if proven track record and high risk tolerance
cp configs/examples/aggressive.yaml configs/config.yaml
# Test in paper mode first for 2+ weeks
./bin/orchestrator --verify-keys
# Start with small allocation
task run-orchestrator
```

---

## Support and Documentation

- **Full Documentation**: `../docs/`
- **Task Commands**: `task --list`
- **Deployment Guide**: `../deployments/README.md`
- **Implementation Plan**: `../TASKS.md`
- **Architecture**: `../CLAUDE.md`

---

## Risk Disclosure

**IMPORTANT**: Cryptocurrency trading involves substantial risk of loss and is not suitable for every investor. The valuation of cryptocurrencies may fluctuate, and, as a result, clients may lose more than their original investment.

- Past performance is not indicative of future results
- This software is provided "as is" without warranty
- Always do your own research (DYOR)
- Never invest more than you can afford to lose
- Start with paper trading to understand the system
- Use conservative settings when starting live trading
- Monitor your positions actively
- Understand the risks before trading

**CryptoFunk is a tool. The responsibility for trading decisions and outcomes rests with the user.**
