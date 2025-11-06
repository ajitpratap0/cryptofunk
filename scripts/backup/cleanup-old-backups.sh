#!/usr/bin/env bash
#
# cleanup-old-backups.sh - Enforce backup retention policies
#
# Retention Policy:
# - Hourly backups: Keep last 48 (2 days)
# - Daily backups: Keep last 30 (1 month)
# - Weekly backups: Keep last 52 (1 year)
#
# Usage:
#   ./scripts/backup/cleanup-old-backups.sh [--type hourly|daily|weekly]
#   ./scripts/backup/cleanup-old-backups.sh --all  # Clean all backup types

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
STORAGE_TYPE="${BACKUP_STORAGE:-local}"
BACKUP_BUCKET="${BACKUP_BUCKET:-}"
LOCAL_PATH="${BACKUP_LOCAL_PATH:-/var/backups/cryptofunk}"

# Retention settings
HOURLY_KEEP=48   # Keep last 48 hourly backups (2 days)
DAILY_KEEP=30    # Keep last 30 daily backups (1 month)
WEEKLY_KEEP=52   # Keep last 52 weekly backups (1 year)

cleanup_type() {
    local type=$1
    local keep_count=0

    case "$type" in
        hourly) keep_count=$HOURLY_KEEP ;;
        daily)  keep_count=$DAILY_KEEP ;;
        weekly) keep_count=$WEEKLY_KEEP ;;
        *)
            echo -e "${YELLOW}Unknown backup type: $type${NC}"
            return 1
            ;;
    esac

    echo -e "${YELLOW}Cleaning $type backups (keeping last $keep_count)...${NC}"

    case "$STORAGE_TYPE" in
        s3)
            # List backups, sort by date, delete old ones
            local bucket_path="s3://$BACKUP_BUCKET/database/$type/"
            
            # Get list of backup files (excluding metadata)
            local files=$(aws s3 ls "$bucket_path" --recursive | grep "\.sql\.gz$" | sort -r | awk '{print $4}')
            local count=0
            
            while IFS= read -r file; do
                ((count++))
                if [[ $count -gt $keep_count ]]; then
                    echo "  Deleting: $file"
                    aws s3 rm "s3://$BACKUP_BUCKET/$file"
                    # Also delete metadata
                    aws s3 rm "s3://$BACKUP_BUCKET/${file%.sql.gz}.meta.json" 2>/dev/null || true
                fi
            done <<< "$files"
            ;;

        gcs)
            local bucket_path="gs://$BACKUP_BUCKET/database/$type/"
            
            local files=$(gsutil ls "$bucket_path**/*.sql.gz" | sort -r)
            local count=0
            
            while IFS= read -r file; do
                ((count++))
                if [[ $count -gt $keep_count ]]; then
                    echo "  Deleting: $file"
                    gsutil rm "$file"
                    gsutil rm "${file%.sql.gz}.meta.json" 2>/dev/null || true
                fi
            done <<< "$files"
            ;;

        local)
            # Find backup files, sort by date, keep newest N
            find "$LOCAL_PATH/$type" -name "*.sql.gz" -type f -printf '%T@ %p\n' | \
                sort -rn | \
                tail -n +$((keep_count + 1)) | \
                cut -d' ' -f2- | \
                while read -r file; do
                    echo "  Deleting: $file"
                    rm -f "$file"
                    rm -f "${file%.sql.gz}.meta.json" 2>/dev/null || true
                done

            # Clean empty directories
            find "$LOCAL_PATH/$type" -type d -empty -delete 2>/dev/null || true
            ;;
    esac

    echo -e "${GREEN}✓ Cleanup completed for $type backups${NC}"
}

# Parse arguments
if [[ "${1:-}" == "--all" ]]; then
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}Backup Cleanup (All Types)${NC}"
    echo -e "${YELLOW}========================================${NC}"
    cleanup_type "hourly"
    cleanup_type "daily"
    cleanup_type "weekly"
elif [[ "${1:-}" == "--type" ]]; then
    cleanup_type "${2:-hourly}"
else
    # Default: clean hourly backups
    cleanup_type "hourly"
fi

exit 0
