# Docker Deployment Guide

This directory contains Docker and Docker Compose configurations for running CryptoFunk in containerized environments.

## Table of Contents

- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Services Overview](#services-overview)
- [Environment Configuration](#environment-configuration)
- [Building Images](#building-images)
- [Running with Docker Compose](#running-with-docker-compose)
- [Scaling Services](#scaling-services)
- [Logs and Monitoring](#logs-and-monitoring)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Prerequisites

- Docker Engine 20.10+
- Docker Compose 2.0+
- At least 8GB RAM available
- At least 20GB disk space

### Initial Setup

1. **Copy environment file**:
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env` file** with your API keys:
   ```bash
   # Required: Add your LLM API keys
   ANTHROPIC_API_KEY=sk-ant-api03-...
   OPENAI_API_KEY=sk-proj-...

   # Required: Set a strong PostgreSQL password
   POSTGRES_PASSWORD=your_secure_password

   # Optional: Add exchange keys (use testnet first!)
   BINANCE_API_KEY=...
   BINANCE_API_SECRET=...
   ```

3. **Start all services**:
   ```bash
   docker-compose up -d
   ```

4. **Check service health**:
   ```bash
   docker-compose ps
   ```

5. **View logs**:
   ```bash
   docker-compose logs -f
   ```

## Architecture

CryptoFunk uses a microservices architecture with the following components:

```
┌─────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                      │
├─────────────────────────────────────────────────────────────┤
│ PostgreSQL + TimescaleDB │ Redis │ NATS │ Bifrost (LLM)    │
│ Prometheus               │ Grafana                           │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                   Application Layer                          │
├─────────────────────────────────────────────────────────────┤
│ Orchestrator (MCP Coordinator)                              │
├─────────────────────────────────────────────────────────────┤
│ MCP Servers:                                                │
│  • market-data-server    • technical-indicators-server      │
│  • risk-analyzer-server  • order-executor-server            │
├─────────────────────────────────────────────────────────────┤
│ Trading Agents:                                             │
│  • technical-agent       • orderbook-agent                  │
│  • sentiment-agent       • trend-agent                      │
│  • reversion-agent       • risk-agent                       │
├─────────────────────────────────────────────────────────────┤
│ API Server (REST + WebSocket)                               │
└─────────────────────────────────────────────────────────────┘
```

## Services Overview

### Infrastructure Services

| Service | Port | Description |
|---------|------|-------------|
| **postgres** | 5432 | PostgreSQL 17 with TimescaleDB and pgvector extensions |
| **redis** | 6379 | Redis 7 for caching and pub/sub |
| **nats** | 4222, 8222, 6222 | NATS messaging with JetStream |
| **bifrost** | 8080 | LLM gateway (Claude/GPT-4/Gemini) |
| **prometheus** | 9090 | Metrics collection and storage |
| **grafana** | 3000 | Dashboards and visualization |

### Application Services

| Service | Port | Description |
|---------|------|-------------|
| **migrate** | - | One-shot database migration |
| **orchestrator** | 8081 | MCP orchestrator (coordinates agents) |
| **market-data-server** | - | Market data + CoinGecko integration |
| **technical-indicators-server** | - | RSI, MACD, Bollinger Bands, etc. |
| **risk-analyzer-server** | - | Kelly Criterion, VaR, risk limits |
| **order-executor-server** | - | Paper/live trading execution |
| **technical-agent** | - | Technical analysis signals |
| **orderbook-agent** | - | Order book depth analysis |
| **sentiment-agent** | - | Market sentiment analysis |
| **trend-agent** | - | Trend following strategy |
| **reversion-agent** | - | Mean reversion strategy |
| **risk-agent** | - | Risk management with veto power |
| **api** | 8082 | REST/WebSocket API |

### Port Mapping Summary

- **3000**: Grafana UI
- **4222**: NATS client connections
- **5432**: PostgreSQL
- **6379**: Redis
- **8080**: Bifrost LLM Gateway
- **8081**: Orchestrator
- **8082**: API Server
- **8222**: NATS HTTP monitoring
- **9090**: Prometheus

## Environment Configuration

### Required Variables

```bash
# PostgreSQL
POSTGRES_PASSWORD=your_secure_password

# LLM API Keys (at least one required)
ANTHROPIC_API_KEY=sk-ant-api03-...
OPENAI_API_KEY=sk-proj-...

# Application
TRADING_MODE=PAPER  # PAPER or LIVE
LOG_LEVEL=info      # debug, info, warn, error
```

### Optional Variables

```bash
# Exchange API (optional for paper trading)
BINANCE_API_KEY=...
BINANCE_API_SECRET=...
COINGECKO_API_KEY=...

# API Security
JWT_SECRET=changeme_in_production
CORS_ORIGINS=*

# Grafana
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=...

# LLM (optional)
GEMINI_API_KEY=...
```

## Building Images

### Build All Images

```bash
docker-compose build
```

### Build Specific Service

```bash
docker-compose build orchestrator
docker-compose build market-data-server
docker-compose build technical-agent
```

### Build with No Cache

```bash
docker-compose build --no-cache
```

### Build Individual Images Manually

```bash
# Orchestrator
docker build -f Dockerfile.orchestrator -t cryptofunk/orchestrator:latest ../..

# MCP Server (with build arg)
docker build -f Dockerfile.mcp-server \
  --build-arg SERVER_NAME=market-data \
  -t cryptofunk/market-data-server:latest ../..

# Trading Agent (with build arg)
docker build -f Dockerfile.agent \
  --build-arg AGENT_NAME=technical-agent \
  -t cryptofunk/technical-agent:latest ../..
```

## Running with Docker Compose

### Start All Services

```bash
# Start in background
docker-compose up -d

# Start with logs
docker-compose up

# Start specific services
docker-compose up -d postgres redis nats
```

### Stop Services

```bash
# Stop all services
docker-compose down

# Stop and remove volumes (WARNING: deletes data!)
docker-compose down -v

# Stop specific service
docker-compose stop orchestrator
```

### Restart Services

```bash
# Restart all
docker-compose restart

# Restart specific service
docker-compose restart orchestrator
```

### Service Startup Order

The docker-compose.yml enforces proper startup order:

1. **Infrastructure**: postgres, redis, nats, bifrost, prometheus, grafana
2. **Migration**: migrate (waits for postgres health check)
3. **MCP Servers**: All MCP servers (wait for migrate completion)
4. **Orchestrator**: orchestrator (waits for postgres, redis, nats, bifrost)
5. **Agents**: All agents (wait for orchestrator startup)
6. **API**: api (waits for orchestrator)

## Scaling Services

### Scale Trading Agents

You can scale agents horizontally for better performance:

```bash
# Scale technical agents to 3 instances
docker-compose up -d --scale technical-agent=3

# Scale multiple agents
docker-compose up -d \
  --scale technical-agent=2 \
  --scale trend-agent=2 \
  --scale risk-agent=2
```

**Note**: Agents must support distributed coordination (implemented in Phase 5).

### Resource Limits

To add resource limits, edit `docker-compose.yml`:

```yaml
services:
  orchestrator:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

## Logs and Monitoring

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f orchestrator

# Last 100 lines
docker-compose logs --tail=100 technical-agent

# Multiple services
docker-compose logs -f orchestrator api
```

### Access Monitoring

- **Grafana**: http://localhost:3000
  - Username: `admin`
  - Password: from `GRAFANA_ADMIN_PASSWORD` in `.env`

- **Prometheus**: http://localhost:9090

- **NATS Monitoring**: http://localhost:8222

### Service Health Checks

```bash
# Check all services
docker-compose ps

# Inspect specific service
docker inspect cryptofunk-postgres

# Check health status
docker-compose ps | grep healthy
```

## Troubleshooting

### Service Won't Start

1. **Check logs**:
   ```bash
   docker-compose logs <service-name>
   ```

2. **Check dependencies**:
   ```bash
   docker-compose ps
   ```

3. **Verify environment variables**:
   ```bash
   docker-compose config
   ```

### Database Connection Issues

```bash
# Check postgres health
docker-compose exec postgres pg_isready -U postgres -d cryptofunk

# Connect to database
docker-compose exec postgres psql -U postgres -d cryptofunk

# Check migration status
docker-compose logs migrate
```

### Memory Issues

```bash
# Check resource usage
docker stats

# Increase Docker memory limit (Docker Desktop)
# Settings → Resources → Memory → Increase to 8GB+
```

### Network Issues

```bash
# List networks
docker network ls

# Inspect network
docker network inspect cryptofunk-network

# Recreate network
docker-compose down
docker network prune
docker-compose up -d
```

### Clean Start

If you need to completely reset:

```bash
# Stop all services
docker-compose down

# Remove volumes (WARNING: deletes all data!)
docker-compose down -v

# Remove images
docker-compose down --rmi all

# Clean Docker system
docker system prune -a --volumes

# Rebuild and start
docker-compose build --no-cache
docker-compose up -d
```

### Common Issues

#### "MCP server not responding"
- Check MCP server logs: `docker-compose logs <server-name>`
- Verify stdout/stderr separation (protocol on stdout, logs on stderr)
- Restart MCP servers: `docker-compose restart market-data-server`

#### "Agent connection failed"
- Verify orchestrator is running: `docker-compose ps orchestrator`
- Check NATS connectivity: `docker-compose logs nats`
- Ensure agents start after orchestrator: check `depends_on` in docker-compose.yml

#### "Database migration failed"
- Check postgres health: `docker-compose ps postgres`
- View migration logs: `docker-compose logs migrate`
- Reset database: `docker-compose down -v && docker-compose up -d postgres`

#### "Out of memory"
- Increase Docker memory limit (Settings → Resources)
- Scale down services: `docker-compose up -d --scale technical-agent=1`
- Add resource limits to docker-compose.yml

## Production Deployment

For production deployment, see:
- Kubernetes manifests in `../k8s/`
- Production hardening checklist in `../../docs/PRODUCTION.md`
- Security best practices in `../../docs/SECURITY.md`

### Production Checklist

- [ ] Set strong `POSTGRES_PASSWORD`
- [ ] Change `JWT_SECRET` to random string
- [ ] Use production LLM API keys (not test keys)
- [ ] Set `TRADING_MODE=PAPER` initially
- [ ] Configure `CORS_ORIGINS` to specific domains
- [ ] Enable HTTPS/TLS for all external endpoints
- [ ] Set up log aggregation (ELK, Loki, etc.)
- [ ] Configure backup for PostgreSQL
- [ ] Set resource limits for all services
- [ ] Enable monitoring alerts in Prometheus/Grafana
- [ ] Review and test disaster recovery procedures

## Further Reading

- [Docker Documentation](https://docs.docker.com/)
- [Docker Compose Reference](https://docs.docker.com/compose/compose-file/)
- [Dockerfile Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [CryptoFunk Architecture](../../docs/ARCHITECTURE.md)
- [MCP Integration Guide](../../docs/MCP_INTEGRATION.md)
