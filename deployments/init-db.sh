#!/bin/bash
set -e

# Initialize PostgreSQL database with required extensions
echo "Initializing CryptoFunk database with extensions..."

# Enable TimescaleDB extension
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Enable TimescaleDB for time-series data
    CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

    -- Enable pgvector for LLM embeddings
    CREATE EXTENSION IF NOT EXISTS vector;

    -- Enable additional useful extensions
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
    CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
EOSQL

echo "Database extensions installed successfully!"
echo "- timescaledb: Enabled for time-series data"
echo "- vector: Enabled for LLM embeddings"
echo "- uuid-ossp: Enabled for UUID generation"
echo "- pg_stat_statements: Enabled for query performance analysis"
