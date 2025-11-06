#!/usr/bin/env bash
#
# test-backup.sh - Test backup and restore procedures
#
# This script validates that:
# 1. Backups can be created successfully
# 2. Backups can be restored successfully
# 3. Data integrity is maintained
#
# Usage:
#   ./scripts/backup/test-backup.sh

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
DB_PASS="${DATABASE_PASSWORD:-postgres}"
TEST_DB="cryptofunk_backup_test"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Backup/Restore Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Step 1: Create test database with sample data
echo -e "${YELLOW}[1/5] Creating test database...${NC}"

PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres << SQL
DROP DATABASE IF EXISTS $TEST_DB;
CREATE DATABASE $TEST_DB;
SQL

# Create sample schema and data
PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" << SQL
CREATE TABLE test_table (
    id SERIAL PRIMARY KEY,
    data TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO test_table (data) VALUES
    ('Test data 1'),
    ('Test data 2'),
    ('Test data 3'),
    ('Test data 4'),
    ('Test data 5');
SQL

echo -e "${GREEN}✓ Test database created with 5 rows${NC}"

# Step 2: Create backup
echo -e "${YELLOW}[2/5] Creating backup...${NC}"

BACKUP_FILE="/tmp/test_backup_$(date +%Y%m%d_%H%M%S).sql.gz"
PGPASSWORD="$DB_PASS" pg_dump \
    -h "$DB_HOST" \
    -p "$DB_PORT" \
    -U "$DB_USER" \
    -d "$TEST_DB" | \
    gzip > "$BACKUP_FILE"

BACKUP_SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
echo -e "${GREEN}✓ Backup created ($BACKUP_SIZE)${NC}"

# Step 3: Modify data (to verify restore overwrites)
echo -e "${YELLOW}[3/5] Modifying data...${NC}"

PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" << SQL
INSERT INTO test_table (data) VALUES ('Modified data');
DELETE FROM test_table WHERE id = 1;
SQL

MODIFIED_COUNT=$(PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" -t -c "SELECT COUNT(*) FROM test_table;")
echo -e "${GREEN}✓ Data modified (now $MODIFIED_COUNT rows)${NC}"

# Step 4: Restore backup
echo -e "${YELLOW}[4/5] Restoring backup...${NC}"

PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres << SQL
DROP DATABASE $TEST_DB;
CREATE DATABASE $TEST_DB;
SQL

gunzip -c "$BACKUP_FILE" | \
    PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" > /dev/null 2>&1

echo -e "${GREEN}✓ Backup restored${NC}"

# Step 5: Verify data integrity
echo -e "${YELLOW}[5/5] Verifying data integrity...${NC}"

RESTORED_COUNT=$(PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" -t -c "SELECT COUNT(*) FROM test_table;")
RESTORED_DATA=$(PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB" -t -c "SELECT data FROM test_table ORDER BY id;")

if [[ "$RESTORED_COUNT" == "5" ]]; then
    echo -e "${GREEN}✓ Row count correct ($RESTORED_COUNT rows)${NC}"
else
    echo -e "${RED}✗ Row count mismatch (expected 5, got $RESTORED_COUNT)${NC}"
    exit 1
fi

if echo "$RESTORED_DATA" | grep -q "Test data 1"; then
    echo -e "${GREEN}✓ Data content verified${NC}"
else
    echo -e "${RED}✗ Data content mismatch${NC}"
    exit 1
fi

# Cleanup
echo ""
echo -e "${YELLOW}Cleaning up...${NC}"
PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "DROP DATABASE $TEST_DB;" > /dev/null
rm -f "$BACKUP_FILE"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}All tests passed!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "✓ Backup creation works"
echo "✓ Backup restore works"
echo "✓ Data integrity maintained"
echo ""
echo "Backup/restore system is functional."
echo ""

exit 0
