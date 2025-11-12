#!/usr/bin/env bash
#
# reset-db.sh - Reset the database to a clean state
#
# This script:
# 1. Drops the existing database
# 2. Creates a fresh database
# 3. Runs all migrations
# 4. (Optional) Seeds with test data
#
# Usage:
#   ./scripts/dev/reset-db.sh             # Reset and run migrations
#   ./scripts/dev/reset-db.sh --seed      # Reset, migrate, and seed test data
#
# WARNING: This will DELETE ALL DATA in the database!

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Database configuration from environment or defaults
DB_HOST="${DATABASE_HOST:-localhost}"
DB_PORT="${DATABASE_PORT:-5432}"
DB_USER="${DATABASE_USER:-postgres}"
DB_PASS="${DATABASE_PASSWORD:-postgres}"
DB_NAME="${DATABASE_NAME:-cryptofunk}"

# Parse arguments
SEED_DATA=false
if [[ "${1:-}" == "--seed" ]]; then
    SEED_DATA=true
fi

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}CryptoFunk Database Reset${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo -e "${RED}WARNING: This will DELETE ALL DATA!${NC}"
echo -e "Database: ${DB_NAME}@${DB_HOST}:${DB_PORT}"
echo ""
read -p "Are you sure you want to continue? (yes/no): " -r
echo ""

if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo -e "${YELLOW}Aborted.${NC}"
    exit 0
fi

# Step 1: Drop existing database
echo -e "${YELLOW}[1/4] Dropping existing database...${NC}"
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;" 2>/dev/null || {
    echo -e "${RED}Failed to drop database. Is PostgreSQL running?${NC}"
    echo "Try: docker-compose up -d postgres"
    exit 1
}
echo -e "${GREEN}✓ Database dropped${NC}"

# Step 2: Create fresh database
echo -e "${YELLOW}[2/4] Creating fresh database...${NC}"
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d postgres -c "CREATE DATABASE $DB_NAME;"
echo -e "${GREEN}✓ Database created${NC}"

# Step 3: Enable extensions
echo -e "${YELLOW}[3/4] Enabling extensions (TimescaleDB, pgvector)...${NC}"
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME << SQL
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
SQL
echo -e "${GREEN}✓ Extensions enabled${NC}"

# Step 4: Run migrations
echo -e "${YELLOW}[4/4] Running migrations...${NC}"
if [[ -f "./bin/migrate" ]]; then
    ./bin/migrate up
elif [[ -f "./cmd/migrate/main.go" ]]; then
    go run ./cmd/migrate up
else
    echo -e "${YELLOW}No migration tool found. Applying migrations manually...${NC}"
    for migration in migrations/*.sql; do
        if [[ -f "$migration" ]]; then
            echo "  Applying: $migration"
            PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$migration"
        fi
    done
fi
echo -e "${GREEN}✓ Migrations applied${NC}"

# Optional: Seed test data
if [[ "$SEED_DATA" == true ]]; then
    echo -e "${YELLOW}[SEED] Generating test data...${NC}"
    if [[ -f "./scripts/dev/generate-test-data.sh" ]]; then
        ./scripts/dev/generate-test-data.sh
    else
        echo -e "${YELLOW}  No seed script found, skipping...${NC}"
    fi
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Database reset complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Next steps:"
echo "  1. Start the system: docker-compose up -d"
echo "  2. Run orchestrator: ./bin/orchestrator"
echo "  3. Check database: psql -h $DB_HOST -U $DB_USER -d $DB_NAME"
echo ""
