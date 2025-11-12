# CryptoFunk Troubleshooting Guide

This guide helps you diagnose and fix common issues with CryptoFunk.

## Table of Contents

- [Quick Diagnosis](#quick-diagnosis)
- [Configuration Issues](#configuration-issues)
- [Database Issues](#database-issues)
- [Agent Issues](#agent-issues)
- [Trading Issues](#trading-issues)
- [Performance Issues](#performance-issues)
- [Network & Connectivity](#network--connectivity)
- [Debugging Tools](#debugging-tools)
- [FAQ](#faq)

---

## Quick Diagnosis

Start here if you're not sure what's wrong:

```bash
# 1. Check Docker services
docker-compose ps

# 2. Verify configuration
./bin/orchestrator --verify-keys

# 3. Check database connectivity
psql -h localhost -U postgres -d cryptofunk -c "SELECT 1;"

# 4. View recent logs
./scripts/dev/watch-logs.sh --errors

# 5. Check agent status
./scripts/dev/run-all-agents.sh status
```

---

## Configuration Issues

### Error: "Configuration validation failed"

**Symptom:**
```
Configuration validation failed with 3 error(s):

  1. database.password: Database password is required in non-development environments
  2. exchanges.binance.api_key: API key is required for live trading
  3. risk.max_position_size: Invalid max_position_size 1.50
```

**Cause:** Missing or invalid configuration values.

**Solution:**
1. Check the specific fields mentioned in the error
2. Verify environment variables are set:
   ```bash
   env | grep CRYPTOFUNK
   env | grep DATABASE
   env | grep BINANCE
   ```
3. Update `.env` file or `configs/config.yaml`
4. For production secrets, see [Production Secret Enforcement](#production-secret-enforcement)

**Common Fixes:**
```bash
# Missing database password
export DATABASE_PASSWORD=your_strong_password

# Missing exchange API keys (for live trading)
export BINANCE_API_KEY=your_api_key
export BINANCE_API_SECRET=your_api_secret

# Invalid risk parameter (must be 0-1)
# Edit configs/config.yaml:
risk:
  max_position_size: 0.1  # Must be between 0 and 1
```

---

### Production Secret Enforcement

**Symptom:**
```
Configuration validation failed:
  1. database.password: Password is too weak for production
  2. database.password: Contains common placeholder 'changeme'
```

**Cause:** Weak or placeholder passwords detected in production environment.

**Solution:**
Generate strong passwords for production:

```bash
# Generate strong password (Linux/Mac)
openssl rand -base64 32

# Or use pwgen
pwgen -s 32 1

# Set in environment
export DATABASE_PASSWORD="$(openssl rand -base64 32)"
export REDIS_PASSWORD="$(openssl rand -base64 32)"
```

**Requirements for production:**
- Minimum 12 characters
- No common placeholders (changeme, password, etc.)
- SSL enabled for database (`ssl_mode: require`)
- Test net disabled (`testnet: false`)

---

### Environment Variable Not Working

**Symptom:** Changes to `.env` file or environment variables not taking effect.

**Possible Causes:**
1. Environment variables not exported
2. Docker not restarted after `.env` changes
3. Wrong environment variable name

**Solution:**
```bash
# 1. Verify variable is exported
env | grep YOUR_VARIABLE

# 2. Restart Docker services
docker-compose down
docker-compose up -d

# 3. Check variable naming
# Must use CRYPTOFUNK_ prefix for config overrides
export CRYPTOFUNK_TRADING_MODE=paper  # ✓ Correct
export TRADING_MODE=paper              # ✗ Won't work

# 4. For Docker, update .env and restart
nano .env
docker-compose up -d --force-recreate
```

---

## Database Issues

### Error: "Failed to connect to database"

**Symptom:**
```
Failed to initialize database: connection refused
```

**Diagnosis:**
```bash
# Check if PostgreSQL is running
docker-compose ps postgres

# Check PostgreSQL logs
docker-compose logs postgres | tail -20

# Try connecting manually
psql -h localhost -U postgres -d cryptofunk
```

**Solutions:**

**1. PostgreSQL not running:**
```bash
docker-compose up -d postgres
# Wait 10 seconds for startup
sleep 10
psql -h localhost -U postgres -d cryptofunk
```

**2. Wrong password:**
```bash
# Check configured password
grep POSTGRES_PASSWORD .env

# Update and restart
echo "POSTGRES_PASSWORD=your_password" >> .env
docker-compose up -d --force-recreate postgres
```

**3. Port conflict (5432 already in use):**
```bash
# Check what's using port 5432
lsof -i :5432
# or
netstat -an | grep 5432

# Stop conflicting service or change port
docker-compose down
# Edit docker-compose.yml to use different port:
# ports:
#   - "5433:5432"  # Host:Container
docker-compose up -d postgres
```

**4. Database doesn't exist:**
```bash
# Create database
./scripts/dev/reset-db.sh

# Or manually
psql -h localhost -U postgres -c "CREATE DATABASE cryptofunk;"
```

---

### Migration Errors

**Symptom:**
```
Migration failed: relation "trading_sessions" already exists
```

**Cause:** Migrations run out of order or database in inconsistent state.

**Solution:**
```bash
# Option 1: Reset database (DESTROYS ALL DATA)
./scripts/dev/reset-db.sh

# Option 2: Check migration status
psql -h localhost -U postgres -d cryptofunk \
  -c "SELECT * FROM schema_version ORDER BY version;"

# Option 3: Manual migration fix
# Drop specific table and re-run migration
psql -h localhost -U postgres -d cryptofunk \
  -c "DROP TABLE IF EXISTS trading_sessions CASCADE;"
# Then re-run migrations
```

---

### TimescaleDB Extension Missing

**Symptom:**
```
ERROR: extension "timescaledb" is not available
```

**Solution:**
```bash
# Use correct PostgreSQL image
# In docker-compose.yml:
postgres:
  image: timescale/timescaledb:latest-pg15  # ✓ Correct
  # NOT: postgres:15                        # ✗ Missing TimescaleDB

# Recreate container
docker-compose down
docker-compose up -d postgres
```

---

## Agent Issues

### Agents Not Starting

**Diagnosis:**
```bash
# Check if binaries exist
ls -lh bin/*-agent

# Check agent logs
./scripts/dev/watch-logs.sh agents

# Try starting manually
./bin/technical-agent
```

**Solutions:**

**1. Binaries not built:**
```bash
# Build all agents
go build -o bin/technical-agent cmd/agents/technical-agent/main.go
go build -o bin/trend-agent cmd/agents/trend-agent/main.go
go build -o bin/risk-agent cmd/agents/risk-agent/main.go

# Or use helper script
./scripts/build-all.sh
```

**2. Port conflicts (metrics endpoints):**
```bash
# Check which ports are in use
lsof -i :9101  # technical-agent
lsof -i :9102  # trend-agent
lsof -i :9103  # risk-agent

# Kill conflicting processes
kill -9 $(lsof -t -i :9101)

# Or change ports in agent config
```

**3. NATS not running:**
```bash
# Agents need NATS for communication
docker-compose ps nats

# Start NATS
docker-compose up -d nats

# Verify NATS is healthy
curl http://localhost:8222/varz
```

---

### Agents Disconnecting

**Symptom:** Agents start but disconnect after a few seconds.

**Diagnosis:**
```bash
# Check agent logs for errors
tail -f tmp/logs/technical-agent.log

# Check NATS connectivity
docker-compose logs nats | tail -50
```

**Common Causes:**

**1. Configuration issues:**
- Wrong NATS URL
- Missing environment variables
- Invalid agent configuration

**2. Resource limits:**
```bash
# Check memory/CPU usage
docker stats

# Increase limits in docker-compose.yml if needed
```

**3. Network issues:**
```bash
# Verify agents can reach NATS
ping localhost
telnet localhost 4222  # NATS port
```

---

### Agent Signals Not Appearing

**Symptom:** Agents running but no signals in database.

**Diagnosis:**
```bash
# Check database for signals
psql -h localhost -U postgres -d cryptofunk \
  -c "SELECT * FROM agent_signals ORDER BY created_at DESC LIMIT 10;"

# Check agent logs
./scripts/dev/watch-logs.sh agents | grep -i signal

# Check orchestrator is running
docker-compose ps orchestrator
```

**Solutions:**

**1. Orchestrator not running:**
```bash
docker-compose up -d orchestrator
# Or
./bin/orchestrator
```

**2. Agents not registered:**
```bash
# Check agent_status table
psql -h localhost -U postgres -d cryptofunk \
  -c "SELECT * FROM agent_status;"

# Restart agents to re-register
./scripts/dev/run-all-agents.sh restart
```

**3. Database connection issues:**
- Check agent logs for database errors
- Verify connection string in config
- Ensure database is running

---

## Trading Issues

### No Trades Executing

**Diagnosis:**
```bash
# 1. Check trading mode
grep "mode:" configs/config.yaml

# 2. Check recent orders
psql -h localhost -U postgres -d cryptofunk \
  -c "SELECT * FROM orders ORDER BY created_at DESC LIMIT 10;"

# 3. Check risk agent logs
tail -f tmp/logs/risk-agent.log | grep -i veto

# 4. Check circuit breakers
psql -h localhost -U postgres -d cryptofunk \
  -c "SELECT * FROM agent_status WHERE agent_type = 'risk';"
```

**Common Causes:**

**1. Risk agent vetoing all trades:**
```
Max drawdown threshold exceeded (10% > 5%)
Circuit breaker active: max_daily_loss
```

**Solution:**
- Adjust risk parameters in `configs/config.yaml`
- Reset trading session
- Check if in paper vs live mode

**2. Insufficient confidence:**
```
Signal confidence 0.65 below minimum 0.70
```

**Solution:**
- Lower `min_confidence` in config
- Improve agent strategies
- Check LLM is responding

**3. No trading signals:**
- Agents not generating signals
- Market conditions not suitable
- Technical indicators not showing setups

**4. Exchange API issues:**
```bash
# Check exchange connectivity
tail -f tmp/logs/order-executor.log

# Verify API keys
./bin/orchestrator --verify-keys
```

---

### Paper Trading Not Working

**Symptom:** Expected trades in paper mode but nothing happening.

**Verification:**
```bash
# Confirm paper mode
grep "mode:" configs/config.yaml
# Should show: mode: "paper"

# Check mock exchange is active
docker-compose logs orchestrator | grep -i "paper\|mock"

# Verify no real API calls
# Should NOT see real exchange API calls in logs
```

**If still not working:**
1. Restart with paper trading config:
   ```bash
   cp configs/examples/paper-trading.yaml configs/config.yaml
   docker-compose restart orchestrator
   ```

2. Check orders table:
   ```bash
   psql -h localhost -U postgres -d cryptofunk \
     -c "SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '1 hour';"
   ```

3. Generate test signal manually (for debugging):
   ```bash
   # Insert test signal
   psql -h localhost -U postgres -d cryptofunk << SQL
   INSERT INTO agent_signals (session_id, agent_type, signal_type, symbol, confidence, reasoning)
   VALUES (
     (SELECT id FROM trading_sessions WHERE status = 'ACTIVE' LIMIT 1),
     'technical',
     'BUY',
     'BTCUSDT',
     0.85,
     'Test signal for debugging'
   );
   SQL
   ```

---

### Live Trading Not Working (Dangerous - Real Money!)

⚠️ **DANGER:** Only debug live trading issues if you understand the risks.

**Safety First:**
1. Switch to paper mode immediately if something is wrong
2. Check position sizes are appropriate
3. Verify stop losses are set
4. Monitor closely for the first few trades

**Common Issues:**

**1. Exchange API authentication failed:**
```bash
# Verify API keys
./bin/orchestrator --verify-keys

# Check API key permissions on exchange
# Required: Read, Trade (NOT Withdraw)

# Test API connection
curl -X GET 'https://api.binance.com/api/v3/account' \
  -H 'X-MBX-APIKEY: your_api_key'
```

**2. Test net mode enabled:**
```bash
# Verify testnet is OFF
grep "testnet:" configs/config.yaml
# Should show: testnet: false

# Update if needed
sed -i 's/testnet: true/testnet: false/' configs/config.yaml
```

**3. Insufficient balance:**
```bash
# Check exchange balance
# Log into exchange web interface
# or use API to check balance
```

---

## Performance Issues

### High CPU Usage

**Diagnosis:**
```bash
# Check CPU usage by service
docker stats

# Check which process is using CPU
top -o cpu

# Check agent processing times
./scripts/dev/watch-logs.sh agents | grep "duration"
```

**Solutions:**

**1. Too many indicators:**
- Reduce indicator calculations
- Increase calculation intervals
- Cache results

**2. Too many agents running:**
```bash
# Disable unused agents
# Edit configs/agents.yaml or docker-compose.yml
# Comment out agents you don't need
```

**3. Database queries slow:**
```bash
# Check slow queries
psql -h localhost -U postgres -d cryptofunk << SQL
SELECT query, calls, total_time, mean_time
FROM pg_stat_statements
ORDER BY total_time DESC
LIMIT 10;
SQL

# Add indexes if needed
# Vacuum database
VACUUM ANALYZE;
```

---

### High Memory Usage

**Diagnosis:**
```bash
# Check memory by service
docker stats

# Check Go memory profiles (if enabled)
curl http://localhost:8081/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**Solutions:**

**1. Reduce cache size:**
```yaml
# In configs/config.yaml
redis:
  maxmemory: 256mb  # Reduce from 512mb
```

**2. Limit database connections:**
```yaml
database:
  pool_size: 5  # Reduce from 10
```

**3. Clear old data:**
```bash
# Delete old candlestick data
psql -h localhost -U postgres -d cryptofunk << SQL
DELETE FROM candlesticks WHERE open_time < NOW() - INTERVAL '30 days';
SQL
```

---

## Network & Connectivity

### Port Conflicts

**Symptom:**
```
Error: bind: address already in use
```

**Diagnosis:**
```bash
# Check what's using the port
lsof -i :8080  # API
lsof -i :8081  # Orchestrator
lsof -i :5432  # PostgreSQL
lsof -i :6379  # Redis

# On Linux
netstat -tulpn | grep :8080
```

**Solution:**
```bash
# Option 1: Stop conflicting service
kill -9 $(lsof -t -i :8080)

# Option 2: Change port
# Edit docker-compose.yml or config.yaml
ports:
  - "8082:8080"  # Use host port 8082 instead
```

---

### Can't Access Web UI

**Symptom:** `curl http://localhost:8080` returns "Connection refused"

**Diagnosis:**
```bash
# Check if API is running
docker-compose ps api

# Check API logs
docker-compose logs api | tail -20

# Verify port mapping
docker-compose ps | grep api
# Should show: 0.0.0.0:8080->8080/tcp
```

**Solution:**
```bash
# Start API
docker-compose up -d api

# Check firewall (Linux)
sudo ufw allow 8080/tcp

# Check firewall (Mac)
# System Preferences > Security & Privacy > Firewall
```

---

## Debugging Tools

### Enable Debug Logging

```yaml
# In configs/config.yaml
app:
  log_level: "debug"  # Change from "info"
```

```bash
# Or via environment variable
export CRYPTOFUNK_APP_LOG_LEVEL=debug
docker-compose restart orchestrator
```

### Interactive Database Shell

```bash
# Connect to database
psql -h localhost -U postgres -d cryptofunk

# Useful queries
\dt                          # List tables
\d trading_sessions          # Describe table

SELECT * FROM agent_status;  # Check agent health
SELECT * FROM orders WHERE status = 'FILLED' ORDER BY created_at DESC LIMIT 10;
SELECT * FROM positions WHERE status = 'OPEN';
```

### View Metrics (Prometheus)

```bash
# Access Prometheus UI
open http://localhost:9090

# Query metrics
# Example queries:
cryptofunk_total_trades
cryptofunk_agent_status{agent_type="technical"}
rate(cryptofunk_http_requests_total[5m])
```

### View Dashboards (Grafana)

```bash
# Access Grafana
open http://localhost:3000
# Default: admin/admin

# Check if dashboards are loaded
# Dashboards > Manage
```

### MCP Server Debugging

```bash
# Test MCP server directly
echo '{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}' | ./bin/technical-indicators-server

# Capture MCP protocol traffic
./bin/technical-indicators-server 2>stderr.log 1>stdout.log

# View protocol messages
cat stdout.log | jq .

# View server logs
cat stderr.log
```

---

## FAQ

### Q: How do I switch between paper and live trading?

**A:** Update `trading.mode` in `configs/config.yaml`:

```yaml
trading:
  mode: "paper"  # or "live"
```

Then restart:
```bash
docker-compose restart orchestrator
```

Always test in paper mode first!

---

### Q: Where are the logs stored?

**A:**
- **Docker services:** `docker-compose logs [service]`
- **Local agents:** `./tmp/logs/*.log`
- **Orchestrator:** `./tmp/logs/orchestrator.log` or Docker logs

View all logs:
```bash
./scripts/dev/watch-logs.sh
```

---

### Q: How do I reset everything?

**A:**
```bash
# Stop all services
docker-compose down

# Reset database (deletes all data!)
./scripts/dev/reset-db.sh

# Reset Docker volumes (deletes all Docker data!)
docker-compose down -v

# Start fresh
docker-compose up -d
```

---

### Q: Can I run CryptoFunk without Docker?

**A:** Yes, but you need to run services manually:

```bash
# Start PostgreSQL locally
brew services start postgresql@15  # Mac
# or
sudo systemctl start postgresql    # Linux

# Start Redis
brew services start redis          # Mac
# or
sudo systemctl start redis         # Linux

# Start NATS
nats-server &

# Run orchestrator
./bin/orchestrator

# Run agents
./scripts/dev/run-all-agents.sh start
```

---

### Q: How do I add more trading pairs?

**A:** Edit `configs/config.yaml`:

```yaml
trading:
  symbols:
    - "BTCUSDT"
    - "ETHUSDT"
    - "BNBUSDT"   # Add your pairs here
    - "ADAUSDT"
```

Restart orchestrator:
```bash
docker-compose restart orchestrator
```

---

### Q: Why are my API keys being rejected?

**A:** Common causes:

1. **Incorrect keys**: Copy-paste error, missing characters
   ```bash
   ./bin/orchestrator --verify-keys
   ```

2. **Wrong permissions**: API key needs "Read" and "Trade" permissions
   - Check on exchange website
   - Recreate key if needed

3. **IP restrictions**: Exchange requires whitelisted IPs
   - Add your IP to whitelist on exchange
   - Or disable IP restriction (less secure)

4. **Testnet vs mainnet**: Using testnet keys with mainnet or vice versa
   - Verify `testnet: true/false` matches your keys

---

### Q: The system is making bad trades, what do I do?

**A:** Immediately:

1. **Switch to paper mode**:
   ```bash
   # Edit config
   sed -i 's/mode: "live"/mode: "paper"/' configs/config.yaml
   # Restart
   docker-compose restart orchestrator
   ```

2. **Review recent trades**:
   ```sql
   SELECT * FROM trades ORDER BY executed_at DESC LIMIT 20;
   SELECT * FROM positions WHERE status = 'CLOSED' ORDER BY closed_at DESC LIMIT 10;
   ```

3. **Analyze what went wrong**:
   - Check agent signals leading to trades
   - Review LLM decisions
   - Look for pattern in bad trades

4. **Adjust strategy**:
   - Increase `min_confidence`
   - Tighten `stop_loss`
   - Reduce `max_position_size`
   - Review risk parameters

5. **Backtest changes**:
   - Test new settings in paper mode
   - Monitor for several days
   - Only go live when confident

---

## Getting Help

If you're still stuck:

1. **Check logs**:
   ```bash
   ./scripts/dev/watch-logs.sh --errors
   ```

2. **Search existing issues**:
   https://github.com/yourusername/cryptofunk/issues

3. **Create new issue** with:
   - CryptoFunk version
   - Operating system
   - Docker version
   - Configuration (sanitized, no API keys!)
   - Full error message
   - Steps to reproduce

4. **Join community**:
   - Discord: [link]
   - Telegram: [link]

---

## Appendix: Useful Commands

```bash
# Configuration
./bin/orchestrator --verify-keys         # Verify all API keys
grep "mode:" configs/config.yaml         # Check trading mode

# Database
./scripts/dev/reset-db.sh                # Reset database
./scripts/dev/generate-test-data.sh      # Add test data
psql -h localhost -U postgres -d cryptofunk  # Connect to DB

# Services
docker-compose ps                        # Service status
docker-compose up -d                     # Start all
docker-compose down                      # Stop all
docker-compose restart orchestrator      # Restart service
docker-compose logs -f api               # View logs

# Agents
./scripts/dev/run-all-agents.sh start    # Start all agents
./scripts/dev/run-all-agents.sh stop     # Stop all agents
./scripts/dev/run-all-agents.sh status   # Agent status

# Logs
./scripts/dev/watch-logs.sh              # All logs
./scripts/dev/watch-logs.sh --errors     # Errors only
./scripts/dev/watch-logs.sh technical    # Specific agent

# Monitoring
curl http://localhost:8081/health        # Orchestrator health
curl http://localhost:8080/health        # API health
open http://localhost:9090               # Prometheus
open http://localhost:3000               # Grafana
```
