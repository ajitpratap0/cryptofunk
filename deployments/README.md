# CryptoFunk Deployment Guide

This directory contains deployment configurations for running CryptoFunk in different environments.

## Quick Start (Local Development)

1. **Copy environment file**:
```bash
cp .env.example .env
# Edit .env and set secure passwords
```

2. **Start services**:
```bash
docker-compose up -d
```

3. **Check service health**:
```bash
docker-compose ps
curl http://localhost:8081/health  # Orchestrator
curl http://localhost:8080/health  # API
```

4. **View logs**:
```bash
docker-compose logs -f
docker-compose logs -f orchestrator  # Specific service
```

5. **Stop services**:
```bash
docker-compose down
# To remove volumes (fresh start):
docker-compose down -v
```

## Services

### Infrastructure Services

#### PostgreSQL (Port 5432)
- **Image**: timescale/timescaledb-ha:pg15-latest
- **Extensions**: TimescaleDB, pgvector, uuid-ossp
- **Default Password**: Set via `POSTGRES_PASSWORD` in .env
- **Volume**: `postgres_data`
- **Access**: `psql postgresql://postgres:password@localhost:5432/cryptofunk`

#### Redis (Port 6379)
- **Image**: redis:7-alpine
- **Default Password**: Set via `REDIS_PASSWORD` in .env
- **Volume**: `redis_data`
- **Access**: `redis-cli -h localhost -p 6379 -a password`

#### NATS (Ports 4222, 8222)
- **Image**: nats:2.10-alpine
- **Client Port**: 4222
- **Monitoring Port**: 8222
- **Volume**: `nats_data`
- **Monitoring**: http://localhost:8222

#### Prometheus (Port 9090)
- **Image**: prom/prometheus:latest
- **Config**: `prometheus/prometheus.yml`
- **Volume**: `prometheus_data`
- **UI**: http://localhost:9090

#### Grafana (Port 3000)
- **Image**: grafana/grafana:latest
- **Default Credentials**: admin / (set via `GRAFANA_ADMIN_PASSWORD` in .env)
- **Volume**: `grafana_data`
- **UI**: http://localhost:3000

### Application Services

#### Orchestrator (Port 8081)
- **Description**: MCP orchestrator coordinating all trading agents
- **Health**: http://localhost:8081/health
- **Metrics**: http://localhost:8081/metrics

#### API Server (Port 8080)
- **Description**: REST API and WebSocket server
- **Health**: http://localhost:8080/health
- **API**: http://localhost:8080/api/v1/
- **WebSocket**: ws://localhost:8080/api/v1/ws

#### Migrate
- **Description**: Database migration service (runs once on startup)
- **Behavior**: Applies migrations and exits

## Directory Structure

```
deployments/
├── docker-compose.yml              # Main compose file
├── .env.example                    # Environment variables template
├── .env                            # Your local environment (gitignored)
├── init-db.sh                      # Database initialization script
├── docker/                         # Dockerfiles
│   ├── Dockerfile.orchestrator
│   ├── Dockerfile.api
│   ├── Dockerfile.agent
│   ├── Dockerfile.mcp-server
│   ├── Dockerfile.migrate
│   └── Dockerfile.backtest
├── prometheus/
│   └── prometheus.yml              # Prometheus configuration
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/            # Data source configs
│   │   └── dashboards/             # Dashboard provisioning
│   └── dashboards/                 # Dashboard JSON files
└── k8s/                            # Kubernetes manifests
```

## Environment Variables

See `.env.example` for all available variables. Key variables:

### Required
- `POSTGRES_PASSWORD`: PostgreSQL password (**CHANGE IN PRODUCTION**)
- `REDIS_PASSWORD`: Redis password (**CHANGE IN PRODUCTION**)
- `GRAFANA_ADMIN_PASSWORD`: Grafana admin password (**CHANGE IN PRODUCTION**)

