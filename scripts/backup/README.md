# Database Backup System

Automated PostgreSQL backup and restore system for CryptoFunk with support for multiple storage backends (S3, GCS, local).

## Features

- **Automated Backups**: Hourly, daily, and weekly backup schedules
- **Multiple Storage Backends**: Amazon S3, Google Cloud Storage, or local filesystem
- **Retention Policies**: Automatic cleanup of old backups (48 hourly, 30 daily, 52 weekly)
- **Point-in-Time Recovery**: Restore from any backup or specific point in time
- **Metadata Tracking**: JSON metadata for each backup with database stats and timing
- **Automated Testing**: Built-in backup/restore validation
- **Cloud Integration**: Native AWS S3 and Google Cloud Storage support
- **Compression**: gzip compression for efficient storage
- **Verification**: Automatic backup integrity verification

## Quick Start

### Basic Local Backup

```bash
# Create a backup
./scripts/backup/database-backup.sh --type hourly

# List available backups
./scripts/backup/restore-backup.sh --list

# Restore latest backup
./scripts/backup/restore-backup.sh --latest hourly

# Test backup/restore system
./scripts/backup/test-backup.sh
```

### Environment Variables

```bash
# Database connection
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=postgres
export DATABASE_PASSWORD=your_password
export DATABASE_NAME=cryptofunk

# Storage configuration
export BACKUP_STORAGE=local              # or s3, gcs
export BACKUP_LOCAL_PATH=/var/backups/cryptofunk
export BACKUP_BUCKET=cryptofunk-backups  # for S3/GCS
```

## Scripts

| Script | Purpose | Usage |
|--------|---------|-------|
| `database-backup.sh` | Create database backups | `./database-backup.sh --type hourly` |
| `cleanup-old-backups.sh` | Enforce retention policies | `./cleanup-old-backups.sh --all` |
| `restore-backup.sh` | Restore from backup | `./restore-backup.sh --latest daily` |
| `test-backup.sh` | Test backup/restore system | `./test-backup.sh` |

## Storage Backend Configuration

### Local Storage (Default)

```bash
export BACKUP_STORAGE=local
export BACKUP_LOCAL_PATH=/var/backups/cryptofunk
mkdir -p $BACKUP_LOCAL_PATH/{hourly,daily,weekly}
```

Backups are stored in:
```
/var/backups/cryptofunk/
├── hourly/
│   └── 20251106/
│       ├── cryptofunk_hourly_20251106_140000.sql.gz
│       └── cryptofunk_hourly_20251106_140000.meta.json
├── daily/
└── weekly/
```

### Amazon S3

```bash
export BACKUP_STORAGE=s3
export BACKUP_BUCKET=cryptofunk-backups
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_DEFAULT_REGION=us-east-1
```

Install AWS CLI:
```bash
# macOS
brew install awscli

# Linux
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install
```

Create S3 bucket:
```bash
aws s3 mb s3://cryptofunk-backups
aws s3api put-bucket-versioning --bucket cryptofunk-backups --versioning-configuration Status=Enabled
```

### Google Cloud Storage

```bash
export BACKUP_STORAGE=gcs
export BACKUP_BUCKET=cryptofunk-backups
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json
```

Install gcloud CLI:
```bash
# macOS
brew install google-cloud-sdk

# Linux
curl https://sdk.cloud.google.com | bash
exec -l $SHELL
gcloud init
```

Create GCS bucket:
```bash
gsutil mb gs://cryptofunk-backups
gsutil versioning set on gs://cryptofunk-backups
```

## Automated Backup Setup

### Using Cron (Linux/macOS)

```bash
# Edit crontab
crontab -e

# Add backup schedules
# Hourly backups (every hour)
0 * * * * cd /path/to/cryptofunk && ./scripts/backup/database-backup.sh --type hourly >> /var/log/cryptofunk/backup.log 2>&1

# Daily backups (2 AM)
0 2 * * * cd /path/to/cryptofunk && ./scripts/backup/database-backup.sh --type daily >> /var/log/cryptofunk/backup.log 2>&1

# Weekly backups (Sunday 3 AM)
0 3 * * 0 cd /path/to/cryptofunk && ./scripts/backup/database-backup.sh --type weekly >> /var/log/cryptofunk/backup.log 2>&1

# Cleanup old backups (daily at 4 AM)
0 4 * * * cd /path/to/cryptofunk && ./scripts/backup/cleanup-old-backups.sh --all >> /var/log/cryptofunk/cleanup.log 2>&1

# Test backups monthly (1st of month, 5 AM)
0 5 1 * * cd /path/to/cryptofunk && ./scripts/backup/test-backup.sh >> /var/log/cryptofunk/test-backup.log 2>&1
```

