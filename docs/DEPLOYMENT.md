# CryptoFunk Deployment Guide

**Version:** 1.0.0
**Last Updated:** 2025-01-15

Complete guide for deploying CryptoFunk in development, staging, and production environments.

## Table of Contents

- [Deployment Options](#deployment-options)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Local Development (Docker Compose)](#local-development-docker-compose)
- [Production (Kubernetes)](#production-kubernetes)
- [Database Migrations](#database-migrations)
- [Secrets Management](#secrets-management)
- [Monitoring Setup](#monitoring-setup)
- [Rollback Procedures](#rollback-procedures)
- [Disaster Recovery](#disaster-recovery)
- [Troubleshooting](#troubleshooting)
- [Production Checklist](#production-checklist)

---

## Deployment Options

CryptoFunk supports three deployment strategies:

| Environment | Method | Use Case | Complexity |
|-------------|--------|----------|------------|
| **Development** | Docker Compose | Local development and testing | Low |
| **Staging** | Docker Compose or K8s | Pre-production validation | Medium |
| **Production** | Kubernetes | Production workloads | High |

---

## Prerequisites

### General Requirements

- **Go 1.21+** (for local builds)
- **Task** (taskfile.dev) - Build automation
- **Git** - Version control

### For Docker Compose

- **Docker Engine 20.10+**
- **Docker Compose 2.0+**
- **8GB RAM minimum** (16GB recommended)
- **20GB disk space minimum**

### For Kubernetes

- **kubectl 1.28+**
- **kustomize 5.0+** (or use kubectl built-in)
- **Kubernetes cluster 1.28+** with:
  - 3+ worker nodes (recommended)
  - 16 CPU cores total (minimum)
  - 32GB RAM total (minimum)
  - 200GB storage available
- **Ingress Controller** (NGINX recommended)
- **Dynamic storage provisioner** (or static PVs)

### API Keys (Required)

- **LLM Provider** (at least one):
  - Anthropic API key (Claude Sonnet 4)
  - OpenAI API key (GPT-4 Turbo)
  - Or configure Bifrost with other providers
- **Exchange API Keys** (for live trading):
  - Binance testnet keys (recommended for testing)
  - Binance production keys (live trading only)

---

## Quick Start

### Choose Your Path

**For Development:**
```bash
# 1. Clone repository
git clone https://github.com/ajitpratap0/cryptofunk.git
cd cryptofunk

# 2. Setup environment
cp .env.example .env
vim .env  # Add your API keys

# 3. Start all services
docker-compose up -d

# 4. Check health
curl http://localhost:8082/health
```

**For Production:**
```bash
# See "Production (Kubernetes)" section below
```

---

## Local Development (Docker Compose)

### Architecture

Docker Compose orchestrates 18 services across two layers:

```
Infrastructure Layer (6 services):
├── postgres (PostgreSQL 17 + TimescaleDB + pgvector)
├── redis (Redis 7 for caching)
├── nats (NATS with JetStream)
├── bifrost (LLM gateway)
├── prometheus (Metrics)
└── grafana (Dashboards)

Application Layer (12 services):
├── migrate (DB migrations)
├── orchestrator (MCP coordinator)
├── 4 MCP Servers (market-data, technical-indicators, risk-analyzer, order-executor)
├── 6 Trading Agents (technical, orderbook, sentiment, trend, reversion, risk)
└── api (REST + WebSocket)
```

### Step 1: Environment Setup

1. **Copy environment file:**
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env` with required variables:**
   ```bash
   # PostgreSQL (REQUIRED)
   POSTGRES_PASSWORD=your_secure_password_here

   # LLM API Keys (at least one REQUIRED)
   ANTHROPIC_API_KEY=sk-ant-api03-xxx
   OPENAI_API_KEY=sk-proj-xxx

   # Application Configuration
   TRADING_MODE=PAPER          # PAPER or LIVE (ALWAYS start with PAPER!)
   LOG_LEVEL=info              # debug, info, warn, error

   # Exchange API Keys (OPTIONAL, only for live trading)
   BINANCE_TESTNET=true        # Use testnet first!
   BINANCE_API_KEY=
   BINANCE_API_SECRET=

   # Optional: CoinGecko Pro API
   COINGECKO_API_KEY=

   # Security (IMPORTANT for production)
   JWT_SECRET=generate_a_long_random_string_here
   ```

3. **Generate secure secrets:**
   ```bash
   # Generate JWT secret (64 characters)
   openssl rand -hex 32

   # Generate PostgreSQL password
   openssl rand -base64 32
   ```

### Step 2: Start Services

```bash
# Start all services in detached mode
docker-compose up -d

# Or start with logs visible (useful for debugging)
docker-compose up

# Start only infrastructure (without app services)
docker-compose up -d postgres redis nats prometheus grafana

# Start specific services
docker-compose up -d orchestrator api
```

### Step 3: Verify Deployment

```bash
# Check all services are running
docker-compose ps

# Expected output: All services should be "Up" or "healthy"

# Check orchestrator health
curl http://localhost:8081/health

# Check API health
curl http://localhost:8082/health

# View logs
docker-compose logs -f orchestrator
docker-compose logs -f api

# View all logs
docker-compose logs --tail=100 -f
```

### Step 4: Access Services

| Service | URL | Credentials |
|---------|-----|-------------|
| **API** | http://localhost:8082 | No auth (yet) |
| **Orchestrator** | http://localhost:8081/health | Health check only |
| **Grafana** | http://localhost:3000 | admin / admin |
| **Prometheus** | http://localhost:9090 | No auth |
| **PostgreSQL** | localhost:5432 | postgres / (from .env) |
| **Redis** | localhost:6379 | No auth (local only) |

### Step 5: Initialize Trading

```bash
# Start a paper trading session via API
curl -X POST http://localhost:8082/api/v1/trade/start \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "BTC/USDT",
    "initial_capital": 10000.00,
    "mode": "PAPER"
  }'

# Connect to WebSocket for real-time updates
# See API.md for WebSocket examples
```

### Managing Docker Compose

```bash
# Stop all services (preserves data)
docker-compose stop

# Start stopped services
docker-compose start

# Restart specific service
docker-compose restart orchestrator

# Stop and remove containers (preserves volumes)
docker-compose down

# Stop, remove containers AND volumes (DESTRUCTIVE)
docker-compose down -v

# View resource usage
docker stats

# Scale a service (e.g., run 3 API instances)
docker-compose up -d --scale api=3
```

### Development Workflow

```bash
# 1. Make code changes
vim internal/orchestrator/voting.go

# 2. Rebuild specific service
docker-compose build orchestrator

# 3. Restart service
docker-compose up -d orchestrator

# 4. Check logs
docker-compose logs -f orchestrator

# Or combine steps 2-3
docker-compose up -d --build orchestrator
```

### Accessing Logs

```bash
# Follow logs for all services
docker-compose logs -f

# Follow logs for specific service
docker-compose logs -f orchestrator

# Last 100 lines
docker-compose logs --tail=100

# Since timestamp
docker-compose logs --since=2025-01-15T10:00:00

# Export logs to file
docker-compose logs > logs/cryptofunk.log
```

---

## Production (Kubernetes)

### Architecture

Kubernetes deployment consists of:

- **1 Namespace**: cryptofunk
- **13 Deployments**: Infrastructure (6) + Application (7)
- **8 Services**: Internal (ClusterIP) + External (LoadBalancer)
- **4 PersistentVolumeClaims**: postgres, redis, prometheus, grafana
- **1 Job**: Database migration
- **1 Ingress**: NGINX with TLS

### Resource Requirements

| Component | Replicas | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-----------|----------|-------------|-----------|----------------|--------------|
| postgres | 1 | 500m | 2000m | 1Gi | 4Gi |
| redis | 1 | 100m | 500m | 512Mi | 1Gi |
| nats | 1 | 100m | 500m | 256Mi | 512Mi |
| bifrost | 2 | 200m | 1000m | 512Mi | 2Gi |
| prometheus | 1 | 200m | 1000m | 1Gi | 2Gi |
| grafana | 1 | 100m | 500m | 256Mi | 1Gi |
| orchestrator | 1 | 200m | 1000m | 512Mi | 2Gi |
| MCP servers | 2 each | 100m | 500m | 256Mi | 512Mi |
| Agents | 2 each | 100m | 500m | 256Mi | 512Mi |
| API | 3 | 200m | 1000m | 512Mi | 1Gi |

**Total Minimum**: ~8 CPU cores, ~16GB RAM
**Total Maximum**: ~32 CPU cores, ~48GB RAM

### Step 1: Prepare Cluster

#### Install Required Components

```bash
# 1. NGINX Ingress Controller
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml

# 2. cert-manager (for TLS certificates)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# 3. metrics-server (for autoscaling)
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Verify installations
kubectl get pods -n ingress-nginx
kubectl get pods -n cert-manager
kubectl get deployment metrics-server -n kube-system
```

### Step 2: Build and Push Images

```bash
# 1. Navigate to project root
cd /path/to/cryptofunk

# 2. Set your registry
export REGISTRY=your-registry.io/cryptofunk
# Examples:
# - Docker Hub: docker.io/youruser/cryptofunk
# - AWS ECR: 123456789.dkr.ecr.us-east-1.amazonaws.com/cryptofunk
# - GCR: gcr.io/your-project/cryptofunk

# 3. Build all images
docker-compose -f deployments/docker-compose.yml build

# 4. Tag images for your registry
docker tag cryptofunk/orchestrator:latest $REGISTRY/orchestrator:v1.0.0
docker tag cryptofunk/api:latest $REGISTRY/api:v1.0.0
docker tag cryptofunk/market-data-server:latest $REGISTRY/market-data-server:v1.0.0
docker tag cryptofunk/technical-indicators-server:latest $REGISTRY/technical-indicators-server:v1.0.0
docker tag cryptofunk/risk-analyzer-server:latest $REGISTRY/risk-analyzer-server:v1.0.0
docker tag cryptofunk/order-executor-server:latest $REGISTRY/order-executor-server:v1.0.0
docker tag cryptofunk/technical-agent:latest $REGISTRY/technical-agent:v1.0.0
docker tag cryptofunk/trend-agent:latest $REGISTRY/trend-agent:v1.0.0
docker tag cryptofunk/risk-agent:latest $REGISTRY/risk-agent:v1.0.0
docker tag cryptofunk/migrate:latest $REGISTRY/migrate:v1.0.0

# 5. Login to registry
docker login $REGISTRY

# 6. Push all images
docker push $REGISTRY/orchestrator:v1.0.0
docker push $REGISTRY/api:v1.0.0
docker push $REGISTRY/market-data-server:v1.0.0
docker push $REGISTRY/technical-indicators-server:v1.0.0
docker push $REGISTRY/risk-analyzer-server:v1.0.0
docker push $REGISTRY/order-executor-server:v1.0.0
docker push $REGISTRY/technical-agent:v1.0.0
docker push $REGISTRY/trend-agent:v1.0.0
docker push $REGISTRY/risk-agent:v1.0.0
docker push $REGISTRY/migrate:v1.0.0
```

### Step 3: Configure Secrets

```bash
cd deployments/k8s/base

# Create secrets file from template
cp secrets.yaml.example secrets.yaml

# Generate base64-encoded secrets
echo -n "your_postgres_password" | base64
echo -n "sk-ant-api03-your-anthropic-key" | base64
echo -n "sk-proj-your-openai-key" | base64
echo -n "your_binance_api_key" | base64
echo -n "your_binance_api_secret" | base64
echo -n "your_jwt_secret" | base64

# Edit secrets.yaml with encoded values
vim secrets.yaml

# IMPORTANT: Never commit secrets.yaml to git!
# Add to .gitignore if not already present
```

**secrets.yaml Template:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cryptofunk-secrets
  namespace: cryptofunk
type: Opaque
data:
  postgres-password: <base64-encoded-password>
  anthropic-api-key: <base64-encoded-key>
  openai-api-key: <base64-encoded-key>
  binance-api-key: <base64-encoded-key>
  binance-api-secret: <base64-encoded-secret>
  jwt-secret: <base64-encoded-secret>
```

### Step 4: Update Configuration

```bash
# 1. Edit ConfigMap for your environment
vim configmap.yaml

# Key settings to update:
# - TRADING_MODE (PAPER or LIVE)
# - LOG_LEVEL (info recommended for production)
# - CORS_ORIGINS (your frontend domains)

# 2. Edit kustomization.yaml with your registry
vim kustomization.yaml

# Update image registry:
images:
  - name: cryptofunk/orchestrator
    newName: your-registry.io/cryptofunk/orchestrator
    newTag: v1.0.0
  # ... repeat for all images

# 3. Edit ingress.yaml with your domain
vim ingress.yaml

# Update hosts:
spec:
  rules:
    - host: api.cryptofunk.example.com  # Your domain
    - host: grafana.cryptofunk.example.com  # Your domain
```

### Step 5: Deploy to Kubernetes

```bash
# 1. Create namespace
kubectl apply -f namespace.yaml

# 2. Apply all manifests
kubectl apply -k .

# Or use kustomize separately
kustomize build . | kubectl apply -f -

# 3. Check deployment status
kubectl get all -n cryptofunk

# 4. Watch pod startup
kubectl get pods -n cryptofunk -w

# 5. Check migration job completed
kubectl get jobs -n cryptofunk
kubectl logs -n cryptofunk job/migrate

# 6. Check services
kubectl get svc -n cryptofunk

# 7. Check ingress
kubectl get ingress -n cryptofunk
```

### Step 6: Verify Deployment

```bash
# 1. Check pod status (all should be Running)
kubectl get pods -n cryptofunk

# 2. Check service endpoints
kubectl get endpoints -n cryptofunk

# 3. Test orchestrator health
kubectl port-forward -n cryptofunk svc/orchestrator 8081:8080
curl http://localhost:8081/health

# 4. Test API health
kubectl port-forward -n cryptofunk svc/api 8082:8080
curl http://localhost:8082/health

# 5. Check logs
kubectl logs -n cryptofunk deployment/orchestrator --tail=50
kubectl logs -n cryptofunk deployment/api --tail=50

# 6. Check resource usage
kubectl top pods -n cryptofunk
kubectl top nodes
```

### Step 7: Configure DNS and TLS

```bash
# 1. Get Ingress external IP
kubectl get ingress -n cryptofunk cryptofunk-ingress

# 2. Create DNS A records:
# api.cryptofunk.example.com -> <EXTERNAL-IP>
# grafana.cryptofunk.example.com -> <EXTERNAL-IP>

# 3. Create TLS certificate (using cert-manager)
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: cryptofunk-tls
  namespace: cryptofunk
spec:
  secretName: cryptofunk-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
    - api.cryptofunk.example.com
    - grafana.cryptofunk.example.com
EOF

# 4. Check certificate status
kubectl describe certificate cryptofunk-tls -n cryptofunk

# 5. Test HTTPS
curl https://api.cryptofunk.example.com/health
```

### Scaling

#### Manual Scaling

```bash
# Scale API for higher load
kubectl scale deployment/api --replicas=5 -n cryptofunk

# Scale agents for more processing
kubectl scale deployment/trend-agent --replicas=3 -n cryptofunk

# Check scaling status
kubectl get deployments -n cryptofunk
```

#### Horizontal Pod Autoscaler (HPA)

```bash
# Create HPA for API (scale 3-10 based on CPU)
kubectl autoscale deployment api -n cryptofunk \
  --min=3 --max=10 --cpu-percent=70

# Create HPA for orchestrator
kubectl autoscale deployment orchestrator -n cryptofunk \
  --min=1 --max=3 --cpu-percent=80

# Check HPA status
kubectl get hpa -n cryptofunk

# Describe HPA for details
kubectl describe hpa api -n cryptofunk
```

---

## Database Migrations

### Overview

CryptoFunk uses SQL migrations in `migrations/` directory:
- **001_initial_schema.sql**: Complete schema (TimescaleDB + pgvector)
- **002_semantic_memory.sql**: Semantic memory tables (Phase 11)
- **003_procedural_memory.sql**: Procedural memory tables (Phase 11)

### Running Migrations

#### Docker Compose

Migrations run automatically on startup via the `migrate` service.

**Manual migration:**
```bash
# Run migration container
docker-compose run --rm migrate

# Check migration status
docker-compose run --rm migrate -status

# Force specific version
docker-compose run --rm migrate -version 2
```

#### Kubernetes

Migrations run as a Kubernetes Job before app services start.

**Check migration status:**
```bash
# Check job completion
kubectl get jobs -n cryptofunk

# View migration logs
kubectl logs -n cryptofunk job/migrate

# Re-run migration job (if needed)
kubectl delete job migrate -n cryptofunk
kubectl apply -f job-migrate.yaml
```

#### Manual Migration (Direct SQL)

```bash
# Connect to database
kubectl exec -it -n cryptofunk deployment/postgres -- psql -U postgres cryptofunk

# Check migration status
SELECT * FROM schema_version;

# Run specific migration
\i /migrations/002_semantic_memory.sql
```

### Creating New Migrations

```bash
# 1. Create migration file
cd migrations
vim 004_your_migration.sql

# 2. Add SQL statements
CREATE TABLE IF NOT EXISTS new_table (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

# 3. Test locally
docker-compose run --rm migrate

# 4. Commit migration
git add migrations/004_your_migration.sql
git commit -m "feat: Add new_table migration (004)"
```

### Migration Rollback

**Warning**: CryptoFunk does not currently support automatic rollbacks. Create manual rollback scripts:

```bash
# Create rollback file
cd migrations
vim 004_your_migration_rollback.sql

# Add rollback SQL
DROP TABLE IF EXISTS new_table;

# Apply rollback manually if needed
psql -U postgres cryptofunk < migrations/004_your_migration_rollback.sql
```

---

## Secrets Management

### Current Approach (Phase 10 - In Progress)

Secrets are stored as Kubernetes Secrets or environment variables. This is **NOT production-ready**.

### Planned: HashiCorp Vault (Phase 10, Task T273)

**Architecture:**
```
Application → Vault Agent Injector → HashiCorp Vault → Secrets
```

**Configuration (Future):**
```yaml
# Vault integration (planned)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cryptofunk
  namespace: cryptofunk
---
apiVersion: v1
kind: Pod
metadata:
  annotations:
    vault.hashicorp.com/agent-inject: "true"
    vault.hashicorp.com/role: "cryptofunk"
    vault.hashicorp.com/agent-inject-secret-config: "secret/data/cryptofunk/api-keys"
spec:
  serviceAccountName: cryptofunk
```

### Best Practices (Until Vault is Integrated)

1. **Never commit secrets to git**:
   ```bash
   # Add to .gitignore
   echo "deployments/k8s/base/secrets.yaml" >> .gitignore
   echo ".env" >> .gitignore
   ```

2. **Use strong secrets**:
   ```bash
   # Generate strong passwords
   openssl rand -base64 32

   # Generate UUIDs for JWT secrets
   uuidgen
   ```

3. **Rotate secrets regularly**:
   ```bash
   # Update Kubernetes secret
   kubectl create secret generic cryptofunk-secrets -n cryptofunk \
     --from-literal=postgres-password=NEW_PASSWORD \
     --dry-run=client -o yaml | kubectl apply -f -

   # Restart pods to pick up new secrets
   kubectl rollout restart deployment/orchestrator -n cryptofunk
   ```

4. **Use separate secrets per environment**:
   - Development: Weak passwords OK
   - Staging: Production-like but separate
   - Production: Strong secrets, rotated monthly

---

## Monitoring Setup

### Access Dashboards

**Docker Compose:**
- Grafana: http://localhost:3000 (admin / admin)
- Prometheus: http://localhost:9090

**Kubernetes:**
```bash
# Port-forward Grafana
kubectl port-forward -n cryptofunk svc/grafana 3000:3000

# Port-forward Prometheus
kubectl port-forward -n cryptofunk svc/prometheus 9090:9090

# Or access via Ingress (if configured)
https://grafana.cryptofunk.example.com
```

### Grafana Dashboards

**Available Dashboards** (Phase 10, Task T276 - Not Yet Implemented):
1. **System Overview**: CPU, memory, disk, network
2. **Trading Performance**: P&L, trades, win rate, Sharpe ratio
3. **Agent Performance**: Signal accuracy, latency, LLM costs
4. **Risk Metrics**: Drawdown, VaR, position sizes

**Import Dashboards** (once created):
```bash
# Copy dashboard JSON to Grafana
kubectl cp grafana/dashboards/trading-performance.json \
  cryptofunk/grafana-pod:/var/lib/grafana/dashboards/

# Or use Grafana UI: Create → Import → Upload JSON
```

### Prometheus Metrics

**Available Metrics:**
- System: `node_cpu_seconds_total`, `node_memory_MemAvailable_bytes`
- Application: `http_requests_total`, `http_request_duration_seconds`
- Trading: `position_count`, `order_count`, `pnl_total`

**Query Examples:**
```promql
# HTTP request rate
rate(http_requests_total[5m])

# Average response time
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])

# Open positions
sum(position_count{status="open"})
```

### Alerting (Phase 10, Task T278 - Not Yet Implemented)

**Planned Alerts:**
- System: High CPU (>80%), low memory (<20%), disk full (>90%)
- Trading: Max drawdown exceeded, unusual loss streak, agent down
- Database: Connection failures, slow queries (>1s)

---

## Rollback Procedures

### Docker Compose Rollback

```bash
# 1. Stop current version
docker-compose down

# 2. Checkout previous version
git checkout v0.9.0

# 3. Rebuild and start
docker-compose up -d --build

# 4. Verify
curl http://localhost:8082/health
```

### Kubernetes Rollback

#### Method 1: Rollout Undo (Recommended)

```bash
# Check rollout history
kubectl rollout history deployment/orchestrator -n cryptofunk

# Rollback to previous version
kubectl rollout undo deployment/orchestrator -n cryptofunk

# Rollback to specific revision
kubectl rollout undo deployment/orchestrator -n cryptofunk --to-revision=2

# Check rollback status
kubectl rollout status deployment/orchestrator -n cryptofunk

# Rollback all deployments
kubectl rollout undo deployment --all -n cryptofunk
```

#### Method 2: Manual Revert

```bash
# 1. Update kustomization.yaml with previous image tag
vim deployments/k8s/base/kustomization.yaml

# Change:
# newTag: v1.1.0
# To:
# newTag: v1.0.0

# 2. Apply changes
kubectl apply -k deployments/k8s/base/

# 3. Monitor rollout
kubectl get pods -n cryptofunk -w
```

#### Method 3: Git Revert

```bash
# 1. Revert to previous git commit
git log --oneline  # Find commit hash
git checkout abc123def

# 2. Redeploy
kubectl apply -k deployments/k8s/base/

# 3. Verify
kubectl get deployments -n cryptofunk
```

### Database Rollback

**Warning**: Database rollbacks are **DESTRUCTIVE**. Always backup first.

```bash
# 1. Stop all app services
kubectl scale deployment --all --replicas=0 -n cryptofunk

# 2. Backup current database
kubectl exec -n cryptofunk deployment/postgres -- \
  pg_dump -U postgres cryptofunk > backup_pre_rollback.sql

# 3. Apply rollback migration
kubectl exec -it -n cryptofunk deployment/postgres -- psql -U postgres cryptofunk
\i /path/to/rollback_migration.sql

# 4. Restart services
kubectl scale deployment --all --replicas=1 -n cryptofunk
```

---

## Disaster Recovery

### Backup Strategy

#### Database Backups

**Automated Backups** (Phase 10, Task T288 - Not Yet Implemented):
```bash
# Planned: Automated pg_dump cronjob
# Retention: Daily (7 days), Weekly (4 weeks), Monthly (12 months)
```

**Manual Backup:**
```bash
# Docker Compose
docker-compose exec postgres pg_dump -U postgres cryptofunk > backup_$(date +%Y%m%d).sql

# Kubernetes
kubectl exec -n cryptofunk deployment/postgres -- \
  pg_dump -U postgres cryptofunk | gzip > backup_$(date +%Y%m%d).sql.gz

# Upload to S3 (or similar)
aws s3 cp backup_$(date +%Y%m%d).sql.gz s3://your-bucket/backups/
```

#### Configuration Backups

```bash
# Backup Kubernetes manifests
kubectl get all -n cryptofunk -o yaml > k8s_backup_$(date +%Y%m%d).yaml

# Backup ConfigMaps and Secrets
kubectl get configmap,secret -n cryptofunk -o yaml > config_backup_$(date +%Y%m%d).yaml
```

### Restore Procedures

#### Database Restore

```bash
# Docker Compose
docker-compose exec -T postgres psql -U postgres cryptofunk < backup_20250115.sql

# Kubernetes
kubectl exec -i -n cryptofunk deployment/postgres -- \
  psql -U postgres cryptofunk < backup_20250115.sql

# Or from S3
aws s3 cp s3://your-bucket/backups/backup_20250115.sql.gz - | \
  gunzip | \
  kubectl exec -i -n cryptofunk deployment/postgres -- psql -U postgres cryptofunk
```

#### Full System Restore

```bash
# 1. Restore database (see above)

# 2. Redeploy application
kubectl apply -k deployments/k8s/base/

# 3. Verify all pods running
kubectl get pods -n cryptofunk

# 4. Run smoke tests
curl https://api.cryptofunk.example.com/health
```

### RPO/RTO Targets

**Recovery Point Objective (RPO)**: 1 hour
- Database backed up hourly to S3
- Maximum data loss: 1 hour of trading data

**Recovery Time Objective (RTO)**: 15 minutes
- Automated Kubernetes deployment
- Database restore: ~10 minutes (for 100GB database)
- Service startup: ~5 minutes

---

## Troubleshooting

### Common Issues

#### Issue 1: Pod CrashLoopBackOff

```bash
# Check pod status
kubectl get pods -n cryptofunk

# View pod logs
kubectl logs -n cryptofunk <pod-name>

# Describe pod for events
kubectl describe pod -n cryptofunk <pod-name>

# Common causes:
# - Missing environment variables
# - Database connection failure
# - Invalid API keys
# - Insufficient resources
```

**Solution:**
```bash
# Check ConfigMap and Secrets are applied
kubectl get configmap -n cryptofunk
kubectl get secret -n cryptofunk

# Check resource limits
kubectl top pods -n cryptofunk

# Restart pod
kubectl delete pod -n cryptofunk <pod-name>
```

#### Issue 2: Database Connection Failures

```bash
# Check PostgreSQL pod
kubectl get pods -n cryptofunk | grep postgres

# Check PostgreSQL logs
kubectl logs -n cryptofunk deployment/postgres

# Test connection
kubectl exec -it -n cryptofunk deployment/postgres -- psql -U postgres cryptofunk
```

**Solution:**
```bash
# Verify DATABASE_URL in ConfigMap
kubectl get configmap cryptofunk-config -n cryptofunk -o yaml | grep DATABASE_URL

# Check password in Secret
kubectl get secret cryptofunk-secrets -n cryptofunk -o jsonpath='{.data.postgres-password}' | base64 -d

# Restart database pod
kubectl delete pod -n cryptofunk <postgres-pod-name>
```

#### Issue 3: Ingress Not Working

```bash
# Check Ingress status
kubectl get ingress -n cryptofunk

# Describe Ingress
kubectl describe ingress cryptofunk-ingress -n cryptofunk

# Check Ingress Controller logs
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller
```

**Solution:**
```bash
# Verify DNS resolves to Ingress IP
dig api.cryptofunk.example.com

# Check TLS certificate
kubectl get certificate -n cryptofunk

# Manually test service
kubectl port-forward -n cryptofunk svc/api 8082:8080
curl http://localhost:8082/health
```

#### Issue 4: High Memory Usage

```bash
# Check resource usage
kubectl top pods -n cryptofunk
kubectl top nodes

# Check OOMKilled events
kubectl get events -n cryptofunk | grep OOMKilled

# Describe pod for resource limits
kubectl describe pod -n cryptofunk <pod-name>
```

**Solution:**
```bash
# Increase memory limits in deployment
kubectl edit deployment <deployment-name> -n cryptofunk

# Update resources:
spec:
  template:
    spec:
      containers:
      - name: orchestrator
        resources:
          requests:
            memory: "1Gi"
          limits:
            memory: "3Gi"

# Or scale down replicas temporarily
kubectl scale deployment <deployment-name> --replicas=1 -n cryptofunk
```

### Getting Help

For additional support:
1. Check logs: `kubectl logs -n cryptofunk <pod-name>`
2. Review documentation: `docs/` directory
3. GitHub Issues: https://github.com/ajitpratap0/cryptofunk/issues
4. Discussion Forum: (TBD)

---

## Production Checklist

Before deploying to production, verify:

### Security
- [ ] All secrets stored in Vault or Kubernetes Secrets (not in code)
- [ ] Strong passwords generated (min 32 characters)
- [ ] JWT secret is unique and secure
- [ ] API keys rotated regularly (monthly)
- [ ] TLS enabled for all external endpoints
- [ ] CORS configured with allowed origins only
- [ ] Network policies configured (if required)
- [ ] Pod security policies applied
- [ ] No debug mode enabled (`LOG_LEVEL=info` or `warn`)
- [ ] Secrets manager integrated (Task T273)

### Configuration
- [ ] `TRADING_MODE=PAPER` for initial deployment (CRITICAL!)
- [ ] Correct exchange API keys (testnet first)
- [ ] Database connection string correct
- [ ] Redis and NATS endpoints correct
- [ ] LLM API keys valid and tested
- [ ] Resource requests and limits set appropriately
- [ ] Health check endpoints configured
- [ ] Monitoring enabled (Prometheus + Grafana)

### Infrastructure
- [ ] Kubernetes cluster provisioned (min 3 nodes)
- [ ] Ingress Controller installed
- [ ] Storage provisioner configured
- [ ] DNS records created
- [ ] TLS certificates configured
- [ ] Load balancer provisioned
- [ ] Backup storage configured (S3 or equivalent)
- [ ] Firewall rules configured

### Application
- [ ] All Docker images built and pushed
- [ ] Database migrations run successfully
- [ ] All pods running and healthy
- [ ] Health checks passing
- [ ] Logs streaming to monitoring system
- [ ] Metrics visible in Prometheus
- [ ] Grafana dashboards configured (Task T276)
- [ ] Alerting rules configured (Task T278)

### Testing
- [ ] Smoke tests pass
- [ ] Integration tests pass (Task T261)
- [ ] Load tests pass (Task T265)
- [ ] Failover tested (pod restarts, node failures)
- [ ] Database backup/restore tested
- [ ] Rollback procedure tested

### Documentation
- [ ] Deployment runbook updated
- [ ] Incident response playbook created (Task T287)
- [ ] On-call rotation established
- [ ] Team trained on deployment procedures

### Compliance
- [ ] LICENSE file present (MIT)
- [ ] Terms of service reviewed
- [ ] Privacy policy (if collecting user data)
- [ ] Trading regulations reviewed (varies by jurisdiction)
- [ ] Audit trail enabled

---

## Next Steps

After successful deployment:

1. **Monitor System Health**:
   - Check Grafana dashboards every hour (first 24h)
   - Review logs for errors
   - Monitor resource usage

2. **Start Paper Trading**:
   - Run with `TRADING_MODE=PAPER` for at least 1 week
   - Validate strategy performance
   - Tune risk parameters

3. **Gradual Rollout**:
   - Start with small capital (~$100)
   - Increase gradually after proven performance
   - Monitor closely for first 48 hours

4. **Ongoing Maintenance**:
   - Review logs weekly
   - Update dependencies monthly
   - Rotate secrets monthly
   - Test backups monthly

---

## Additional Resources

- **API Documentation**: [docs/API.md](API.md)
- **MCP Integration Guide**: [docs/MCP_INTEGRATION.md](MCP_INTEGRATION.md)
- **Architecture Overview**: [docs/ARCHITECTURE.md](ARCHITECTURE.md)
- **Contributing**: [CONTRIBUTING.md](../CONTRIBUTING.md)
- **Task Tracking**: [TASKS.md](../TASKS.md)

---

**Last Updated:** 2025-01-15
**Maintained By:** CryptoFunk Team
**Version:** 1.0.0
