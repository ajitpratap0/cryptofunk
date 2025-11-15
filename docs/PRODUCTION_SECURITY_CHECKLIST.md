# Production Security Checklist

This checklist ensures all security requirements are met before deploying CryptoFunk to production.

**Last Updated**: 2025-01-15
**Status**: ✅ All P0 Security Issues Resolved

## Overview

Phase 13 security audit (T303) identified several P0 (production-blocking) security issues. This document tracks their resolution and provides a pre-deployment checklist.

## P0 Security Findings - Resolution Status

### ✅ 1. Default Credentials (CRITICAL)
**Status**: RESOLVED
**Risk Level**: P0 - Critical
**Impact**: Unauthorized access to database, Grafana, API

**Issues**:
- ❌ `docker-compose.yml` had default password `cryptofunk_dev_password`
- ❌ Grafana had default password `cryptofunk_grafana`
- ❌ JWT secret had default value `changeme_in_production`

**Resolution**:
- ✅ Removed all default credentials from `docker-compose.yml`
- ✅ Changed to required environment variables with `${VAR:?error message}` syntax
- ✅ Updated `.env.example` with secure placeholder values
- ✅ Added password generation instructions (`openssl rand -base64 32`)
- ✅ Docker Compose now fails fast if credentials not provided

**Verification**:
```bash
# Should fail with clear error message
docker-compose up postgres
# Error: POSTGRES_PASSWORD environment variable is required

# Set environment variable
export POSTGRES_PASSWORD=$(openssl rand -base64 32)
docker-compose up -d postgres  # Should succeed
```

**Files Modified**:
- `docker-compose.yml`
- `.env.example`

---

### ✅ 2. TLS Not Enforced (CRITICAL)
**Status**: RESOLVED
**Risk Level**: P0 - Critical
**Impact**: Database and Redis traffic sent in plaintext, credentials can be intercepted

**Issues**:
- ❌ PostgreSQL connections used `sslmode=disable`
- ❌ Redis connections used unencrypted `redis://` protocol
- ❌ No TLS certificates provided
- ❌ No production deployment configuration

**Resolution**:
- ✅ Created `docker-compose.prod.yml` for production deployments
- ✅ Enforces `sslmode=require` for PostgreSQL
- ✅ Enforces `rediss://` (TLS) for Redis
- ✅ Created certificate generation script (`scripts/generate-certs.sh`)
- ✅ Created comprehensive TLS setup guide (`docs/TLS_SETUP.md`)
- ✅ Added TLS verification to production validator
- ✅ Supports self-signed (dev) and CA-signed (prod) certificates

**Production Deployment**:
```bash
# 1. Generate certificates
./scripts/generate-certs.sh

# 2. Set environment variables
export POSTGRES_PASSWORD=$(openssl rand -base64 32)
export GRAFANA_ADMIN_PASSWORD=$(openssl rand -base64 32)
export JWT_SECRET=$(openssl rand -base64 32)
export REDIS_PASSWORD=$(openssl rand -base64 32)
export DATABASE_URL="postgresql://postgres:${POSTGRES_PASSWORD}@postgres:5432/cryptofunk?sslmode=require&sslrootcert=/certs/ca.crt"
export REDIS_URL="rediss://:${REDIS_PASSWORD}@redis:6380?tls_ca_cert=/certs/ca.crt"

# 3. Start with production overrides
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

**Verification**:
```bash
# Check PostgreSQL SSL
psql "$DATABASE_URL" -c "\conninfo"
# Should show: SSL connection (protocol: TLSv1.3, cipher: ...)

# Check Redis TLS
redis-cli --tls --cacert certs/redis/ca.crt -h localhost -p 6380 ping
# Should return: PONG
```

**Files Created**:
- `docker-compose.prod.yml`
- `scripts/generate-certs.sh`
- `docs/TLS_SETUP.md`

**Files Modified**:
- `internal/config/validator.go` (added TLS validation)

---

### ✅ 3. Vault Not Enforced (CRITICAL)
**Status**: RESOLVED
**Risk Level**: P0 - Critical
**Impact**: Secrets stored in environment variables, not rotated, logged to container metadata

**Issues**:
- ❌ No enforcement of Vault in production
- ❌ Secrets passed via environment variables
- ❌ No validation of Vault configuration

**Resolution**:
- ✅ Added `validateProductionRequirements()` to config validator
- ✅ Enforces `VAULT_ENABLED=true` when `CRYPTOFUNK_APP_ENVIRONMENT=production`
- ✅ Validates Vault address and auth method configuration
- ✅ Validates auth-specific requirements:
  - Kubernetes: Service account token must exist
  - Token: VAULT_TOKEN must be set
  - AppRole: VAULT_ROLE_ID and VAULT_SECRET_ID must be set
- ✅ Application fails to start if Vault not properly configured
- ✅ Clear error messages with documentation references

**Production Deployment**:
```bash
# Set production environment
export CRYPTOFUNK_APP_ENVIRONMENT=production