### Using Kubernetes CronJobs

```yaml
# backup-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: cryptofunk-hourly-backup
  namespace: cryptofunk
spec:
  schedule: "0 * * * *"  # Every hour
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: cryptofunk/backup:latest
            command: ["/scripts/backup/database-backup.sh"]
            args: ["--type", "hourly"]
            env:
            - name: DATABASE_HOST
              value: postgres.cryptofunk.svc.cluster.local
            - name: DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: cryptofunk-secrets
                  key: database-password
            - name: BACKUP_STORAGE
              value: s3
            - name: BACKUP_BUCKET
              value: cryptofunk-backups
          restartPolicy: OnFailure
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: cryptofunk-daily-backup
  namespace: cryptofunk
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: cryptofunk/backup:latest
            command: ["/scripts/backup/database-backup.sh"]
            args: ["--type", "daily"]
            env:
            - name: DATABASE_HOST
              value: postgres.cryptofunk.svc.cluster.local
            - name: DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: cryptofunk-secrets
                  key: database-password
            - name: BACKUP_STORAGE
              value: s3
            - name: BACKUP_BUCKET
              value: cryptofunk-backups
          restartPolicy: OnFailure
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: cryptofunk-weekly-backup
  namespace: cryptofunk
spec:
  schedule: "0 3 * * 0"  # Weekly on Sunday at 3 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: cryptofunk/backup:latest
            command: ["/scripts/backup/database-backup.sh"]
            args: ["--type", "weekly"]
            env:
            - name: DATABASE_HOST
              value: postgres.cryptofunk.svc.cluster.local
            - name: DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: cryptofunk-secrets
                  key: database-password
            - name: BACKUP_STORAGE
              value: s3
            - name: BACKUP_BUCKET
              value: cryptofunk-backups
          restartPolicy: OnFailure
```

Deploy CronJobs:
```bash
kubectl apply -f backup-cronjob.yaml
kubectl get cronjobs -n cryptofunk
```

## Retention Policies

Default retention periods:

| Backup Type | Retention | Storage Period |
|-------------|-----------|----------------|
| Hourly | 48 backups | 2 days |
| Daily | 30 backups | 1 month |
| Weekly | 52 backups | 1 year |

Modify retention in `cleanup-old-backups.sh`:
```bash
HOURLY_KEEP=48   # Keep last 48 hourly backups (2 days)
DAILY_KEEP=30    # Keep last 30 daily backups (1 month)
WEEKLY_KEEP=52   # Keep last 52 weekly backups (1 year)
```

## Recovery Scenarios

### Scenario 1: Restore Latest Backup

```bash
# Stop services
docker-compose stop orchestrator api

# Restore latest daily backup
./scripts/backup/restore-backup.sh --latest daily

# Restart services
docker-compose start orchestrator api
```

### Scenario 2: Restore Specific Backup

```bash
# List available backups
./scripts/backup/restore-backup.sh --list

# Restore specific backup
./scripts/backup/restore-backup.sh cryptofunk_daily_20251106_020000.sql.gz
```

### Scenario 3: Restore from Cloud Storage

```bash
# Set storage backend
export BACKUP_STORAGE=s3
export BACKUP_BUCKET=cryptofunk-backups

# Restore latest backup (auto-downloads from S3)
./scripts/backup/restore-backup.sh --latest daily
```

### Scenario 4: Point-in-Time Recovery (PITR)

For precise point-in-time recovery, you'll need WAL (Write-Ahead Log) archiving enabled:

```bash
# Enable WAL archiving in PostgreSQL (postgresql.conf)
wal_level = replica
archive_mode = on
archive_command = 'test ! -f /path/to/archive/%f && cp %p /path/to/archive/%f'

# Restore base backup
./scripts/backup/restore-backup.sh --latest daily

# Replay WAL files to specific point in time
# (Requires manual configuration in recovery.conf)
```

