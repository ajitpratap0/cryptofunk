#!/usr/bin/env bash
#
# database-backup.sh - Automated PostgreSQL backup to S3/GCS/local
#
# This script creates compressed backups of the CryptoFunk database
# and uploads them to cloud storage with retention policies.
#
# Usage:
#   ./scripts/backup/database-backup.sh [--type hourly|daily|weekly]
#
# Environment Variables:
#   DATABASE_HOST      - PostgreSQL host (default: localhost)
#   DATABASE_PORT      - PostgreSQL port (default: 5432)
#   DATABASE_USER      - PostgreSQL user (default: postgres)
#   DATABASE_PASSWORD  - PostgreSQL password (required)
#   DATABASE_NAME      - Database name (default: cryptofunk)
#   BACKUP_STORAGE     - Storage type: s3|gcs|local (default: local)
#   BACKUP_BUCKET      - S3 bucket or GCS bucket name
#   BACKUP_LOCAL_PATH  - Local backup path (default: /var/backups/cryptofunk)

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
DB_HOST="${DATABASE_HOST:-localhost}"
DB_PORT="${DATABASE_PORT:-5432}"
DB_USER="${DATABASE_USER:-postgres}"
DB_PASS="${DATABASE_PASSWORD:-}"
DB_NAME="${DATABASE_NAME:-cryptofunk}"

BACKUP_TYPE="${1:---type hourly}"
BACKUP_TYPE="${BACKUP_TYPE#--type }"  # Remove --type prefix

STORAGE_TYPE="${BACKUP_STORAGE:-local}"
BACKUP_BUCKET="${BACKUP_BUCKET:-}"
LOCAL_PATH="${BACKUP_LOCAL_PATH:-/var/backups/cryptofunk}"

# Timestamps
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DATE=$(date +%Y%m%d)
HOUR=$(date +%H)

# Backup file names
BACKUP_FILE="cryptofunk_${BACKUP_TYPE}_${TIMESTAMP}.sql.gz"
BACKUP_METADATA="cryptofunk_${BACKUP_TYPE}_${TIMESTAMP}.meta.json"

# Temporary directory
TMP_DIR="/tmp/cryptofunk-backup-$$"
mkdir -p "$TMP_DIR"

