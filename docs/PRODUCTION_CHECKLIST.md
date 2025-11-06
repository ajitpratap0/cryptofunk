# Production Deployment Checklist

**Version**: 1.0  
**Last Updated**: 2025-11-06  
**Status**: Pre-Production  

This checklist ensures CryptoFunk is production-ready before deployment. Complete all sections and check off items as you verify them.

---

## Table of Contents

1. [Pre-Deployment Security Review](#1-pre-deployment-security-review)
2. [Performance Review](#2-performance-review)
3. [Configuration Review](#3-configuration-review)
4. [Infrastructure Readiness](#4-infrastructure-readiness)
5. [Monitoring & Alerting](#5-monitoring--alerting)
6. [Documentation Review](#6-documentation-review)
7. [Deployment Steps](#7-deployment-steps)
8. [Post-Deployment Verification](#8-post-deployment-verification)
9. [Rollback Procedures](#9-rollback-procedures)
10. [Sign-off](#10-sign-off)

---

## 1. Pre-Deployment Security Review

### API Keys & Secrets

- [ ] All production API keys generated (separate from dev/test)
- [ ] API keys stored in secure secrets manager (Vault/AWS Secrets Manager)
- [ ] No API keys committed to git (run `git log -p | grep -i "api.*key"`)
- [ ] Exchange API keys have minimal required permissions
  - [ ] Read permission enabled
  - [ ] Trade permission enabled
  - [ ] **Withdraw permission DISABLED** (critical!)
- [ ] IP whitelist configured on exchange (if supported)
- [ ] API key expiration/rotation policy documented
- [ ] Verify keys work: `./bin/orchestrator --verify-keys`

**Verification Commands:**
```bash
# Check for leaked secrets
git log --all -- '*.env' '*.yaml' | grep -i "key\|secret\|password"
grep -r "AKIA" .  # AWS keys
grep -r "sk_live" .  # Stripe keys

# Verify no secrets in code
./scripts/security/scan-secrets.sh  # If available
```

### Password & Authentication

- [ ] Database password is strong (12+ chars, mixed case, numbers, symbols)
- [ ] Redis password set (if exposed to network)
- [ ] Grafana default password changed
- [ ] Prometheus admin auth enabled (if exposing metrics)
- [ ] JWT secret generated (32+ random bytes)
- [ ] SSH keys rotated for servers
- [ ] No default passwords in use (`admin`, `changeme`, etc.)

**Generate Strong Passwords:**
```bash
# Database password
openssl rand -base64 32

# JWT secret
openssl rand -hex 32

# Redis password
pwgen -s 32 1
```

### SSL/TLS Configuration

- [ ] SSL enabled for database connections (`ssl_mode: require`)
- [ ] HTTPS enabled for API endpoints
- [ ] Valid SSL certificates obtained (Let's Encrypt or commercial)
- [ ] Certificate auto-renewal configured
- [ ] TLS 1.2+ only (1.0/1.1 disabled)
- [ ] Strong cipher suites configured

### Network Security

- [ ] Firewall rules configured (only necessary ports open)
- [ ] Database not accessible from public internet
- [ ] Redis not accessible from public internet
- [ ] Rate limiting enabled on API
- [ ] DDoS protection in place (Cloudflare, AWS Shield, etc.)
- [ ] VPN or bastion host for admin access

**Required Open Ports:**
```
80/443  - HTTP/HTTPS (API, Grafana)
22      - SSH (admin only, IP restricted)

Internal only:
5432    - PostgreSQL
6379    - Redis
4222    - NATS
9090    - Prometheus
3000    - Grafana
```

### Code Security

- [ ] All dependencies up to date (`go list -u -m all`)
- [ ] No known vulnerabilities (`go list -json -m all | nancy sleuth`)
- [ ] Input validation on all API endpoints
- [ ] SQL injection prevented (parameterized queries)
- [ ] XSS prevention in web UI
- [ ] CORS configured properly
- [ ] No debug endpoints exposed in production

### Compliance & Legal

- [ ] Trading bot disclosure (if required by jurisdiction)
- [ ] Terms of service reviewed
- [ ] Privacy policy in place (if collecting user data)
- [ ] GDPR compliance (if EU users)
- [ ] Financial regulations reviewed
- [ ] Liability disclaimers in place

---

## 2. Performance Review

### Database Optimization

- [ ] Database properly indexed
  - [ ] Check slow queries: `SELECT * FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;`
  - [ ] Add missing indexes if found
- [ ] TimescaleDB hypertables configured
  - [ ] Candlesticks table: chunk interval = 1 day
  - [ ] Compression enabled for old data (>7 days)
- [ ] Connection pooling configured (10-20 connections)
- [ ] Vacuum and analyze scheduled (`VACUUM ANALYZE` weekly)
- [ ] Database size monitored (set alerts at 80% capacity)

**Performance Check:**
```bash
# Check database performance
psql -h $DB_HOST -U postgres -d cryptofunk << SQL
SELECT schemaname, tablename, pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename))
FROM pg_tables
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 10;
SQL

# Check indexes
psql -h $DB_HOST -U postgres -d cryptofunk -c "\di+"
```

### Cache Configuration

- [ ] Redis cache hit rate >80%
  - Check: `redis-cli INFO stats | grep keyspace_hits`
- [ ] Cache TTLs appropriate (60s for prices, 5min for OHLCV)
- [ ] Redis maxmemory policy set (`allkeys-lru`)
- [ ] Redis persistence configured (AOF or RDB)

### Resource Limits

- [ ] CPU limits appropriate (not hitting 100% sustained)
- [ ] Memory limits appropriate (90% max, with headroom)
- [ ] Disk I/O not saturated
- [ ] Network bandwidth sufficient
- [ ] Connection limits not hit

**Resource Monitoring:**
```bash
# Check current resource usage
docker stats --no-stream

# Kubernetes resource usage
kubectl top pods -n cryptofunk
kubectl top nodes
```

### Load Testing

- [ ] Load tests completed (simulate 100+ concurrent users)
- [ ] API response times <200ms for 95th percentile
- [ ] Database queries <50ms for 95th percentile
- [ ] MCP tool calls <100ms for 95th percentile
- [ ] No memory leaks detected in 24hr soak test
- [ ] Graceful degradation under load

**Run Load Tests:**
```bash
# API load test
ab -n 10000 -c 100 http://localhost:8080/health

# Or use k6
k6 run scripts/load-test.js

# Check for memory leaks
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

### Optimization Applied

- [ ] Database queries optimized (no N+1 queries)
- [ ] Caching strategy implemented
- [ ] Static assets compressed (gzip)
- [ ] Connection pooling enabled
- [ ] Inefficient code paths refactored

---

## 3. Configuration Review

### Environment-Specific Configuration

- [ ] `app.environment` set to `production`
- [ ] `trading.mode` explicitly set (`paper` or `live`)
- [ ] Log level set to `info` or `warn` (not `debug`)
- [ ] Testnet disabled (`testnet: false`)
- [ ] Development features disabled

**Verify Configuration:**
```bash
# Run configuration validation
./bin/orchestrator --verify-keys

# Check critical settings
grep -E "environment|trading.*mode|testnet|log_level" configs/config.yaml
```

### Risk Parameters

- [ ] Risk parameters reviewed by human trader
- [ ] `max_position_size` appropriate (5-20% recommended)
- [ ] `max_daily_loss` set (1-5% recommended)
- [ ] `max_drawdown` set (5-15% recommended)
- [ ] Stop loss mandatory
- [ ] Position sizing uses Kelly Criterion
- [ ] Circuit breakers configured and tested

**Current Risk Settings:**
```yaml
risk:
  max_position_size: 0.10      # 10% max per position
  max_daily_loss: 0.02         # 2% daily loss limit
  max_drawdown: 0.10           # 10% max drawdown
  default_stop_loss: 0.02      # 2% stop loss
  default_take_profit: 0.05    # 5% take profit
  min_confidence: 0.70         # 70% min confidence
```

- [ ] Above settings match your risk tolerance
- [ ] Settings tested in paper trading for 7+ days

### Trading Parameters

- [ ] Trading pairs validated (sufficient liquidity)
- [ ] Initial capital set correctly
- [ ] Position limits appropriate
- [ ] Order sizes within exchange limits
- [ ] Leverage disabled or strictly limited

### External Service Configuration

- [ ] Exchange API endpoints correct (not testnet)
- [ ] LLM gateway URL correct (Bifrost)
- [ ] CoinGecko API tier sufficient (rate limits)
- [ ] NATS cluster configured
- [ ] Prometheus scrape targets complete

---

## 4. Infrastructure Readiness

### Kubernetes Cluster (if using)

- [ ] Cluster provisioned (EKS, GKE, AKS, or self-hosted)
- [ ] Node autoscaling configured
- [ ] Pod autoscaling configured (HPA)
- [ ] Resource requests and limits set
- [ ] Persistent volumes configured
- [ ] Backup volumes to S3/GCS/Azure
- [ ] Network policies configured
- [ ] RBAC properly configured

**Verify Cluster:**
```bash
kubectl cluster-info
kubectl get nodes
kubectl get pvc -n cryptofunk
kubectl describe hpa -n cryptofunk
```

### Docker/Container Registry

- [ ] Container images built and pushed
- [ ] Image tags versioned (not `latest`)
- [ ] Registry access configured
- [ ] Image scanning enabled (Trivy, Snyk)
- [ ] No high/critical vulnerabilities

**Build and Push Images:**
```bash
# Tag with version
export VERSION=1.0.0

# Build all images
docker build -t cryptofunk/orchestrator:$VERSION -f deployments/docker/Dockerfile.orchestrator .
docker build -t cryptofunk/api:$VERSION -f deployments/docker/Dockerfile.api .
# ... build all services

# Push to registry
docker push cryptofunk/orchestrator:$VERSION
docker push cryptofunk/api:$VERSION
```

### Database Infrastructure

- [ ] PostgreSQL 15+ installed
- [ ] TimescaleDB extension enabled
- [ ] pgvector extension enabled
- [ ] Automated backups configured (see T288)
- [ ] Backup restoration tested
- [ ] Replication configured (optional but recommended)
- [ ] Failover tested

### Message Queue

- [ ] NATS server running
- [ ] JetStream enabled
- [ ] Persistence enabled
- [ ] Cluster mode configured (3+ nodes recommended)

### Object Storage (for backups)

- [ ] S3/GCS/Azure Storage bucket created
- [ ] Credentials configured
- [ ] Lifecycle policies set (retention)
- [ ] Versioning enabled

---

## 5. Monitoring & Alerting

### Metrics Collection

- [ ] Prometheus running and scraping all targets
  - [ ] Orchestrator metrics (port 8081)
  - [ ] API metrics (port 8080)
  - [ ] Agent metrics (ports 9101-9106)
  - [ ] PostgreSQL metrics (port 9187)
  - [ ] Redis metrics (port 9121)
- [ ] Metrics retention configured (15 days recommended)
- [ ] Prometheus storage not filling up

**Verify Scraping:**
```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'

# Check metrics endpoint
curl http://localhost:8081/metrics | grep cryptofunk_
```

### Dashboards

- [ ] Grafana dashboards imported
  - [ ] System overview dashboard
  - [ ] Trading performance dashboard
  - [ ] Agent performance dashboard
  - [ ] Risk metrics dashboard
- [ ] Dashboards accessible and loading data
- [ ] Dashboard variables configured
- [ ] Refresh intervals set (10-30s)

**Access Dashboards:**
```bash
open http://localhost:3000/dashboards
# Login: admin/[your-password]
```

### Alerting

- [ ] AlertManager configured
- [ ] Alert rules defined
  - [ ] High CPU usage (>90% for 5min)
  - [ ] High memory usage (>90%)
  - [ ] Disk space low (<20%)
  - [ ] Database connection failures
  - [ ] Circuit breaker triggered
  - [ ] Max drawdown exceeded
  - [ ] Agent disconnected
  - [ ] Trade execution failures
- [ ] Notification channels configured
  - [ ] Email
  - [ ] Slack/Discord
  - [ ] PagerDuty (for critical)
- [ ] Alert test successful
- [ ] On-call rotation defined

**Test Alerts:**
```bash
# Trigger test alert
curl -X POST http://localhost:9093/api/v1/alerts -d '[{
  "labels": {"alertname": "TestAlert", "severity": "critical"},
  "annotations": {"summary": "This is a test alert"}
}]'
```

### Logging

- [ ] Centralized logging configured (ELK, Loki, CloudWatch)
- [ ] Log retention policy set (30 days minimum)
- [ ] Log levels appropriate (info in production)
- [ ] Structured logging (JSON format)
- [ ] Correlation IDs implemented (T279 if available)
- [ ] PII/secrets not logged

### Health Checks

- [ ] All services have `/health` endpoints
- [ ] Kubernetes liveness probes configured
- [ ] Kubernetes readiness probes configured
- [ ] Health check endpoints tested
- [ ] Status page created (optional)

---

## 6. Documentation Review

### User Documentation

- [ ] README.md complete and accurate
- [ ] Quick start guide works
- [ ] Configuration examples provided
- [ ] Troubleshooting guide available (docs/TROUBLESHOOTING.md)
- [ ] FAQ section complete

### Technical Documentation

- [ ] CLAUDE.md up to date
- [ ] Architecture documented (docs/MCP_INTEGRATION.md)
- [ ] API documentation complete
- [ ] Database schema documented
- [ ] Deployment guide complete

### Operational Documentation

- [ ] Runbooks created for common issues
- [ ] Deployment procedures documented
- [ ] Rollback procedures documented (see Section 9)
- [ ] Disaster recovery procedures documented (T287)
- [ ] Maintenance procedures documented
- [ ] Escalation procedures defined

### Code Documentation

- [ ] All public functions have docstrings
- [ ] Complex algorithms explained
- [ ] Configuration options documented
- [ ] Environment variables documented (.env.example)

---

## 7. Deployment Steps

### Pre-Deployment

- [ ] Code freeze initiated
- [ ] All tests passing
  - [ ] Unit tests: `go test ./...`
  - [ ] Integration tests: `go test -tags=integration ./...`
  - [ ] E2E tests: `./scripts/test-e2e.sh`
- [ ] Test coverage >80%
- [ ] No critical/high security vulnerabilities
- [ ] Changelog updated (CHANGELOG.md)
- [ ] Version tag created (`git tag v1.0.0`)

### Database Migration

- [ ] Backup current production database
  ```bash
  pg_dump -h $DB_HOST -U postgres cryptofunk > backup_pre_deploy_$(date +%Y%m%d_%H%M%S).sql
  ```
- [ ] Review migration SQL files
- [ ] Test migrations on staging
- [ ] Run migrations on production
  ```bash
  ./bin/migrate up
  ```
- [ ] Verify migration success
  ```bash
  ./bin/migrate version
  psql -h $DB_HOST -U postgres -d cryptofunk -c "SELECT version FROM schema_version ORDER BY version DESC LIMIT 5;"
  ```

### Kubernetes Deployment

- [ ] Apply ConfigMaps
  ```bash
  kubectl apply -f deployments/k8s/configmap.yaml
  ```
- [ ] Apply Secrets
  ```bash
  kubectl apply -f deployments/k8s/secrets.yaml
  ```
- [ ] Apply database/infrastructure
  ```bash
  kubectl apply -f deployments/k8s/postgres.yaml
  kubectl apply -f deployments/k8s/redis.yaml
  kubectl apply -f deployments/k8s/nats.yaml
  ```
- [ ] Wait for infrastructure to be ready
  ```bash
  kubectl wait --for=condition=ready pod -l app=postgres -n cryptofunk --timeout=300s
  kubectl wait --for=condition=ready pod -l app=redis -n cryptofunk --timeout=300s
  ```
- [ ] Run database migration job
  ```bash
  kubectl apply -f deployments/k8s/migrate-job.yaml
  kubectl wait --for=condition=complete job/migrate -n cryptofunk --timeout=300s
  ```
- [ ] Deploy application services
  ```bash
  kubectl apply -f deployments/k8s/orchestrator.yaml
  kubectl apply -f deployments/k8s/mcp-servers.yaml
  kubectl apply -f deployments/k8s/agents.yaml
  kubectl apply -f deployments/k8s/api.yaml
  ```
- [ ] Verify pods are running
  ```bash
  kubectl get pods -n cryptofunk
  kubectl logs -f deployment/orchestrator -n cryptofunk
  ```

### Docker Compose Deployment (Simpler Alternative)

- [ ] Pull latest images
  ```bash
  docker-compose -f docker-compose.prod.yml pull
  ```
- [ ] Stop current services gracefully
  ```bash
  docker-compose -f docker-compose.prod.yml down
  ```
- [ ] Start services
  ```bash
  docker-compose -f docker-compose.prod.yml up -d
  ```
- [ ] Verify services
  ```bash
  docker-compose -f docker-compose.prod.yml ps
  docker-compose -f docker-compose.prod.yml logs -f orchestrator
  ```

### Configuration Deployment

- [ ] Update configuration files (if changed)
- [ ] Restart affected services
- [ ] Verify configuration loaded
  ```bash
  # Check orchestrator logs for config validation
  kubectl logs deployment/orchestrator -n cryptofunk | grep -i "configuration"
  ```

---

## 8. Post-Deployment Verification

### Health Checks

- [ ] All pods/containers running
  ```bash
  kubectl get pods -n cryptofunk
  # or
  docker-compose ps
  ```
- [ ] Health endpoints responding
  ```bash
  curl http://your-domain.com/health
  curl http://your-domain.com/api/health
  ```
- [ ] No crash loops
  ```bash
  kubectl get pods -n cryptofunk --watch
  ```

### Smoke Tests

- [ ] Can access API
  ```bash
  curl https://api.cryptofunk.io/health
  ```
- [ ] Can connect to database
  ```bash
  psql -h $DB_HOST -U postgres -d cryptofunk -c "SELECT COUNT(*) FROM trading_sessions;"
  ```
- [ ] Can read from Redis cache
  ```bash
  redis-cli PING
  redis-cli GET test_key
  ```
- [ ] NATS messaging working
  ```bash
  nats pub test.subject "test message"
  nats sub test.subject
  ```
- [ ] Prometheus scraping metrics
  ```bash
  curl http://prometheus.cryptofunk.io/api/v1/targets
  ```
- [ ] Grafana dashboards loading
  ```bash
  open http://grafana.cryptofunk.io
  ```

### Functional Tests

- [ ] Authentication working
- [ ] API endpoints responding correctly
- [ ] Agents connecting to orchestrator
- [ ] Agents generating signals
- [ ] Risk management working
- [ ] Circuit breakers functional
- [ ] Paper trading works (test first!)
- [ ] Live trading works (small test trade)

**Test Trade (Paper Mode):**
```bash
# Manually insert a test signal to trigger a trade
psql -h $DB_HOST -U postgres -d cryptofunk << SQL
INSERT INTO agent_signals (session_id, agent_type, signal_type, symbol, confidence, reasoning)
VALUES (
  (SELECT id FROM trading_sessions WHERE status = 'ACTIVE' LIMIT 1),
  'technical',
  'BUY',
  'BTCUSDT',
  0.85,
  'Test signal - post-deployment verification'
);
SQL

# Watch for order creation
watch -n 1 "psql -h $DB_HOST -U postgres -d cryptofunk -c \"SELECT * FROM orders ORDER BY created_at DESC LIMIT 5;\""
```

### Performance Verification

- [ ] API response times acceptable (<200ms p95)
- [ ] Database queries fast (<50ms p95)
- [ ] No memory leaks (stable memory usage)
- [ ] No CPU spikes
- [ ] Cache hit rate >80%

**Check Performance:**
```bash
# API response time
ab -n 100 -c 10 https://api.cryptofunk.io/health

# Database query times
psql -h $DB_HOST -U postgres -d cryptofunk << SQL
SELECT query, calls, mean_time, max_time
FROM pg_stat_statements
WHERE query NOT LIKE '%pg_stat_statements%'
ORDER BY mean_time DESC
LIMIT 10;
SQL

# Memory usage
kubectl top pods -n cryptofunk
```

### Monitoring Verification

- [ ] Metrics flowing to Prometheus
- [ ] Dashboards showing live data
- [ ] Alerts configured and working
- [ ] Logs flowing to centralized logging
- [ ] No alert spam (false positives)

### Security Verification

- [ ] SSL certificate valid
  ```bash
  openssl s_client -connect api.cryptofunk.io:443 -servername api.cryptofunk.io
  ```
- [ ] API key authentication working
- [ ] Rate limiting working
- [ ] CORS configured correctly
- [ ] No exposed credentials in logs

---

## 9. Rollback Procedures

### When to Rollback

Rollback immediately if:
- [ ] Critical functionality broken
- [ ] Data loss or corruption detected
- [ ] Security vulnerability introduced
- [ ] Performance degradation >50%
- [ ] High error rate (>5% of requests)

### Kubernetes Rollback

```bash
# Check deployment history
kubectl rollout history deployment/orchestrator -n cryptofunk

# Rollback to previous version
kubectl rollout undo deployment/orchestrator -n cryptofunk

# Rollback to specific revision
kubectl rollout undo deployment/orchestrator -n cryptofunk --to-revision=2

# Verify rollback
kubectl rollout status deployment/orchestrator -n cryptofunk
kubectl get pods -n cryptofunk
```

### Docker Compose Rollback

```bash
# Use previous image version
export VERSION=0.9.0  # Previous working version

# Update docker-compose.prod.yml with old version
sed -i "s/:1.0.0/:$VERSION/g" docker-compose.prod.yml

# Restart services
docker-compose -f docker-compose.prod.yml down
docker-compose -f docker-compose.prod.yml up -d
```

### Database Rollback

```bash
# Restore from backup
psql -h $DB_HOST -U postgres -d postgres -c "DROP DATABASE cryptofunk;"
psql -h $DB_HOST -U postgres -d postgres -c "CREATE DATABASE cryptofunk;"
psql -h $DB_HOST -U postgres -d cryptofunk < backup_pre_deploy_YYYYMMDD_HHMMSS.sql

# Or use migration rollback (if supported)
./bin/migrate down 1
```

### Communication

- [ ] Notify team of rollback
- [ ] Update status page (if exists)
- [ ] Document rollback reason
- [ ] Create incident report
- [ ] Schedule post-mortem

---

## 10. Sign-off

### Deployment Team Sign-off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Tech Lead | | | |
| DevOps Engineer | | | |
| Security Engineer | | | |
| QA Lead | | | |

### Final Verification

- [ ] All checklist items completed
- [ ] No known critical issues
- [ ] Monitoring confirms healthy state
- [ ] Team has access to all necessary tools
- [ ] On-call rotation active
- [ ] Incident response plan reviewed

### Post-Deployment Actions

- [ ] Monitor system for 24 hours
- [ ] Review logs for errors/warnings
- [ ] Check trading performance
- [ ] Verify no financial discrepancies
- [ ] Update documentation with lessons learned
- [ ] Schedule retrospective meeting

### Notes

_Add any deployment-specific notes, issues encountered, or deviations from standard procedure:_

```
[Your notes here]
```

---

## Appendix: Emergency Contacts

| Role | Name | Phone | Email | Slack/Discord |
|------|------|-------|-------|---------------|
| Tech Lead | | | | |
| DevOps | | | | |
| Security | | | | |
| Database Admin | | | | |
| Exchange Support | | | | |
| Cloud Provider Support | | | | |

---

## Appendix: Quick Commands Reference

```bash
# Configuration
./bin/orchestrator --verify-keys

# Database
psql -h $DB_HOST -U postgres -d cryptofunk
./scripts/dev/reset-db.sh  # DANGER: Development only!

# Kubernetes
kubectl get pods -n cryptofunk
kubectl logs -f deployment/orchestrator -n cryptofunk
kubectl exec -it deployment/orchestrator -n cryptofunk -- /bin/sh
kubectl rollout restart deployment/orchestrator -n cryptofunk

# Docker Compose
docker-compose -f docker-compose.prod.yml ps
docker-compose -f docker-compose.prod.yml logs -f orchestrator
docker-compose -f docker-compose.prod.yml restart orchestrator

# Monitoring
curl http://prometheus.cryptofunk.io/api/v1/query?query=up
curl http://grafana.cryptofunk.io/api/health

# Health Checks
curl https://api.cryptofunk.io/health
curl https://api.cryptofunk.io/metrics
```

---

**END OF CHECKLIST**

Print this document, check off items as you complete them, and keep it for your records. Good luck with your deployment!