## Monitoring and Alerting

### Check Last Backup

```bash
# For S3
aws s3 ls s3://cryptofunk-backups/database/hourly/ --recursive | tail -1

# For GCS
gsutil ls gs://cryptofunk-backups/database/hourly/** | tail -1

# For local
ls -lht /var/backups/cryptofunk/hourly/ | head -5
```

### Prometheus Metrics

Add these metrics to your monitoring:

```yaml
# prometheus-backup-exporter.yaml
- job_name: 'backup-status'
  static_configs:
  - targets: ['localhost:9100']

  # Alert if no backup in last 2 hours
  - alert: BackupMissing
    expr: time() - backup_last_success_timestamp > 7200
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "Database backup missing for 2+ hours"
```

### Slack Notifications (Optional)

Add to end of `database-backup.sh`:

```bash
if command -v curl &> /dev/null; then
    curl -X POST -H 'Content-type: application/json' \
        --data "{\"text\":\"✅ Database backup completed: $BACKUP_FILE ($BACKUP_SIZE)\"}" \
        $SLACK_WEBHOOK_URL
fi
```

## Troubleshooting

### Backup Fails: "Cannot connect to database"

**Solution**: Verify database credentials and network connectivity:
```bash
psql -h $DATABASE_HOST -p $DATABASE_PORT -U $DATABASE_USER -d $DATABASE_NAME -c "SELECT 1;"
```

### Restore Fails: "Permission denied"

**Solution**: Ensure the database user has sufficient privileges:
```sql
ALTER USER postgres WITH SUPERUSER;
-- Or grant specific privileges
GRANT ALL PRIVILEGES ON DATABASE cryptofunk TO postgres;
```

### S3 Upload Fails: "Access Denied"

**Solution**: Verify AWS credentials and bucket permissions:
```bash
aws sts get-caller-identity
aws s3 ls s3://cryptofunk-backups/
```