# Cleanup on exit
trap 'rm -rf "$TMP_DIR"' EXIT

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}CryptoFunk Database Backup${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Type: ${GREEN}$BACKUP_TYPE${NC}"
echo -e "Database: ${GREEN}$DB_NAME@$DB_HOST:$DB_PORT${NC}"
echo -e "Storage: ${GREEN}$STORAGE_TYPE${NC}"
echo -e "Timestamp: ${GREEN}$TIMESTAMP${NC}"
echo ""

# Verify database is accessible
echo -e "${YELLOW}[1/5] Verifying database connection...${NC}"
if ! PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" > /dev/null 2>&1; then
    echo -e "${RED}✗ Cannot connect to database${NC}"
    echo -e "${RED}Check DATABASE_HOST, DATABASE_PORT, DATABASE_USER, DATABASE_PASSWORD${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Database connection verified${NC}"

# Create backup
echo -e "${YELLOW}[2/5] Creating database backup...${NC}"
START_TIME=$(date +%s)

PGPASSWORD="$DB_PASS" pg_dump \
    -h "$DB_HOST" \
    -p "$DB_PORT" \
    -U "$DB_USER" \
    -d "$DB_NAME" \
    --format=plain \
    --no-owner \
    --no-acl \
    --verbose \
    2>&1 | gzip > "$TMP_DIR/$BACKUP_FILE"

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
BACKUP_SIZE=$(du -h "$TMP_DIR/$BACKUP_FILE" | cut -f1)

echo -e "${GREEN}✓ Backup created (${BACKUP_SIZE}, ${DURATION}s)${NC}"

# Create metadata file
echo -e "${YELLOW}[3/5] Creating backup metadata...${NC}"
cat > "$TMP_DIR/$BACKUP_METADATA" << METADATA
{
  "backup_type": "$BACKUP_TYPE",
  "timestamp": "$TIMESTAMP",
  "database": {
    "host": "$DB_HOST",
    "port": $DB_PORT,
    "name": "$DB_NAME",
    "user": "$DB_USER"
  },
  "backup_file": "$BACKUP_FILE",
  "backup_size_bytes": $(stat -f%z "$TMP_DIR/$BACKUP_FILE" 2>/dev/null || stat -c%s "$TMP_DIR/$BACKUP_FILE"),
  "backup_size_human": "$BACKUP_SIZE",
  "duration_seconds": $DURATION,
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "postgresql_version": "$(PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT version();" | head -1 | xargs)",
  "table_counts": {
$(PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
SELECT '    \"' || tablename || '\": ' || n_live_tup
FROM pg_stat_user_tables
ORDER BY tablename;" | paste -sd ',' -)
  }
}
METADATA

echo -e "${GREEN}✓ Metadata created${NC}"

# Upload backup
echo -e "${YELLOW}[4/5] Uploading backup...${NC}"

case "$STORAGE_TYPE" in
    s3)
        if [[ -z "$BACKUP_BUCKET" ]]; then
            echo -e "${RED}✗ BACKUP_BUCKET not set for S3 storage${NC}"
            exit 1
        fi

        # Determine S3 path based on backup type
        S3_PATH="s3://$BACKUP_BUCKET/database/$BACKUP_TYPE/$DATE/"

        # Upload backup file
        if aws s3 cp "$TMP_DIR/$BACKUP_FILE" "$S3_PATH$BACKUP_FILE" --storage-class STANDARD_IA; then
            echo -e "${GREEN}✓ Backup uploaded to $S3_PATH$BACKUP_FILE${NC}"
        else
            echo -e "${RED}✗ Failed to upload backup to S3${NC}"
            exit 1
        fi

        # Upload metadata
        aws s3 cp "$TMP_DIR/$BACKUP_METADATA" "$S3_PATH$BACKUP_METADATA" --content-type "application/json"
        ;;

    gcs)
        if [[ -z "$BACKUP_BUCKET" ]]; then
            echo -e "${RED}✗ BACKUP_BUCKET not set for GCS storage${NC}"
            exit 1
        fi

        GCS_PATH="gs://$BACKUP_BUCKET/database/$BACKUP_TYPE/$DATE/"

        # Upload to Google Cloud Storage
        if gsutil cp "$TMP_DIR/$BACKUP_FILE" "$GCS_PATH$BACKUP_FILE"; then
            echo -e "${GREEN}✓ Backup uploaded to $GCS_PATH$BACKUP_FILE${NC}"
        else
            echo -e "${RED}✗ Failed to upload backup to GCS${NC}"
            exit 1
        fi

        # Upload metadata
        gsutil cp "$TMP_DIR/$BACKUP_METADATA" "$GCS_PATH$BACKUP_METADATA"
        ;;

    local)
        # Create directory structure
        mkdir -p "$LOCAL_PATH/$BACKUP_TYPE/$DATE"

        # Copy backup file
        cp "$TMP_DIR/$BACKUP_FILE" "$LOCAL_PATH/$BACKUP_TYPE/$DATE/"
        cp "$TMP_DIR/$BACKUP_METADATA" "$LOCAL_PATH/$BACKUP_TYPE/$DATE/"

        echo -e "${GREEN}✓ Backup saved to $LOCAL_PATH/$BACKUP_TYPE/$DATE/$BACKUP_FILE${NC}"
        ;;

    *)
        echo -e "${RED}✗ Unknown storage type: $STORAGE_TYPE${NC}"
        echo -e "${YELLOW}Supported types: s3, gcs, local${NC}"
        exit 1
        ;;
esac

# Cleanup old backups (retention policy)
echo -e "${YELLOW}[5/5] Applying retention policy...${NC}"

if [[ -x "$(dirname $0)/cleanup-old-backups.sh" ]]; then
    "$(dirname $0)/cleanup-old-backups.sh" --type "$BACKUP_TYPE"
else
    echo -e "${YELLOW}⚠ Cleanup script not found, skipping retention policy${NC}"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Backup completed successfully!${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "Backup file: ${GREEN}$BACKUP_FILE${NC}"
echo -e "Backup size: ${GREEN}$BACKUP_SIZE${NC}"
echo -e "Duration: ${GREEN}${DURATION}s${NC}"
echo -e "Storage: ${GREEN}$STORAGE_TYPE${NC}"
echo ""

# Send notification (optional)
if command -v slack-notify &> /dev/null; then
    slack-notify "✅ Database backup completed: $BACKUP_FILE ($BACKUP_SIZE)"
fi

exit 0