# Configure Vault
export VAULT_ENABLED=true
export VAULT_ADDR=https://vault.example.com:8200
export VAULT_AUTH_METHOD=kubernetes  # or token, or approle
export VAULT_MOUNT_PATH=secret
export VAULT_SECRET_PATH=cryptofunk/production

# Start orchestrator (will fail if Vault not configured)
./bin/orchestrator
# Validates Vault configuration before starting
```

**Verification**:
```bash
# Should fail without Vault in production
export CRYPTOFUNK_APP_ENVIRONMENT=production
export VAULT_ENABLED=false
./bin/orchestrator
# Error: Vault must be enabled in production (set VAULT_ENABLED=true)

# Should succeed with Vault configured
export VAULT_ENABLED=true
export VAULT_ADDR=https://vault.example.com:8200
export VAULT_AUTH_METHOD=kubernetes
./bin/orchestrator
# ✓ Production security requirements validated successfully
```

**Files Modified**:
- `internal/config/validator.go` (added 120+ lines of production validation)

---

### ✅ 4. Containers Run as Root (HIGH)
**Status**: RESOLVED
**Risk Level**: P0 - High
**Impact**: Container escape allows root access to host

**Issues**:
- ❓ Need to verify non-root user in all Dockerfiles

**Resolution**:
- ✅ Verified all Dockerfiles already have non-root user (UID/GID 1000)
- ✅ All containers run as `appuser` (not root)
- ✅ Proper file ownership and permissions set
- ✅ Security context configured in Kubernetes manifests

**Dockerfiles with Non-Root User**:
- ✅ `Dockerfile.orchestrator` - USER appuser
- ✅ `Dockerfile.api` - USER appuser
- ✅ `Dockerfile.agent` - USER appuser
- ✅ `Dockerfile.mcp-server` - USER appuser
- ✅ `Dockerfile.migrate` - USER appuser
- ✅ `Dockerfile.backtest` - USER appuser

**Verification**:
```bash
# Check user in running container
docker-compose exec orchestrator whoami
# Should return: appuser

docker-compose exec orchestrator id
# Should return: uid=1000(appuser) gid=1000(appuser) groups=1000(appuser)
```

**No Changes Required**: All Dockerfiles already compliant.

---

### ✅ 5. No Network Policies (HIGH)
**Status**: RESOLVED
**Risk Level**: P0 - High
**Impact**: No network isolation, lateral movement possible if one container compromised

**Issues**:
- ❌ No Kubernetes NetworkPolicy resources
- ❌ All pods can communicate with all other pods
- ❌ Database and Redis accessible from any pod

**Resolution**:
- ✅ Created comprehensive `deployments/k8s/base/network-policy.yaml`
- ✅ Implemented zero-trust network segmentation
- ✅ Default deny-all policy for ingress and egress
- ✅ Explicit allow rules for each component
- ✅ Infrastructure (PostgreSQL, Redis) only accessible by app components
- ✅ External HTTPS only allowed where needed (MCP servers, Bifrost, AlertManager)
- ✅ DNS resolution allowed for all components
- ✅ Prometheus can scrape metrics from all components

**Network Policies Created**:
- ✅ `default-deny-all` - Deny all traffic by default
- ✅ `postgres-allow` - PostgreSQL access control
- ✅ `redis-allow` - Redis access control
- ✅ `nats-allow` - NATS access control
- ✅ `orchestrator-allow` - Orchestrator communication
- ✅ `mcp-servers-allow` - MCP server communication
- ✅ `agents-allow` - Agent communication
- ✅ `api-allow` - API server (external facing)
- ✅ `bifrost-allow` - LLM gateway
- ✅ `prometheus-allow` - Metrics scraping
- ✅ `alertmanager-allow` - Alert notifications
- ✅ `grafana-allow` - Dashboard access

**Verification**:
```bash
# Apply network policies
kubectl apply -f deployments/k8s/base/network-policy.yaml