### Optional
- `ENVIRONMENT`: development, staging, or production (default: development)
- `LOG_LEVEL`: debug, info, warn, error (default: info)
- `TRADING_MODE`: paper or live (default: paper)
- `INITIAL_CAPITAL`: Starting capital for trading (default: 10000)

### Exchange API Keys (for live trading)
- `BINANCE_API_KEY`
- `BINANCE_API_SECRET`

### LLM API Keys
- `OPENAI_API_KEY`
- `ANTHROPIC_API_KEY`

## Common Tasks

### Accessing Services

**PostgreSQL**:
```bash
# Via docker exec
docker exec -it cryptofunk-postgres psql -U postgres -d cryptofunk

# Via local psql
psql postgresql://postgres:password@localhost:5432/cryptofunk
```

**Redis**:
```bash
# Via docker exec
docker exec -it cryptofunk-redis redis-cli -a password

# Via local redis-cli
redis-cli -h localhost -p 6379 -a password
```

**NATS**:
```bash
# Monitor connections
curl http://localhost:8222/connz

# View subscriptions
curl http://localhost:8222/subsz
```

### Database Operations

**Run migrations**:
```bash
docker-compose run --rm migrate
```

**Reset database**:
```bash
docker-compose down
docker volume rm deployments_postgres_data
docker-compose up -d
```

**Backup database**:
```bash
docker exec cryptofunk-postgres pg_dump -U postgres cryptofunk > backup.sql
```

**Restore database**:
```bash
docker exec -i cryptofunk-postgres psql -U postgres -d cryptofunk < backup.sql
```

### Monitoring

**Prometheus Targets**: http://localhost:9090/targets

**Grafana Dashboards**:
1. Navigate to http://localhost:3000
2. Login with admin credentials
3. Browse dashboards under "CryptoFunk Dashboards"

**Service Logs**:
```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f orchestrator

# Last 100 lines
docker-compose logs --tail=100 api
```

## Troubleshooting

### Service Won't Start

1. **Check logs**:
```bash
docker-compose logs service-name
```

2. **Check health**:
```bash
docker-compose ps
```

3. **Restart service**:
```bash
docker-compose restart service-name
```

### Database Connection Issues

1. **Verify PostgreSQL is running**:
```bash
docker-compose ps postgres
```

2. **Check PostgreSQL logs**:
```bash
docker-compose logs postgres
```

3. **Test connection**:
```bash
docker exec cryptofunk-postgres pg_isready -U postgres
```

### Port Conflicts

If ports are already in use, modify `docker-compose.yml`:
```yaml
services:
  postgres:
    ports:
      - "15432:5432"  # Use different external port
```

### Permission Issues

**On Linux**, you may need to adjust file ownership:
```bash
sudo chown -R $USER:$USER .
```

## Security Best Practices

### Development
- Use `.env` file for local secrets
- Never commit `.env` to git
- Use strong passwords even in development

### Production
- **NEVER** use default passwords
- Use secrets management (Vault, AWS Secrets Manager)
- Enable SSL for all services
- Use firewalls to restrict access
- Rotate secrets regularly
- Monitor for security vulnerabilities
- Enable audit logging

## Kubernetes Deployment

See `k8s/README.md` for Kubernetes deployment instructions.

## Production Checklist

Before deploying to production:

- [ ] Change all default passwords in `.env`
- [ ] Enable SSL/TLS for all services
- [ ] Configure proper resource limits
- [ ] Set up backup strategy
- [ ] Configure alerting
- [ ] Enable audit logging
- [ ] Review security settings
- [ ] Test disaster recovery procedures
- [ ] Configure monitoring dashboards
- [ ] Set up log aggregation
- [ ] Configure automated updates
- [ ] Review and test circuit breakers

## Support

For issues or questions:
- Check logs: `docker-compose logs -f`
- Review documentation: `../docs/`
- Check GitHub issues: https://github.com/yourusername/cryptofunk/issues
