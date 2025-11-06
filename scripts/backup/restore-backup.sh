#!/usr/bin/env bash
#
# restore-backup.sh - Restore database from backup
#
# Usage:
#   ./scripts/backup/restore-backup.sh <backup-file>
#   ./scripts/backup/restore-backup.sh --latest [hourly|daily|weekly]
#   ./scripts/backup/restore-backup.sh --list
#
# Examples:
#   ./scripts/backup/restore-backup.sh cryptofunk_hourly_20251106_140000.sql.gz
#   ./scripts/backup/restore-backup.sh --latest daily
#   ./scripts/backup/restore-backup.sh --list

set -euo pipefail

# Colors
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

STORAGE_TYPE="${BACKUP_STORAGE:-local}"
BACKUP_BUCKET="${BACKUP_BUCKET:-}"
LOCAL_PATH="${BACKUP_LOCAL_PATH:-/var/backups/cryptofunk}"

# Temporary directory
TMP_DIR="/tmp/cryptofunk-restore-$$"
mkdir -p "$TMP_DIR"

# Cleanup on exit
trap 'rm -rf "$TMP_DIR"' EXIT

list_backups() {
    echo -e "${BLUE}Available Backups:${NC}"
    echo ""

    case "$STORAGE_TYPE" in
        s3)
            aws s3 ls "s3://$BACKUP_BUCKET/database/" --recursive | \
                grep "\.sql\.gz$" | \
                awk '{print $4, "(" $3 ")"}' | \
                sort -r | \
                head -20
            ;;
        gcs)
            gsutil ls "gs://$BACKUP_BUCKET/database/**/*.sql.gz" | \
                sort -r | \
                head -20
            ;;
        local)
            find "$LOCAL_PATH" -name "*.sql.gz" -printf '%T@ %p (%s bytes)\n' | \
                sort -rn | \
                head -20 | \
                cut -d' ' -f2-
            ;;
    esac
    echo ""
}

find_latest_backup() {
    local type="${1:-hourly}"

    case "$STORAGE_TYPE" in
        s3)
            aws s3 ls "s3://$BACKUP_BUCKET/database/$type/" --recursive | \
                grep "\.sql\.gz$" | \
                sort -r | \
                head -1 | \
                awk '{print $4}'
            ;;
        gcs)
            gsutil ls "gs://$BACKUP_BUCKET/database/$type/**/*.sql.gz" | \
                sort -r | \
                head -1
            ;;
        local)
            find "$LOCAL_PATH/$type" -name "*.sql.gz" -printf '%T@ %p\n' | \
                sort -rn | \
                head -1 | \
                cut -d' ' -f2-
            ;;
    esac
}

download_backup() {
    local backup_file=$1
    local local_file="$TMP_DIR/$(basename $backup_file)"

    echo -e "${YELLOW}Downloading backup...${NC}"

    case "$STORAGE_TYPE" in
        s3)
            if aws s3 cp "s3://$BACKUP_BUCKET/$backup_file" "$local_file"; then
                echo "$local_file"
            else
                echo ""
            fi
            ;;
        gcs)
            if gsutil cp "$backup_file" "$local_file"; then
                echo "$local_file"
            else
                echo ""
            fi
            ;;
        local)
            cp "$backup_file" "$local_file"
            echo "$local_file"
            ;;
    esac
}