# Test that PostgreSQL is NOT accessible from unauthorized pods
kubectl run test-pod --rm -it --image=postgres:15 -- psql -h postgres -U postgres
# Should fail: connection refused or timeout

# Test that orchestrator CAN access PostgreSQL
kubectl exec -it deployment/orchestrator -- psql "$DATABASE_URL" -c "SELECT 1"
# Should succeed: return 1
```

**Files Created**:
- `deployments/k8s/base/network-policy.yaml`

**Files Modified**:
- `deployments/k8s/base/kustomization.yaml` (added network-policy.yaml resource)

---

## Production Deployment Checklist

### Pre-Deployment

- [ ] **Generate Strong Credentials**
  ```bash
  export POSTGRES_PASSWORD=$(openssl rand -base64 32)
  export GRAFANA_ADMIN_PASSWORD=$(openssl rand -base64 32)
  export JWT_SECRET=$(openssl rand -base64 32)
  export REDIS_PASSWORD=$(openssl rand -base64 32)
  ```

- [ ] **Generate TLS Certificates**
  ```bash
  # For development/staging (self-signed)
  ./scripts/generate-certs.sh

  # For production (use Let's Encrypt or commercial CA)
  # See docs/TLS_SETUP.md
  ```

- [ ] **Configure Vault**
  ```bash
  export VAULT_ENABLED=true
  export VAULT_ADDR=https://vault.example.com:8200
  export VAULT_AUTH_METHOD=kubernetes
  ```

- [ ] **Set Production Environment**
  ```bash
  export CRYPTOFUNK_APP_ENVIRONMENT=production
  export TRADING_MODE=PAPER  # Start with paper trading!
  ```

- [ ] **Update Connection Strings with TLS**
  ```bash
  export DATABASE_URL="postgresql://postgres:${POSTGRES_PASSWORD}@postgres:5432/cryptofunk?sslmode=require&sslrootcert=/certs/ca.crt"
  export REDIS_URL="rediss://:${REDIS_PASSWORD}@redis:6380?tls_ca_cert=/certs/ca.crt"
  ```

### Deployment (Docker Compose)

- [ ] **Verify Environment Variables**
  ```bash
  # Should fail if any required variables missing
  docker-compose config
  ```

- [ ] **Deploy with Production Overrides**
  ```bash
  docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
  ```

- [ ] **Verify TLS Connections**
  ```bash
  # PostgreSQL
  docker-compose exec orchestrator psql "$DATABASE_URL" -c "\conninfo"
  # Should show: SSL connection

  # Redis
  docker-compose exec orchestrator redis-cli --tls -h redis -p 6380 ping
  # Should return: PONG
  ```

### Deployment (Kubernetes)

- [ ] **Create Namespace**
  ```bash
  kubectl create namespace cryptofunk
  ```

- [ ] **Create TLS Secrets**
  ```bash
  kubectl create secret generic postgres-tls \
    --from-file=server.crt=certs/postgres/server.crt \
    --from-file=server.key=certs/postgres/server.key \
    --from-file=ca.crt=certs/postgres/ca.crt \
    -n cryptofunk

  kubectl create secret generic redis-tls \
    --from-file=redis.crt=certs/redis/redis.crt \
    --from-file=redis.key=certs/redis/redis.key \
    --from-file=ca.crt=certs/redis/ca.crt \
    -n cryptofunk
  ```

- [ ] **Create Application Secrets**
  ```bash
  kubectl create secret generic cryptofunk-secrets \
    --from-literal=postgres-password="$POSTGRES_PASSWORD" \
    --from-literal=grafana-admin-password="$GRAFANA_ADMIN_PASSWORD" \
    --from-literal=jwt-secret="$JWT_SECRET" \
    --from-literal=redis-password="$REDIS_PASSWORD" \
    --from-literal=anthropic-api-key="$ANTHROPIC_API_KEY" \
    --from-literal=openai-api-key="$OPENAI_API_KEY" \
    -n cryptofunk
  ```

- [ ] **Apply Manifests**
  ```bash
  kubectl apply -k deployments/k8s/base/
  ```

- [ ] **Verify Pods are Running**
  ```bash
  kubectl get pods -n cryptofunk
  # All pods should be Running
  ```

- [ ] **Verify Network Policies Applied**
  ```bash
  kubectl get networkpolicies -n cryptofunk
  # Should show 13 network policies
  ```

- [ ] **Test Network Isolation**
  ```bash
  # Should fail (PostgreSQL not accessible from unauthorized pods)
  kubectl run test-pod --rm -it --image=postgres:15 -n cryptofunk -- \
    psql -h postgres -U postgres
  ```

### Post-Deployment Verification

- [ ] **Check Health Endpoints**
  ```bash
  curl http://localhost:8081/health  # Orchestrator
  curl http://localhost:8080/health  # API
  ```

- [ ] **Verify Vault Integration**
  ```bash
  # Check orchestrator logs for Vault authentication
  kubectl logs -f deployment/orchestrator -n cryptofunk | grep -i vault
  # Should show: "Successfully authenticated to Vault using kubernetes method"
  ```

- [ ] **Verify TLS Enforcement**
  ```bash
  # Orchestrator should refuse to start without TLS in production
  export CRYPTOFUNK_APP_ENVIRONMENT=production
  export DATABASE_URL="postgresql://...?sslmode=disable"
  ./bin/orchestrator
  # Should error: Database SSL cannot be disabled in production
  ```

- [ ] **Run Security Scan**
  ```bash
  ./scripts/security-scan.sh
  # Should show: PASS for all checks
  ```

- [ ] **Check Prometheus Metrics**
  ```bash
  curl http://localhost:9090/api/v1/targets
  # All targets should be "up"
  ```

- [ ] **Test AlertManager**
  ```bash
  # Send test alert
  curl -X POST http://localhost:9093/api/v1/alerts \
    -H 'Content-Type: application/json' \
    -d '[{"labels":{"alertname":"TestAlert","severity":"warning"}}]'

  # Check Slack/email for alert notification
  ```

### Production Monitoring

- [ ] **Set Up Log Aggregation**
  - Configure centralized logging (ELK, Loki, CloudWatch)
  - Ensure secrets are NOT logged

- [ ] **Configure Backup**
  - Database backups (hourly incremental, daily full)
  - Vault backup (if self-hosted)

- [ ] **Set Up Uptime Monitoring**
  - External health check monitoring
  - PagerDuty/OpsGenie integration

- [ ] **Review Alert Runbook**
  - Ensure team has access to `docs/ALERT_RUNBOOK.md`
  - Test escalation procedures

## Security Maintenance

### Weekly
- [ ] Review access logs for anomalies
- [ ] Check for failed authentication attempts
- [ ] Verify all services healthy

### Monthly
- [ ] Rotate database credentials (see `docs/SECRET_ROTATION.md`)
- [ ] Review and update firewall rules
- [ ] Check for dependency vulnerabilities (`go mod tidy && govulncheck ./...`)
- [ ] Review Grafana user access

### Quarterly
- [ ] Rotate API keys (exchange, LLM)
- [ ] Re-run penetration testing
- [ ] Review and update network policies
- [ ] Security audit (OWASP Top 10)
- [ ] Disaster recovery drill

### Annually
- [ ] Rotate TLS certificates (if not automated)
- [ ] Third-party security audit
- [ ] Review and update security policies
- [ ] Compliance assessment (SOC 2, if applicable)

## Incident Response

### If Credentials Compromised
1. Immediately rotate all credentials (see `docs/SECRET_ROTATION.md`)
2. Review access logs for unauthorized access
3. Check for data exfiltration
4. Notify stakeholders if sensitive data accessed

### If Container Compromised
1. Isolate affected pod (network policy + delete pod)
2. Review logs for attack vector
3. Check for lateral movement
4. Rebuild from known-good image
5. Apply security patches

### If Database Compromised
1. Immediately cut database network access (except backup)
2. Create point-in-time backup
3. Review transaction logs
4. Restore from known-good backup if needed
5. Rotate all database credentials

## References

- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - Full security audit report
- [TLS_SETUP.md](TLS_SETUP.md) - TLS/SSL configuration guide
- [SECRET_ROTATION.md](SECRET_ROTATION.md) - Secret rotation procedures
- [ALERT_RUNBOOK.md](ALERT_RUNBOOK.md) - Alert response procedures
- [PHASE_13_SUMMARY.md](PHASE_13_SUMMARY.md) - Phase 13 completion summary

---

**Security Sign-Off**: All P0 security issues resolved. System approved for staging deployment and beta testing.

**Next Steps**: Deploy to staging environment (T301), run 24-hour soak test, then proceed with beta user recruitment (T304).
