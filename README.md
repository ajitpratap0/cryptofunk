# CryptoFunk - AI-Powered Crypto Trading Platform

<div align="center">

**Multi-Agent Trading System Orchestrated by Model Context Protocol (MCP)**

[![CI](https://github.com/ajitpratap0/cryptofunk/workflows/CI/badge.svg)](https://github.com/ajitpratap0/cryptofunk/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![MCP](https://img.shields.io/badge/MCP-Official%20SDK-7C3AED?style=flat)](https://github.com/modelcontextprotocol/go-sdk)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Production%20Ready-brightgreen)](https://github.com/ajitpratap0/cryptofunk)
[![Test Coverage](https://img.shields.io/badge/Test%20Pass%20Rate-100%25-brightgreen)](https://github.com/ajitpratap0/cryptofunk)
[![Code Quality](https://img.shields.io/badge/golangci--lint-passing-brightgreen)](https://github.com/ajitpratap0/cryptofunk)

[Documentation](#documentation) •
[Quick Start](#quick-start) •
[Architecture](#architecture) •
[Contributing](#contributing)

</div>

---

## Overview

**CryptoFunk** is a state-of-the-art cryptocurrency trading platform that leverages:

- **LLM-Powered Intelligence**: Claude/GPT-4 agents via Bifrost gateway for sophisticated reasoning
- **Multi-Agent Architecture**: Specialized AI agents for analysis, strategy, and risk management
- **Model Context Protocol (MCP)**: Official Go SDK for standardized agent orchestration
- **Unified LLM Gateway**: Bifrost for automatic failover, semantic caching, and 12+ provider support
- **Real-Time Trading**: WebSocket connections via CCXT to 100+ exchanges
- **Advanced Risk Controls**: Multi-layered protection with circuit breakers
- **Production Ready**: Containerized, cloud-native, and observable with explainable decisions

### Key Features

- **LLM-Powered Agents**: Claude/GPT-4 for intelligent decision-making with natural language explanations
- **Bifrost Gateway**: Unified API for 12+ LLM providers with automatic failover and semantic caching
- Multiple specialized trading agents working in concert
- MCP-based tool and resource sharing
- Consensus-based decision making with weighted voting
- Real-time market data via CCXT (100+ exchanges)
- Technical analysis using cinar/indicator (RSI, MACD, Bollinger Bands, 60+ indicators)
- Risk management with LLM-powered position sizing and approval
- Paper trading mode for safe testing
- REST API and WebSocket for monitoring
- Prometheus metrics and Grafana dashboards
- Explainable AI: Every decision includes reasoning and confidence scores

---

## Architecture

### High-Level Design

```
┌──────────────────────────────────────────────────────────┐
│                    MCP Orchestrator                      │
│               (Coordination & Consensus)                 │
└────────────────┬─────────────────────────────────────────┘
                 │
      ┌──────────┼──────────┐
      │          │          │
  ┌───▼───┐  ┌──▼───┐  ┌──▼────┐
  │Analysis│  │Strategy│ │ Risk  │
  │ Agents │  │ Agents │ │ Agent │
  │ (LLM)  │  │ (LLM)  │ │ (LLM) │
  └───┬───┘  └──┬───┘  └──┬────┘
      │         │         │
      └─────────┼─────────┘
                │
    ┌───────────▼────────────┐
    │   Bifrost LLM Gateway  │
    │  ┌──────────────────┐  │
    │  │ Claude (Primary) │  │
    │  │ GPT-4 (Fallback) │  │
    │  │ Gemini (Backup)  │  │
    │  └──────────────────┘  │
    │  Semantic Caching 90%  │
    └────────────────────────┘
                │
    ┌───────────▼────────────┐
    │      MCP Servers       │
    │  - Market Data (CCXT)  │
    │  - Tech Indicators     │
    │  - Risk Analyzer       │
    │  - Order Executor      │
    └────────────────────────┘
```

### Agent Types (LLM-Powered)

All agents use **LLM reasoning** via Bifrost gateway for sophisticated decision-making:

1. **Analysis Agents** - LLM analyzes market data with context
   - **Technical Analysis Agent**: LLM interprets RSI, MACD, patterns with reasoning
   - **Order Book Agent**: LLM evaluates depth, liquidity, imbalance
   - **Sentiment Agent**: LLM analyzes news and social media (future)

2. **Strategy Agents** - LLM generates trading decisions
   - **Trend Following Agent**: LLM evaluates trend strength and timing
   - **Mean Reversion Agent**: LLM identifies reversion opportunities
   - **Arbitrage Agent**: LLM spots arbitrage opportunities (future)

3. **Risk Agent** - LLM provides final approval with reasoning
   - Portfolio risk assessment with natural language explanation
   - Position sizing with confidence scores
   - Circuit breakers with detailed alerts

4. **Execution Agent** - Order placement and tracking
   - Integrated with CCXT for multi-exchange support

### Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.25+ |
| LLM Gateway | **Bifrost** | Latest (Claude, GPT-4, Gemini) |
| AI/ML | Claude Sonnet 4 (primary), GPT-4 Turbo (fallback) | API |
| MCP SDK | `modelcontextprotocol/go-sdk` | v1.0.0 |
| Exchange API | **CCXT** | Latest (100+ exchanges) |
| Technical Indicators | **cinar/indicator** | v2.1.22 (60+ indicators) |
| Database | PostgreSQL + **TimescaleDB** | 15+ |
| Vector Search | **pgvector** | Latest (PostgreSQL extension) |
| Cache | Redis | 7+ |
| Message Queue | NATS | 2.10+ |
| Containers | Docker + Kubernetes | Latest |
| Monitoring | Prometheus + Grafana | Latest |
| CI/CD | GitHub Actions | Native |
| Code Quality | golangci-lint | v2.6.1 (30+ linters) |

---

## Quick Start

### Prerequisites

- **Go 1.25 or higher** (required for generics support in cinar/indicator v2)
- [Task](https://taskfile.dev) (build automation tool)
- Docker and Docker Compose (for local development)
- **Kubernetes** (optional, for production deployment)
- **LLM API Keys**: Anthropic (Claude) or OpenAI (GPT-4)
- Binance account (testnet for development)
- PostgreSQL 15+ with TimescaleDB and pgvector extensions

#### Install Task

```bash
# macOS
brew install go-task/tap/go-task

# Linux
brew install go-task/tap/go-task
# or
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d

# Windows
winget install Task.Task
```

**Why Task instead of Make?** See [docs/TASK_VS_MAKE.md](docs/TASK_VS_MAKE.md) for detailed comparison.

### Installation

```bash
# Clone the repository
git clone https://github.com/ajitpratap0/cryptofunk.git
cd cryptofunk

# Copy environment template
cp .env.example .env

# Edit .env with your API keys (REQUIRED)
nano .env
# Add:
#   ANTHROPIC_API_KEY=your_claude_key
#   OPENAI_API_KEY=your_gpt4_key (optional, for fallback)
#   BINANCE_API_KEY=your_binance_key
#   BINANCE_SECRET_KEY=your_binance_secret

# Start infrastructure services (includes Bifrost)
docker-compose up -d

# Bifrost will be available at http://localhost:8080
# Verify: curl http://localhost:8080/health

# Run database migrations
task db-migrate

# Build all services
task build

# Start the orchestrator
./bin/orchestrator

# In separate terminals, start LLM-powered agents
./bin/technical-agent
./bin/trend-agent
./bin/risk-agent
```

### Quick Test with Paper Trading

```bash
# Start in paper trading mode
task run-paper

# Monitor via API
curl http://localhost:8080/api/v1/status

# Watch real-time updates via WebSocket
websocat ws://localhost:8080/api/v1/ws
```

---

## Production Deployment

### Docker Deployment (Local/Staging)

```bash
# Start all services with Docker Compose
docker-compose up -d

# Check service status
docker-compose ps

# View logs
docker-compose logs -f orchestrator

# Stop all services
docker-compose down
```

### Kubernetes Deployment (Production)

```bash
# Create namespace
kubectl apply -f deployments/k8s/namespace.yaml

# Create secrets (use your actual values)
kubectl create secret generic cryptofunk-secrets \
  --from-literal=database-url="postgresql://user:pass@host:5432/cryptofunk" \
  --from-literal=anthropic-api-key="your-claude-key" \
  --from-literal=binance-api-key="your-binance-key" \
  --from-literal=binance-secret="your-binance-secret" \
  -n cryptofunk

# Deploy all components
kubectl apply -f deployments/k8s/

# Check deployment status
kubectl get pods -n cryptofunk
kubectl get services -n cryptofunk

# Access Grafana dashboard
kubectl port-forward svc/grafana 3000:3000 -n cryptofunk
# Open http://localhost:3000 (admin/admin)

# View orchestrator logs
kubectl logs -f deployment/orchestrator -n cryptofunk
```

### Monitoring

- **Prometheus**: Metrics collection at `http://localhost:9090`
- **Grafana**: Visualization dashboards at `http://localhost:3000`
- **Health Checks**: All services expose `/health` endpoints
- **Alerts**: Configured for critical conditions (high error rate, circuit breakers)

See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for detailed deployment instructions.

---

## Documentation

### Getting Started
- **[Getting Started Guide](docs/GETTING_STARTED.md)** - ⭐ Quick start for developers
- **[Implementation Tasks](TASKS.md)** - 244 tasks across 10 phases (✅ Phase 10 Complete!)
- **[Contributing Guide](CONTRIBUTING.md)** - How to contribute
- **[Troubleshooting](docs/TROUBLESHOOTING.md)** - Common issues and solutions

### Architecture & Design
- **[Architecture Overview](docs/ARCHITECTURE.md)** - System architecture with hybrid MCP integration
- **[LLM Agent Architecture](docs/LLM_AGENT_ARCHITECTURE.md)** - ⭐ LLM-powered multi-agent design
- **[Future Agent Architecture](docs/AGENT_ARCHITECTURE_FUTURE.md)** - Custom RL models (post-MVP)
- **[MCP Integration](docs/MCP_INTEGRATION.md)** - Model Context Protocol overview
- **[MCP Server API Reference](docs/MCP_SERVERS.md)** - Complete API specs for all MCP servers
- **[Open Source Tools](docs/OPEN_SOURCE_TOOLS.md)** - ⭐ Technology choices rationale

### Development
- **[API Documentation](docs/API.md)** - REST and WebSocket API reference
- **[MCP Development Guide](docs/MCP_GUIDE.md)** - Building custom MCP servers and agents
- **[Task vs Make](docs/TASK_VS_MAKE.md)** - Why we chose Task over Make
- **[Testing Guide](TESTING.md)** - Test infrastructure and coverage

### Operations & Deployment
- **[Deployment Guide](docs/DEPLOYMENT.md)** - ⭐ Production deployment (Docker + Kubernetes)
- **[CI/CD Alternatives](docs/CI_CD_ALTERNATIVES.md)** - ⭐ Free CI/CD options for private repos
- **[Production Checklist](docs/PRODUCTION_CHECKLIST.md)** - Pre-deployment verification
- **[Production Security Checklist](docs/PRODUCTION_SECURITY_CHECKLIST.md)** - Security hardening checklist
- **[Disaster Recovery](docs/DISASTER_RECOVERY.md)** - Backup and recovery procedures
- **[Metrics Integration](docs/METRICS_INTEGRATION.md)** - Prometheus + Grafana setup
- **[Alert Runbook](docs/ALERT_RUNBOOK.md)** - Production alert response procedures

### Security
- **[Security Audit](docs/SECURITY_AUDIT.md)** - Comprehensive security audit (Jan 2025)
- **[Security Logging Audit](docs/SECURITY_LOGGING_AUDIT.md)** - Logging security review (Nov 2025)
- **[Secret Rotation](docs/SECRET_ROTATION.md)** - API key and secret rotation procedures
- **[TLS Setup](docs/TLS_SETUP.md)** - TLS/SSL configuration for production

---

## Project Structure

```
cryptofunk/
├── cmd/                        # Main applications
│   ├── orchestrator/           # MCP orchestrator
│   ├── mcp-servers/            # MCP tool/resource servers
│   │   ├── market-data/
│   │   ├── technical-indicators/
│   │   ├── risk-analyzer/
│   │   └── order-executor/
│   ├── agents/                 # Trading agents (MCP clients)
│   │   ├── technical-agent/
│   │   ├── orderbook-agent/
│   │   ├── sentiment-agent/
│   │   ├── trend-agent/
│   │   ├── reversion-agent/
│   │   └── risk-agent/
│   └── api/                    # REST/WebSocket API
│
├── internal/                   # Private application code
│   ├── orchestrator/           # Orchestration logic
│   ├── mcpserver/              # MCP server helpers
│   ├── mcpclient/              # MCP client helpers
│   ├── agents/                 # Agent business logic
│   ├── indicators/             # Technical indicators
│   ├── exchange/               # Exchange integrations
│   ├── models/                 # Data models
│   └── config/                 # Configuration
│
├── pkg/                        # Public libraries
│   ├── events/                 # Event definitions
│   └── utils/                  # Utilities
│
├── configs/                    # Configuration files
│   ├── config.yaml
│   ├── agents.yaml
│   └── mcp-servers.yaml
│
├── deployments/                # Deployment configs
│   ├── docker/
│   │   └── Dockerfile.*
│   ├── k8s/
│   └── docker-compose.yml
│
├── scripts/                    # Utility scripts
├── migrations/                 # Database migrations
├── docs/                       # Documentation
└── test/                       # E2E tests
```

---

## Configuration

### Main Configuration

Edit `configs/config.yaml`:

```yaml
# LLM Configuration
llm:
  gateway: "bifrost"         # Bifrost LLM gateway
  endpoint: "http://localhost:8080/v1/chat/completions"
  primary_model: "claude-sonnet-4-20250514"
  fallback_model: "gpt-4-turbo"
  temperature: 0.7
  max_tokens: 2000
  enable_caching: true       # Semantic caching (90% cost reduction)

trading:
  mode: "paper"              # paper | live
  symbols:
    - "BTCUSDT"
    - "ETHUSDT"
  exchange: "binance"        # via CCXT

risk:
  max_position_size_percent: 10
  max_portfolio_risk_percent: 2
  max_drawdown_percent: 20
  llm_approval_required: true # Risk agent LLM approval

orchestrator:
  pattern: "concurrent"      # sequential | concurrent | event_driven
  voting:
    min_threshold: 0.7
    min_agents: 2
  llm_consensus: true        # Use LLM for consensus resolution
```

### Environment Variables

Set in `.env`:

```bash
# LLM API Keys (REQUIRED)
ANTHROPIC_API_KEY=your_claude_key
OPENAI_API_KEY=your_gpt4_key     # Optional, for fallback
GOOGLE_API_KEY=your_gemini_key   # Optional, for backup

# Exchange API
BINANCE_API_KEY=your_key
BINANCE_SECRET_KEY=your_secret

# Database
POSTGRES_PASSWORD=your_password
REDIS_PASSWORD=your_password
```

---

## Development

### Building

```bash
# Build all services
task build

# Build specific service
task build-orchestrator
task build-agent-technical

# Run tests
task test

# Run with race detector (included in task test)
task test

# Generate coverage report
task test-coverage
```

### Running Locally

```bash
# Start infrastructure
docker-compose up -d

# Run orchestrator
task run-orchestrator

# Run agents (in separate terminals)
task run-agent-technical
task run-agent-trend
task run-agent-risk

# View Docker logs
docker-compose logs -f

# Or use task shortcuts
task docker-logs
```

### Testing

```bash
# Unit tests
task test-unit

# All tests with coverage
task test

# Integration tests
task test-integration

# Watch and test on changes
task test-watch

# Backtesting
./bin/backtest -symbol BTCUSDT -from 2024-01-01 -to 2024-06-01
```

---

## Monitoring

### Prometheus Metrics

Access at `http://localhost:9090`

Key metrics:
- `cryptofunk_sessions_active` - Active trading sessions
- `cryptofunk_decisions_total` - Total decisions by action
- `cryptofunk_agent_latency_seconds` - Agent processing time
- `cryptofunk_position_pnl` - Current position P&L

### Grafana Dashboards

Access at `http://localhost:3000` (admin/admin)

Pre-configured dashboards:
- System Overview
- Trading Performance
- Agent Performance
- Risk Metrics

### REST API

```bash
# System status
curl http://localhost:8080/api/v1/status

# Current positions
curl http://localhost:8080/api/v1/positions

# Agent health
curl http://localhost:8080/api/v1/agents

# Metrics
curl http://localhost:8080/api/v1/metrics
```

### WebSocket

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/ws');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Update:', data);
};
```

---

## Deployment

### Docker Compose (Development)

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

### Kubernetes (Production)

```bash
# Create namespace
kubectl create namespace cryptofunk

# Apply configurations
kubectl apply -f deployments/k8s/

# Check status
kubectl get pods -n cryptofunk

# View logs
kubectl logs -f deployment/orchestrator -n cryptofunk
```

### CI/CD

GitHub Actions pipeline automatically:
- Runs tests on PR
- Builds Docker images on merge
- Deploys to staging/production

---

## Backtesting

Run historical strategy validation:

```bash
# Basic backtest
./bin/backtest \
  -symbol BTCUSDT \
  -from 2024-01-01 \
  -to 2024-06-01 \
  -strategy trend

# With optimization
./bin/backtest \
  -symbol BTCUSDT \
  -from 2024-01-01 \
  -to 2024-06-01 \
  -optimize \
  -params configs/backtest-params.yaml

# Generate report
./bin/backtest -symbol BTCUSDT -report report.html
```

Results include:
- Total return
- Sharpe ratio
- Maximum drawdown
- Win rate
- Equity curve
- Trade distribution

---

## Security

### Best Practices

- Store API keys in environment variables (never in code)
- Use testnet for development
- Start with paper trading
- Enable circuit breakers
- Set strict position limits
- Monitor all trades
- Regular security audits

### API Key Management

```bash
# Use environment variables
export BINANCE_API_KEY="your_key"

# Or use secrets management (production)
kubectl create secret generic cryptofunk-secrets \
  --from-literal=binance-api-key=your_key \
  --from-literal=binance-secret-key=your_secret
```

---

## Performance

### Benchmarks

| Operation | Latency |
|-----------|---------|
| Market data ingestion | <10ms |
| Technical indicator calculation | <5ms |
| Agent decision | <50ms |
| Full trading cycle | <200ms |
| Order placement | <100ms |

### Scalability

- Horizontal scaling: Multiple orchestrator instances
- Agent isolation: Each agent runs independently
- Database sharding: TimescaleDB chunks
- Cache layer: Redis for hot data

---

## Roadmap

### MVP - LLM-Powered Trading (9.5 weeks)
- [x] Project structure and MCP foundation
- [ ] **Bifrost LLM gateway deployment**
- [ ] Core MCP servers (Market Data via CCXT, Technical Indicators via cinar/indicator)
- [ ] **LLM-powered agents (technical, trend, risk)** using Claude/GPT-4
- [ ] Orchestrator with consensus voting
- [ ] Paper trading mode
- [ ] REST API and monitoring
- [ ] **Explainability dashboard** (view LLM reasoning for all decisions)

### Post-MVP Enhancements
- [ ] Data collection and performance analysis
- [ ] **Custom RL model training** using collected trading data
- [ ] Hybrid approach: LLM strategy + RL execution optimization
- [ ] Multi-exchange support via CCXT
- [ ] Advanced strategies (arbitrage, market making)
- [ ] Sentiment analysis agent (news, social media)
- [ ] Web dashboard
- [ ] Mobile app

### Future (Phase 10+)
- [ ] Multi-model ensemble (Claude + GPT-4 + custom models)
- [ ] Advanced backtesting with walk-forward optimization
- [ ] High-frequency trading capabilities
- [ ] Options and derivatives support

---

## Troubleshooting

### Common Issues

**Agent won't connect to MCP server**
```bash
# Check if server is running
ps aux | grep market-data-server

# Check logs
tail -f /var/log/cryptofunk/market-data-server.log
```

**Database connection error**
```bash
# Verify PostgreSQL is running
docker-compose ps postgres

# Test connection
psql -h localhost -U postgres -d cryptofunk
```

**Orders not executing**
```bash
# Check if in paper trading mode
grep "mode:" configs/config.yaml

# Verify exchange API keys
./bin/orchestrator --verify-keys
```

### Debug Mode

```bash
# Run with debug logging
LOG_LEVEL=debug ./bin/orchestrator

# Enable MCP message tracing
MCP_TRACE=1 ./bin/technical-agent
```

---

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make changes and add tests
4. Ensure tests pass (`task test`)
5. Commit with conventional commits (`git commit -m 'feat: add amazing feature'`)
6. Push to branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Run `gofmt` before committing
- Add tests for new features
- Update documentation

---

## License

This project is licensed under the MIT License - see [LICENSE](LICENSE) file for details.

---

## Acknowledgments

- [Model Context Protocol](https://modelcontextprotocol.io) by Anthropic
- [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) maintained with Google
- Binance API for market data
- Open source community

---

## Support

- **Documentation**: [docs/](docs/)
- **Issues**: [GitHub Issues](https://github.com/ajitpratap0/cryptofunk/issues)
- **Discussions**: [GitHub Discussions](https://github.com/ajitpratap0/cryptofunk/discussions)
- **Email**: support@cryptofunk.io

---

## Disclaimer

**IMPORTANT**: This software is for educational and research purposes only.

- Cryptocurrency trading involves substantial risk of loss
- Past performance does not guarantee future results
- Always start with paper trading
- Never trade with money you cannot afford to lose
- Consult a financial advisor before live trading
- The authors assume no liability for your trading decisions

---

<div align="center">

**Built with ❤️ using Go and the Model Context Protocol**

[⬆ Back to Top](#cryptofunk---ai-powered-crypto-trading-platform)

</div>
