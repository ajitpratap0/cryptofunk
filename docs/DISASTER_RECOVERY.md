# Disaster Recovery Procedures

**Document Version**: 1.0  
**Last Updated**: 2025-11-06  
**Review Frequency**: Quarterly  
**Next Review Date**: 2025-02-06  

This document defines the disaster recovery (DR) procedures for CryptoFunk trading system. It covers backup strategies, restoration procedures, recovery time objectives, and incident response.

---

## Table of Contents

1. [Recovery Objectives](#1-recovery-objectives)
2. [Backup Procedures](#2-backup-procedures)
3. [Restore Procedures](#3-restore-procedures)
4. [Disaster Scenarios](#4-disaster-scenarios)
5. [Incident Response Playbook](#5-incident-response-playbook)
6. [Testing & Validation](#6-testing--validation)
7. [Contact Information](#7-contact-information)

---

## 1. Recovery Objectives

### Recovery Point Objective (RPO)

**RPO = 5 minutes**

Maximum acceptable data loss in the event of a disaster.

- **Database**: Continuous replication + WAL archiving (5-minute granularity)
- **Trading data**: All trades persisted to database before acknowledgment
- **Configuration**: Version controlled in git (no data loss)
- **Logs**: May lose up to 5 minutes of recent logs

### Recovery Time Objective (RTO)

**RTO = 30 minutes**

Maximum acceptable downtime in the event of a disaster.

| Component | RTO | Notes |
|-----------|-----|-------|
| Database | 10 minutes | Restore from backup or failover to replica |
| Application Services | 15 minutes | Redeploy from Docker images |
| Trading Agents | 5 minutes | Restart from existing images |
| Full System | 30 minutes | Complete system restoration |

### Data Retention

| Data Type | Retention Period | Backup Frequency |
|-----------|------------------|------------------|
| Database (critical) | 30 days | Hourly |
| Database (archives) | 1 year | Daily |
| Application logs | 90 days | Continuous |
| Audit logs | 7 years | Continuous |
| Configuration history | Indefinite | Git commits |
| Trading records | 7 years | Daily |

---

## 2. Backup Procedures

### 2.1 Database Backups

#### Automated Hourly Backups

**Location**: S3/GCS/Azure Storage  
**Frequency**: Every hour  
**Retention**: 48 hours (hourly), 30 days (daily), 1 year (weekly)

**Implementation** (T288):
```bash
#!/bin/bash
# scripts/backup/database-backup.sh

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="cryptofunk_backup_${TIMESTAMP}.sql.gz"
S3_BUCKET="s3://cryptofunk-backups/database"

# Create backup
pg_dump -h $DB_HOST -U postgres cryptofunk | gzip > /tmp/$BACKUP_FILE

# Upload to S3
aws s3 cp /tmp/$BACKUP_FILE $S3_BUCKET/$BACKUP_FILE

# Verify upload
aws s3 ls $S3_BUCKET/$BACKUP_FILE

# Clean up local file
rm /tmp/$BACKUP_FILE

# Clean old backups (keep last 48 hourly, 30 daily, 52 weekly)
./scripts/backup/cleanup-old-backups.sh
```

**Cron Schedule**:
```cron
# Hourly backup
0 * * * * /app/scripts/backup/database-backup.sh >> /var/log/backup.log 2>&1

# Daily full backup (midnight)
0 0 * * * /app/scripts/backup/database-backup-full.sh >> /var/log/backup.log 2>&1

# Weekly archive (Sunday 2AM)
0 2 * * 0 /app/scripts/backup/database-backup-archive.sh >> /var/log/backup.log 2>&1
```

#### PostgreSQL WAL Archiving

For point-in-time recovery (PITR):

```bash
# Enable WAL archiving in postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'aws s3 cp %p s3://cryptofunk-backups/wal/%f'
archive_timeout = 300  # 5 minutes
```

#### Database Replication (Optional but Recommended)

**Setup streaming replication for high availability:**

```sql
-- On primary server
CREATE ROLE replicator WITH REPLICATION LOGIN ENCRYPTED PASSWORD 'strong_password';

-- Configure pg_hba.conf
host replication replicator replica_ip/32 md5
```

```bash
# On replica server
pg_basebackup -h primary_ip -D /var/lib/postgresql/data -U replicator -P --wal-method=stream

# Create recovery.conf (PostgreSQL 12+: recovery.signal + postgresql.conf)
standby_mode = 'on'
primary_conninfo = 'host=primary_ip port=5432 user=replicator password=xxx'
trigger_file = '/tmp/postgresql.trigger'
```

### 2.2 Configuration Backups

**Location**: Git repository (GitHub/GitLab)  
**Frequency**: On every change (git commits)  
**Retention**: Indefinite (version history)

```bash
# All configuration is version controlled
configs/
  config.yaml
  agents.yaml
  orchestrator.yaml
  examples/

deployments/
  k8s/
  docker/

# Backup current production config
git tag production-config-$(date +%Y%m%d) -m "Production config snapshot"
git push origin --tags
```

### 2.3 Secrets Backup

**Location**: HashiCorp Vault / AWS Secrets Manager  
**Frequency**: On change  
**Retention**: Indefinite (versioned)

```bash
# Export secrets (encrypted)
vault kv get -format=json secret/cryptofunk > secrets_backup_$(date +%Y%m%d).json.enc
gpg --encrypt --recipient backup@cryptofunk.io secrets_backup_$(date +%Y%m%d).json.enc

# Store in secure location
aws s3 cp secrets_backup_$(date +%Y%m%d).json.enc.gpg s3://cryptofunk-secrets-backup/
```

### 2.4 Docker Image Backups

**Location**: Container registry (Docker Hub / AWS ECR)  
**Frequency**: On every build  
**Retention**: Last 10 versions per image

```bash
# Images are automatically retained in registry
docker images | grep cryptofunk

# Tag production versions
docker tag cryptofunk/orchestrator:latest cryptofunk/orchestrator:prod-$(date +%Y%m%d)
docker push cryptofunk/orchestrator:prod-$(date +%Y%m%d)
```

### 2.5 Logs Backup

**Location**: S3/GCS or centralized logging (ELK, CloudWatch)  
**Frequency**: Continuous streaming  
**Retention**: 90 days (standard), 7 years (audit logs)

```bash
# Logs are streamed to centralized logging
# Periodic export for long-term storage
aws logs create-export-task \
  --log-group-name /cryptofunk/production \
  --from $(date -d '7 days ago' +%s)000 \
  --to $(date +%s)000 \
  --destination cryptofunk-logs-archive \
  --destination-prefix logs/$(date +%Y/%m/%d)
```

---

## 3. Restore Procedures

### 3.1 Database Restore

#### Full Database Restore

**Scenario**: Complete database loss or corruption

**Steps**:

1. **Stop all application services** (prevent writes to corrupted DB)
   ```bash
   kubectl scale deployment --all --replicas=0 -n cryptofunk
   # or
   docker-compose down orchestrator api agents
   ```

2. **Drop corrupted database** (if exists)
   ```bash
   psql -h $DB_HOST -U postgres -c "DROP DATABASE IF EXISTS cryptofunk;"
   ```

3. **Create new database**
   ```bash
   psql -h $DB_HOST -U postgres -c "CREATE DATABASE cryptofunk;"
   ```

4. **Find latest backup**
   ```bash
   aws s3 ls s3://cryptofunk-backups/database/ --recursive | sort | tail -5
   ```

5. **Download and restore backup**
   ```bash
   # Download backup
   aws s3 cp s3://cryptofunk-backups/database/cryptofunk_backup_20251106_120000.sql.gz /tmp/

   # Restore
   gunzip -c /tmp/cryptofunk_backup_20251106_120000.sql.gz | \
     psql -h $DB_HOST -U postgres -d cryptofunk

   # Verify restore
   psql -h $DB_HOST -U postgres -d cryptofunk -c "\dt"
   psql -h $DB_HOST -U postgres -d cryptofunk -c "SELECT COUNT(*) FROM trading_sessions;"
   ```

6. **Enable extensions** (if not in backup)
   ```sql
   CREATE EXTENSION IF NOT EXISTS timescaledb;
   CREATE EXTENSION IF NOT EXISTS vector;
   CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
   ```

7. **Restart application services**
   ```bash
   kubectl scale deployment --all --replicas=1 -n cryptofunk
   # or
   docker-compose up -d
   ```

8. **Verify system health**
   ```bash
   curl http://localhost:8081/health
   ./scripts/dev/watch-logs.sh --errors
   ```

**Estimated Time**: 10-15 minutes

#### Point-in-Time Recovery (PITR)

**Scenario**: Need to restore to specific time (e.g., before bad trade)

**Steps**:

1. **Identify target recovery time**
   ```
   Target: 2025-11-06 14:30:00 UTC
   ```

2. **Find base backup before target time**
   ```bash
   aws s3 ls s3://cryptofunk-backups/database/ | grep "20251106_14"
   # Use: cryptofunk_backup_20251106_140000.sql.gz (14:00)
   ```

3. **Restore base backup** (same as full restore steps 1-5)

4. **Apply WAL files up to target time**
   ```bash
   # Download WAL files
   aws s3 sync s3://cryptofunk-backups/wal/ /var/lib/postgresql/wal/

   # Create recovery.conf
   cat > /var/lib/postgresql/data/recovery.conf << RECOVERY
   restore_command = 'cp /var/lib/postgresql/wal/%f %p'
   recovery_target_time = '2025-11-06 14:30:00 UTC'
   recovery_target_action = 'promote'
   RECOVERY

   # Start PostgreSQL in recovery mode
   systemctl restart postgresql

   # Wait for recovery to complete (check logs)
   tail -f /var/log/postgresql/postgresql.log
   ```

5. **Verify recovered data**
   ```sql
   SELECT MAX(created_at) FROM trades;
   -- Should show time <= 2025-11-06 14:30:00
   ```

**Estimated Time**: 15-30 minutes

### 3.2 Configuration Restore

**Scenario**: Configuration files lost or corrupted

```bash
# Clone from git
git clone https://github.com/yourorg/cryptofunk.git /tmp/cryptofunk-restore

# Copy production configs
cp /tmp/cryptofunk-restore/configs/*.yaml /app/configs/

# Or checkout specific version
git checkout production-config-20251106

# Verify and restart services
./bin/orchestrator --verify-keys
kubectl rollout restart deployment/orchestrator -n cryptofunk
```

**Estimated Time**: 5 minutes

### 3.3 Secrets Restore

**Scenario**: Secrets lost or need to restore

```bash
# Download encrypted backup
aws s3 cp s3://cryptofunk-secrets-backup/secrets_backup_20251106.json.enc.gpg /tmp/

# Decrypt
gpg --decrypt /tmp/secrets_backup_20251106.json.enc.gpg > /tmp/secrets.json

# Restore to Vault
cat /tmp/secrets.json | jq -r 'to_entries[] | "\(.key)=\(.value)"' | while read kv; do
  key=$(echo $kv | cut -d= -f1)
  value=$(echo $kv | cut -d= -f2-)
  vault kv put secret/cryptofunk/$key value="$value"
done

# Or restore to Kubernetes secrets
kubectl create secret generic cryptofunk-secrets \
  --from-env-file=/tmp/secrets.env \
  --namespace=cryptofunk \
  --dry-run=client -o yaml | kubectl apply -f -
```

**Estimated Time**: 5 minutes

### 3.4 Full System Restore

**Scenario**: Complete infrastructure loss (e.g., AWS region failure)

**Prerequisites**:
- Backup infrastructure in different region/cloud
- Docker images in container registry
- Database backups in S3
- Configuration in git
- Secrets in Vault/Secrets Manager

**Steps**:

1. **Provision new infrastructure** (15 minutes)
   ```bash
   # Kubernetes cluster
   eksctl create cluster --config-file=deployments/k8s/cluster.yaml

   # or Docker Swarm
   docker swarm init
   ```

2. **Restore database** (10 minutes)
   - Follow Database Restore procedure (Section 3.1)

3. **Deploy application** (10 minutes)
   ```bash
   # Apply all Kubernetes manifests
   kubectl apply -f deployments/k8s/namespace.yaml
   kubectl apply -f deployments/k8s/secrets.yaml
   kubectl apply -f deployments/k8s/configmap.yaml
   kubectl apply -f deployments/k8s/

   # Or Docker Compose
   docker-compose -f docker-compose.prod.yml up -d
   ```

4. **Verify system** (5 minutes)
   - Run smoke tests
   - Check health endpoints
   - Verify trading functionality

**Total Estimated Time**: 30-40 minutes

---

## 4. Disaster Scenarios

### 4.1 Database Failure

**Symptoms**:
- Unable to connect to database
- Database returning errors
- Data corruption detected

**Immediate Actions**:
1. Stop all trading immediately
   ```bash
   kubectl scale deployment orchestrator --replicas=0 -n cryptofunk
   ```
2. Assess damage (corruption vs. connection issue)
3. If replica available, promote to primary
4. If no replica, restore from backup

**Recovery**: Follow Section 3.1

### 4.2 Exchange API Failure

**Symptoms**:
- Unable to place orders
- Unable to fetch prices
- Authentication failures

**Immediate Actions**:
1. Switch to circuit breaker mode (automatic)
2. Check exchange status page
3. Verify API keys still valid
4. Contact exchange support if needed

**Recovery**:
```bash
# Test API connectivity
./bin/orchestrator --verify-keys

# If keys invalid, rotate
export BINANCE_API_KEY=new_key
export BINANCE_API_SECRET=new_secret
kubectl rollout restart deployment/orchestrator -n cryptofunk
```

### 4.3 Kubernetes Cluster Failure

**Symptoms**:
- Unable to access cluster
- Nodes down
- Control plane unavailable

**Immediate Actions**:
1. Check cloud provider status
2. Access backup cluster if available
3. If total failure, initiate full system restore

**Recovery**: Follow Section 3.4

### 4.4 Data Center / Region Failure

**Symptoms**:
- Complete loss of connectivity
- All services unreachable

**Immediate Actions**:
1. Activate disaster recovery site
2. Restore from backups in different region
3. Update DNS to point to DR site

**Recovery**: Follow Section 3.4

### 4.5 Security Breach

**Symptoms**:
- Unauthorized access detected
- Suspicious trading activity
- Data exfiltration alerts

**Immediate Actions**:
1. **STOP ALL TRADING IMMEDIATELY**
   ```bash
   kubectl delete deployment orchestrator -n cryptofunk
   ```
2. Isolate affected systems
3. Rotate all API keys and passwords
4. Contact exchange to freeze accounts
5. Notify security team
6. Preserve logs for forensics

**Recovery**:
1. Conduct security audit
2. Patch vulnerabilities
3. Restore from known-good backup
4. Implement additional security controls
5. Resume trading only after clearance

### 4.6 Accidental Data Deletion

**Symptoms**:
- Missing trades/positions
- Deleted configuration
- Lost historical data

**Immediate Actions**:
1. Stop any processes that might overwrite data
2. Identify what was deleted and when
3. Find latest good backup before deletion

**Recovery**: Point-in-Time Recovery (Section 3.1)

---

## 5. Incident Response Playbook

### Incident Severity Levels

| Level | Description | Response Time | Examples |
|-------|-------------|---------------|----------|
| **P0 - Critical** | System down, trading stopped, financial loss | Immediate | Database failure, security breach |
| **P1 - High** | Major functionality impaired | 1 hour | Agent failures, API degradation |
| **P2 - Medium** | Minor functionality impaired | 4 hours | Single agent down, cache issues |
| **P3 - Low** | Cosmetic issues, no impact | 1 business day | Dashboard display issue |

### Incident Response Steps

#### 1. Detection & Alerting

**How incidents are detected**:
- Automated alerts (Prometheus AlertManager)
- Monitoring dashboards (Grafana)
- User reports
- Scheduled health checks

**Alert Channels**:
- PagerDuty (P0/P1)
- Slack #incidents (P0/P1/P2)
- Email (P2/P3)

#### 2. Initial Response (First 5 Minutes)

1. **Acknowledge alert**
   - Respond in PagerDuty/Slack
   - "I'm on it, investigating"

2. **Assess severity**
   - Is trading affected?
   - Is money at risk?
   - Assign P0/P1/P2/P3 level

3. **Stop the bleeding**
   - For P0: Stop trading immediately
   - Contain the damage
   - Prevent escalation

#### 3. Investigation (Minutes 5-15)

1. **Gather information**
   ```bash
   # Check service health
   kubectl get pods -n cryptofunk
   curl http://localhost:8081/health

   # Check logs
   ./scripts/dev/watch-logs.sh --errors

   # Check metrics
   # Open Grafana dashboards
   ```

2. **Identify root cause**
   - Recent deployments?
   - Configuration changes?
   - External service failures?
   - Hardware issues?

3. **Determine fix strategy**
   - Quick fix available?
   - Need to rollback?
   - Need to restore from backup?

#### 4. Mitigation (Minutes 15-30)

1. **Implement fix**
   - Apply hotfix
   - Rollback deployment
   - Restore from backup
   - Scale resources

2. **Verify fix**
   - Run smoke tests
   - Check health endpoints
   - Monitor for recurrence

3. **Resume operations**
   - Gradually restore traffic
   - Monitor closely

#### 5. Communication

**During Incident**:
- Update status page every 15 minutes
- Post updates in #incidents Slack channel
- Notify stakeholders of P0/P1 incidents

**After Resolution**:
- Send all-clear notification
- Update status page
- Brief summary of cause and fix

#### 6. Post-Incident (Within 24 Hours)

1. **Create incident report**
   - Timeline of events
   - Root cause analysis
   - Impact assessment (financial, user, system)

2. **Document lessons learned**
   - What went well?
   - What could be improved?
   - Action items

3. **Schedule post-mortem** (P0/P1 only)
   - Blameless review
   - Identify improvements
   - Assign follow-up tasks

### Emergency Contacts

| Role | Primary | Backup | When to Contact |
|------|---------|--------|-----------------|
| On-Call Engineer | [Name] | [Name] | All incidents |
| Database Admin | [Name] | [Name] | Database issues |
| Security Lead | [Name] | [Name] | Security incidents |
| Tech Lead | [Name] | [Name] | Escalation, P0 |
| Exchange Support | Binance Support | | Exchange API issues |
| Cloud Provider | AWS Support | | Infrastructure issues |

### Incident Template

```markdown
# Incident Report: [Brief Description]

**Date**: 2025-11-06
**Severity**: P0/P1/P2/P3
**Status**: Investigating / Mitigated / Resolved
**Incident Commander**: [Name]

## Summary
Brief description of what happened.

## Impact
- Trading stopped for: 15 minutes
- Financial impact: $XXX
- Users affected: N/A (automated system)

## Timeline (UTC)
- 14:30 - Alert triggered: Database connection failures
- 14:31 - On-call acknowledged
- 14:32 - Trading stopped
- 14:35 - Root cause identified: PostgreSQL max_connections reached
- 14:40 - Mitigation: Restarted database, increased max_connections
- 14:45 - Trading resumed
- 14:50 - All-clear confirmed

## Root Cause
PostgreSQL max_connections limit (100) was exceeded due to connection leak in recent deployment.

## Resolution
1. Restarted PostgreSQL
2. Increased max_connections to 200
3. Fixed connection leak in code (PR #123)
4. Deployed hotfix

## Action Items
- [ ] Add monitoring for database connection count
- [ ] Implement connection pooling audit
- [ ] Update deployment checklist to verify connection handling

## Lessons Learned
- Connection monitoring was insufficient
- Database restart was quick due to good failover setup
- Need better load testing before deployment
```

---

## 6. Testing & Validation

### Backup Testing

**Frequency**: Monthly

**Procedure**:
1. Restore latest backup to staging environment
2. Verify data integrity
3. Run smoke tests
4. Document any issues

**Test Script**:
```bash
#!/bin/bash
# scripts/test/test-backup-restore.sh

echo "Testing backup restore procedure..."

# 1. Create test database
createdb cryptofunk_restore_test

# 2. Download latest backup
aws s3 cp s3://cryptofunk-backups/database/latest.sql.gz /tmp/

# 3. Restore
gunzip -c /tmp/latest.sql.gz | psql cryptofunk_restore_test

# 4. Verify data
psql cryptofunk_restore_test -c "SELECT COUNT(*) FROM trading_sessions;"
psql cryptofunk_restore_test -c "SELECT COUNT(*) FROM trades;"

# 5. Cleanup
dropdb cryptofunk_restore_test

echo "Backup restore test completed successfully!"
```

### Disaster Recovery Drill

**Frequency**: Quarterly

**Procedure**:
1. Simulate disaster scenario (e.g., complete database loss)
2. Follow disaster recovery procedures
3. Measure recovery time
4. Document issues and improvements
5. Update procedures based on learnings

**Drill Checklist**:
- [ ] Scenario selected and communicated
- [ ] Team assembled
- [ ] Backup systems verified accessible
- [ ] Procedure followed step-by-step
- [ ] Recovery time measured
- [ ] Functionality verified
- [ ] Issues documented
- [ ] Post-drill review completed
- [ ] Procedures updated

### Failover Testing

**Frequency**: Monthly

**For database replication**:
```bash
# Trigger planned failover
pg_ctl promote -D /var/lib/postgresql/data

# Verify new primary
psql -c "SELECT pg_is_in_recovery();"  # Should return 'f'

# Update application config to new primary
kubectl set env deployment/orchestrator DB_HOST=new-primary -n cryptofunk
```

---

## 7. Contact Information

### Internal Team

| Name | Role | Phone | Email | Availability |
|------|------|-------|-------|--------------|
| [Name] | On-Call Engineer | +1-XXX-XXX-XXXX | oncall@cryptofunk.io | 24/7 |
| [Name] | Database Admin | +1-XXX-XXX-XXXX | dba@cryptofunk.io | Business hours |
| [Name] | Security Lead | +1-XXX-XXX-XXXX | security@cryptofunk.io | Business hours, P0 24/7 |
| [Name] | Tech Lead | +1-XXX-XXX-XXXX | tech-lead@cryptofunk.io | Business hours, P0 24/7 |

### External Vendors

| Vendor | Contact | Phone | Email | Account # |
|--------|---------|-------|-------|-----------|
| Binance Support | | | support@binance.com | |
| AWS Support | | | | Account: XXX |
| Google Cloud Support | | | | Project: XXX |
| PagerDuty Support | | | support@pagerduty.com | |

### Emergency Escalation

1. On-Call Engineer (immediate)
2. Tech Lead (if no response in 15 min)
3. CTO (if no response in 30 min)

---

## Document Maintenance

**Review Schedule**: Quarterly
**Next Review**: 2025-02-06
**Owner**: Tech Lead
**Approver**: CTO

**Change Log**:
| Date | Version | Changes | Author |
|------|---------|---------|--------|
| 2025-11-06 | 1.0 | Initial version | [Your Name] |

---

**END OF DISASTER RECOVERY DOCUMENTATION**

This is a living document. Update it as procedures change, new scenarios are identified, or lessons are learned from incidents and drills.