Required S3 bucket policy:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::cryptofunk-backups/*",
        "arn:aws:s3:::cryptofunk-backups"
      ]
    }
  ]
}
```

### Backup Too Large

**Solutions**:
1. Enable PostgreSQL compression:
```bash
pg_dump --format=custom --compress=9 -d cryptofunk > backup.dump
```

2. Use parallel dump:
```bash
pg_dump --format=directory --jobs=4 -d cryptofunk -f backup_dir/
```

3. Exclude large tables:
```bash
pg_dump --exclude-table=large_log_table -d cryptofunk
```

### Restore Takes Too Long

**Solutions**:
1. Use parallel restore:
```bash
pg_restore --jobs=4 -d cryptofunk backup.dump
```

2. Disable triggers during restore:
```bash
psql -c "SET session_replication_role = replica;" -d cryptofunk
# ... perform restore ...
psql -c "SET session_replication_role = DEFAULT;" -d cryptofunk
```

## Best Practices

### 1. Test Backups Monthly

```bash
# Run automated test
./scripts/backup/test-backup.sh

# Or manually verify
./scripts/backup/restore-backup.sh --latest daily
# ... check data integrity ...
```

### 2. Store Backups in Multiple Locations

```bash
# Primary: S3
export BACKUP_STORAGE=s3
./scripts/backup/database-backup.sh --type daily

# Secondary: Local
export BACKUP_STORAGE=local
./scripts/backup/database-backup.sh --type daily
```

### 3. Encrypt Sensitive Backups

```bash
# Encrypt backup before upload
BACKUP_FILE="backup.sql.gz"
gpg --encrypt --recipient admin@cryptofunk.com $BACKUP_FILE

# Upload encrypted file
aws s3 cp ${BACKUP_FILE}.gpg s3://cryptofunk-backups/
```

### 4. Monitor Backup Size Trends

```bash
# Check backup growth
aws s3 ls s3://cryptofunk-backups/database/daily/ --recursive --human-readable | \
    awk '{print $3, $4}' | sort
```

### 5. Document Recovery Procedures

Keep a printed copy of recovery procedures in case of complete infrastructure failure. See `docs/DISASTER_RECOVERY.md`.

## Security Considerations

### 1. Protect Database Credentials

```bash
# Use environment files (not checked into git)
echo "DATABASE_PASSWORD=secret" >> .env
chmod 600 .env

# Or use secret management
export DATABASE_PASSWORD=$(aws secretsmanager get-secret-value --secret-id cryptofunk/db/password --query SecretString --output text)
```

### 2. Restrict Backup Access

```bash
# S3 bucket encryption
aws s3api put-bucket-encryption \
    --bucket cryptofunk-backups \
    --server-side-encryption-configuration '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'

# Enable bucket versioning
aws s3api put-bucket-versioning \
    --bucket cryptofunk-backups \
    --versioning-configuration Status=Enabled
```

### 3. Audit Backup Access

```bash
# Enable S3 access logging
aws s3api put-bucket-logging \
    --bucket cryptofunk-backups \
    --bucket-logging-status file://logging.json
```

### 4. Rotate Backup Encryption Keys

```bash
# Use AWS KMS for encryption
aws s3 cp backup.sql.gz s3://cryptofunk-backups/ \
    --server-side-encryption aws:kms \
    --ssekms-key-id arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012
```

## Performance Tuning

### Large Database Optimization

```bash
# Use custom format for faster restore
pg_dump --format=custom --compress=9 -d cryptofunk -f backup.dump

# Parallel dump (4 jobs)
pg_dump --format=directory --jobs=4 -d cryptofunk -f backup_dir/

# Parallel restore (4 jobs)
pg_restore --jobs=4 -d cryptofunk backup.dump
```

### Network Optimization

```bash
# Use AWS S3 Transfer Acceleration
aws configure set s3.use_accelerate_endpoint true

# Multi-part upload for large files
aws s3 cp backup.sql.gz s3://cryptofunk-backups/ \
    --storage-class STANDARD_IA \
    --metadata backup_type=daily
```

### TimescaleDB-Specific

```bash
# Backup with TimescaleDB chunks
pg_dump --schema=public \
    --exclude-table='_timescaledb_*' \
    -d cryptofunk | gzip > backup.sql.gz
```

## FAQ

**Q: How long should I keep backups?**

A: Follow the 3-2-1 rule:
- 3 copies of data
- 2 different storage types (S3 + local)
- 1 offsite backup

**Q: Can I backup while the system is running?**

A: Yes, `pg_dump` creates consistent snapshots without locking. For zero-downtime backups, consider:
- PostgreSQL continuous archiving (WAL)
- Replica-based backups
- Filesystem snapshots (LVM, ZFS)

**Q: What's the difference between backup types?**

A:
- **Hourly**: Frequent backups for recent recovery (2-day retention)
- **Daily**: Balance of frequency and storage (1-month retention)
- **Weekly**: Long-term historical backups (1-year retention)

**Q: How do I backup just schema or just data?**

A:
```bash
# Schema only
pg_dump --schema-only -d cryptofunk > schema.sql

# Data only
pg_dump --data-only -d cryptofunk > data.sql

# Specific table
pg_dump -t positions -d cryptofunk > positions.sql
```

**Q: Can I restore to a different database?**

A: Yes:
```bash
# Create new database
createdb cryptofunk_staging

# Restore to it
gunzip -c backup.sql.gz | psql -d cryptofunk_staging
```

**Q: How do I backup extensions (TimescaleDB, pgvector)?**

A: Extensions are included in schema dumps. After restore, run:
```sql
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

## Additional Resources

- PostgreSQL Backup Documentation: https://www.postgresql.org/docs/current/backup.html
- TimescaleDB Backup Best Practices: https://docs.timescale.com/latest/using-timescaledb/backup
- AWS S3 CLI Reference: https://docs.aws.amazon.com/cli/latest/reference/s3/
- Google Cloud Storage Documentation: https://cloud.google.com/storage/docs
- Disaster Recovery Procedures: `docs/DISASTER_RECOVERY.md`
- Production Checklist: `docs/PRODUCTION_CHECKLIST.md`

## Support

For backup-related issues:
1. Check logs in `/var/log/cryptofunk/backup.log`
2. Run `./scripts/backup/test-backup.sh` to validate system
3. Review troubleshooting section above
4. Contact: ops@cryptofunk.com
