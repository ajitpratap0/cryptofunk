#!/bin/bash
# CryptoFunk Vault Initialization Script
# This script initializes Vault with secrets for local development
#
# ============================================================================
# SECURITY WARNING - LOCAL DEVELOPMENT ONLY
# ============================================================================
# This script contains HARDCODED CREDENTIALS intended ONLY for local
# development with Docker Compose. These credentials MUST NOT be used in
# production environments.
#
# For production deployment:
#   1. Use Vault AppRole or Kubernetes auth instead of static tokens
#   2. Generate unique, strong credentials for each service
#   3. Use external secrets management (Vault agent, external-secrets, etc.)
#   4. Enable Vault audit logging
#   5. Use TLS for Vault communication
#   6. Implement proper secret rotation
#
# The default dev token below is predictable and should NEVER be used
# outside of local development.
# ============================================================================

set -e

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="${VAULT_DEV_TOKEN:-cryptofunk-dev-token}"

# Warn if running in non-development context
if [[ -z "${CRYPTOFUNK_DEV_MODE}" ]]; then
    echo "============================================================"
    echo "WARNING: This script uses hardcoded development credentials."
    echo "Set CRYPTOFUNK_DEV_MODE=1 to suppress this warning."
    echo "For production, use proper Vault initialization procedures."
    echo "============================================================"
    echo ""
fi

echo "=== CryptoFunk Vault Initialization ==="
echo "Vault Address: $VAULT_ADDR"
echo ""

# Wait for Vault to be ready
echo "Waiting for Vault to be ready..."
until curl -s "$VAULT_ADDR/v1/sys/health" > /dev/null 2>&1; do
    sleep 1
done
echo "Vault is ready!"

# Enable KV secrets engine v2 at cryptofunk/
echo ""
echo "Enabling KV v2 secrets engine at cryptofunk/..."
curl -s -X POST \
    -H "X-Vault-Token: $VAULT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"type": "kv", "options": {"version": "2"}}' \
    "$VAULT_ADDR/v1/sys/mounts/cryptofunk" || echo "Secret engine may already exist"

# Store database credentials
echo ""
echo "Storing database credentials..."
curl -s -X POST \
    -H "X-Vault-Token: $VAULT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "data": {
            "host": "localhost",
            "port": "5432",
            "database": "cryptofunk",
            "username": "postgres",
            "password": "test_password_for_local_dev_123",
            "sslmode": "disable"
        }
    }' \
    "$VAULT_ADDR/v1/cryptofunk/data/database"

# Store Redis credentials
echo "Storing Redis credentials..."
curl -s -X POST \
    -H "X-Vault-Token: $VAULT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "data": {
            "host": "localhost",
            "port": "6379",
            "password": ""
        }
    }' \
    "$VAULT_ADDR/v1/cryptofunk/data/redis"

# Store NATS credentials
echo "Storing NATS credentials..."
curl -s -X POST \
    -H "X-Vault-Token: $VAULT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "data": {
            "url": "nats://localhost:4222"
        }
    }' \
    "$VAULT_ADDR/v1/cryptofunk/data/nats"

# Store LLM API keys (placeholders for local dev)
echo "Storing LLM API keys..."
curl -s -X POST \
    -H "X-Vault-Token: $VAULT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "data": {
            "anthropic_api_key": "sk-ant-test-placeholder",
            "openai_api_key": "sk-test-placeholder",
            "gemini_api_key": "test-placeholder"
        }
    }' \
    "$VAULT_ADDR/v1/cryptofunk/data/llm"

# Store exchange API keys (placeholders for paper trading)
echo "Storing exchange API keys..."
curl -s -X POST \
    -H "X-Vault-Token: $VAULT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "data": {
            "binance_api_key": "test_key",
            "binance_api_secret": "test_secret",
            "coingecko_api_key": "test_key"
        }
    }' \
    "$VAULT_ADDR/v1/cryptofunk/data/exchanges"

# Store JWT secret
echo "Storing JWT secret..."
curl -s -X POST \
    -H "X-Vault-Token: $VAULT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "data": {
            "secret": "test_jwt_secret_for_local_dev_only_123"
        }
    }' \
    "$VAULT_ADDR/v1/cryptofunk/data/jwt"

# Store Grafana credentials
echo "Storing Grafana credentials..."
curl -s -X POST \
    -H "X-Vault-Token: $VAULT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "data": {
            "admin_user": "admin",
            "admin_password": "admin_test_password_123"
        }
    }' \
    "$VAULT_ADDR/v1/cryptofunk/data/grafana"

echo ""
echo "=== Vault initialization complete! ==="
echo ""
echo "Secrets stored:"
echo "  - cryptofunk/data/database"
echo "  - cryptofunk/data/redis"
echo "  - cryptofunk/data/nats"
echo "  - cryptofunk/data/llm"
echo "  - cryptofunk/data/exchanges"
echo "  - cryptofunk/data/jwt"
echo "  - cryptofunk/data/grafana"
echo ""
echo "To read a secret:"
echo "  curl -s -H 'X-Vault-Token: $VAULT_TOKEN' $VAULT_ADDR/v1/cryptofunk/data/database | jq"
echo ""
echo "Vault UI available at: $VAULT_ADDR/ui"
echo "Token: $VAULT_TOKEN"