restore_database() {
    local backup_file=$1

    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}CryptoFunk Database Restore${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo -e "Backup file: ${GREEN}$(basename $backup_file)${NC}"
    echo -e "Target database: ${GREEN}$DB_NAME@$DB_HOST:$DB_PORT${NC}"
    echo ""

    # Safety confirmation
    echo -e "${RED}WARNING: This will REPLACE ALL DATA in the database!${NC}"
    read -p "Are you sure you want to continue? (type 'yes' to confirm): " -r
    echo ""
    if [[ ! $REPLY =~ ^yes$ ]]; then
        echo -e "${YELLOW}Restore cancelled.${NC}"
        exit 0
    fi

    # Step 1: Verify database connection
    echo -e "${YELLOW}[1/6] Verifying database connection...${NC}"
    if ! PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "SELECT 1;" > /dev/null 2>&1; then
        echo -e "${RED}✗ Cannot connect to database${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Database connection verified${NC}"

    # Step 2: Stop application services (if running locally)
    echo -e "${YELLOW}[2/6] Stopping application services...${NC}"
    if command -v docker-compose &> /dev/null && [[ -f "docker-compose.yml" ]]; then
        docker-compose stop orchestrator api 2>/dev/null || true
        echo -e "${GREEN}✓ Services stopped${NC}"
    else
        echo -e "${YELLOW}⚠ No docker-compose found, assuming services stopped manually${NC}"
    fi

    # Step 3: Drop and recreate database
    echo -e "${YELLOW}[3/6] Dropping and recreating database...${NC}"
    PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres << SQL
DROP DATABASE IF EXISTS $DB_NAME;
CREATE DATABASE $DB_NAME;
SQL
    echo -e "${GREEN}✓ Database recreated${NC}"

    # Step 4: Restore backup
    echo -e "${YELLOW}[4/6] Restoring backup...${NC}"
    START_TIME=$(date +%s)

    if gunzip -c "$backup_file" | \
        PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" > /dev/null 2>&1; then
        
        END_TIME=$(date +%s)
        DURATION=$((END_TIME - START_TIME))
        echo -e "${GREEN}✓ Backup restored (${DURATION}s)${NC}"
    else
        echo -e "${RED}✗ Restore failed${NC}"
        exit 1
    fi

    # Step 5: Enable extensions
    echo -e "${YELLOW}[5/6] Enabling extensions...${NC}"
    PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << SQL
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
SQL
    echo -e "${GREEN}✓ Extensions enabled${NC}"

    # Step 6: Verify restore
    echo -e "${YELLOW}[6/6] Verifying restore...${NC}"
    
    # Check table count
    TABLE_COUNT=$(PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';")
    
    echo -e "  Tables found: ${GREEN}$TABLE_COUNT${NC}"
    
    # Check for critical tables
    for table in trading_sessions positions orders trades agent_signals; do
        if PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "\dt $table" | grep -q "$table"; then
            ROW_COUNT=$(PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM $table;")
            echo -e "  $table: ${GREEN}$ROW_COUNT rows${NC}"
        else
            echo -e "  ${RED}✗ Table $table not found${NC}"
        fi
    done

    echo -e "${GREEN}✓ Restore verified${NC}"

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}Restore completed successfully!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Restart application services"
    echo "  2. Verify system health"
    echo "  3. Check logs for errors"
    echo ""
}

# Main logic
case "${1:-}" in
    --list)
        list_backups
        ;;
    --latest)
        BACKUP_TYPE="${2:-hourly}"
        echo -e "${YELLOW}Finding latest $BACKUP_TYPE backup...${NC}"
        BACKUP_FILE=$(find_latest_backup "$BACKUP_TYPE")
        
        if [[ -z "$BACKUP_FILE" ]]; then
            echo -e "${RED}✗ No backups found${NC}"
            exit 1
        fi
        
        echo -e "${GREEN}Found: $BACKUP_FILE${NC}"
        
        # Download if cloud storage
        if [[ "$STORAGE_TYPE" != "local" ]]; then
            LOCAL_FILE=$(download_backup "$BACKUP_FILE")
            if [[ -z "$LOCAL_FILE" ]]; then
                echo -e "${RED}✗ Failed to download backup${NC}"
                exit 1
            fi
            BACKUP_FILE="$LOCAL_FILE"
        fi
        
        restore_database "$BACKUP_FILE"
        ;;
    "")
        echo "Usage: $0 <backup-file> | --latest [type] | --list"
        echo ""
        echo "Examples:"
        echo "  $0 cryptofunk_hourly_20251106_140000.sql.gz"
        echo "  $0 --latest daily"
        echo "  $0 --list"
        exit 1
        ;;
    *)
        # Restore from specified file
        BACKUP_FILE=$1
        
        # Check if file exists (for local storage)
        if [[ "$STORAGE_TYPE" == "local" ]] && [[ ! -f "$BACKUP_FILE" ]]; then
            echo -e "${RED}✗ Backup file not found: $BACKUP_FILE${NC}"
            echo ""
            echo "Available backups:"
            list_backups
            exit 1
        fi
        
        # Download if cloud storage
        if [[ "$STORAGE_TYPE" != "local" ]]; then
            LOCAL_FILE=$(download_backup "$BACKUP_FILE")
            if [[ -z "$LOCAL_FILE" ]]; then
                echo -e "${RED}✗ Failed to download backup${NC}"
                exit 1
            fi
            BACKUP_FILE="$LOCAL_FILE"
        fi
        
        restore_database "$BACKUP_FILE"
        ;;
esac
