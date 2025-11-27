# Secret Rotation Procedures

This document outlines procedures for rotating secrets in the CryptoFunk trading system using HashiCorp Vault.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Secret Types](#secret-types)
- [Rotation Schedules](#rotation-schedules)
- [Rotation Procedures](#rotation-procedures)
  - [Database Credentials](#database-credentials)
  - [Redis Credentials](#redis-credentials)
  - [Exchange API Keys](#exchange-api-keys)
  - [LLM API Keys](#llm-api-keys)
- [Emergency Rotation](#emergency-rotation)
- [Vault Setup](#vault-setup)
- [Verification](#verification)
- [CryptoFunk API Key Management](#cryptofunk-api-key-management)

## Overview

CryptoFunk uses HashiCorp Vault for secure secrets management. All production secrets should be stored in Vault and never committed to version control or stored in plain text.

**Security Principle**: Regular secret rotation reduces the window of vulnerability if credentials are compromised.

## Prerequisites

- HashiCorp Vault deployed and accessible
- Vault CLI installed (`brew install vault` or see [Vault installation](https://www.vaultproject.io/downloads))
- Vault authentication credentials (token, Kubernetes service account, or AppRole)
- Appropriate Vault policies for reading/writing secrets

## Secret Types

CryptoFunk manages the following secret types:

1. **Database Credentials** (PostgreSQL)
   - User: `postgres`
   - Password: Rotated quarterly
   - Path: `secret/data/cryptofunk/production/database`

2. **Redis Credentials**
   - Password: Rotated quarterly
   - Path: `secret/data/cryptofunk/production/redis`

3. **Exchange API Keys** (Binance, etc.)
   - API Key and Secret Key
   - Rotated monthly (or immediately if compromised)
   - Path: `secret/data/cryptofunk/production/exchanges/<exchange-name>`

4. **LLM API Keys** (Anthropic, OpenAI, Gemini)
   - Provider-specific API keys
   - Rotated monthly
   - Path: `secret/data/cryptofunk/production/llm`

5. **CryptoFunk API Keys** (User Authentication)
   - SHA-256 hashed keys stored in PostgreSQL
   - Used for REST API authentication
   - Managed via database functions
   - See [CryptoFunk API Key Management](#cryptofunk-api-key-management)

## Rotation Schedules

| Secret Type | Rotation Frequency | Reason |
|-------------|-------------------|--------|
| Database Password | Quarterly (90 days) | Low exposure risk, high rotation cost |
| Redis Password | Quarterly (90 days) | Low exposure risk |
| Exchange API Keys | Monthly (30 days) | High security risk, financial implications |
| LLM API Keys | Monthly (30 days) | Moderate risk, usage tracking |
| All Secrets | Immediately | If compromise suspected |

## Rotation Procedures

### Database Credentials

**Downtime**: ~30 seconds (connection pool refresh)

**Steps**:

1. **Generate new password**:
   ```bash
   # Generate a strong password (32 characters, alphanumeric + special)
   NEW_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
   echo "New password: $NEW_PASSWORD"
   ```

2. **Update PostgreSQL**:
   ```bash
   # Connect to PostgreSQL
   kubectl exec -it -n cryptofunk deployment/postgres -- \
     psql -U postgres -c "ALTER USER postgres WITH PASSWORD '$NEW_PASSWORD';"
   ```

3. **Update Vault**:
   ```bash
   # Authenticate to Vault
   vault login

   # Write new password to Vault
   vault kv put secret/cryptofunk/production/database \
     password="$NEW_PASSWORD" \
     user="postgres"
   ```

4. **Rolling restart applications**:
   ```bash
   # Restart orchestrator (will reload secrets from Vault)
   kubectl rollout restart deployment/orchestrator -n cryptofunk

   # Restart API server
   kubectl rollout restart deployment/api -n cryptofunk

   # Restart MCP servers
   kubectl rollout restart deployment/market-data-server -n cryptofunk
   kubectl rollout restart deployment/technical-indicators-server -n cryptofunk
   kubectl rollout restart deployment/risk-analyzer-server -n cryptofunk
   kubectl rollout restart deployment/order-executor-server -n cryptofunk

   # Restart agents
   kubectl rollout restart deployment/technical-agent -n cryptofunk
   kubectl rollout restart deployment/orderbook-agent -n cryptofunk
   kubectl rollout restart deployment/sentiment-agent -n cryptofunk
   kubectl rollout restart deployment/trend-agent -n cryptofunk
   kubectl rollout restart deployment/reversion-agent -n cryptofunk
   kubectl rollout restart deployment/risk-agent -n cryptofunk
   ```

5. **Verify connectivity**:
   ```bash
   # Check orchestrator logs for successful database connection
   kubectl logs -f deployment/orchestrator -n cryptofunk | grep -i database

   # Verify health endpoint
   curl http://orchestrator-service:8080/health
   ```

### Redis Credentials

**Downtime**: ~15 seconds (cache refresh)

**Steps**:

1. **Generate new password**:
   ```bash
   NEW_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
   ```

2. **Update Redis configuration**:
   ```bash
   # For Docker Compose
   docker-compose exec redis redis-cli CONFIG SET requirepass "$NEW_PASSWORD"

   # For Kubernetes
   kubectl exec -it -n cryptofunk deployment/redis -- \
     redis-cli CONFIG SET requirepass "$NEW_PASSWORD"
   ```

3. **Update Vault**:
   ```bash
   vault kv put secret/cryptofunk/production/redis \
     password="$NEW_PASSWORD"
   ```

4. **Rolling restart applications** (same as database rotation above)

5. **Verify**:
   ```bash
   # Test Redis connection
   kubectl exec -it -n cryptofunk deployment/redis -- \
     redis-cli -a "$NEW_PASSWORD" PING
   ```

### Exchange API Keys

**Downtime**: None (key rotation can be done without downtime)

**Steps**:

1. **Generate new API keys on exchange**:
   - Log into Binance (or other exchange)
   - Navigate to API Management
   - Create new API key with same permissions as old key
   - **IMPORTANT**: Restrict IP addresses to your cluster IPs
   - Copy API Key and Secret Key

2. **Update Vault**:
   ```bash
   # For Binance
   vault kv put secret/cryptofunk/production/exchanges/binance \
     api_key="<NEW_API_KEY>" \
     secret_key="<NEW_SECRET_KEY>"
   ```

3. **Rolling restart order executor** (to load new keys):
   ```bash
   kubectl rollout restart deployment/order-executor-server -n cryptofunk
   ```

4. **Verify connectivity**:
   ```bash
   # Check order executor logs
   kubectl logs -f deployment/order-executor-server -n cryptofunk

   # Test with --verify-keys flag
   kubectl exec -it deployment/orchestrator -n cryptofunk -- \
     /app/orchestrator --verify-keys
   ```

5. **Delete old API key** (after verification):
   - Log into exchange
   - Delete the old API key
   - **IMPORTANT**: Only delete after confirming new key works!

### LLM API Keys

**Downtime**: None (Bifrost handles key rotation gracefully)

**Steps**:

1. **Generate new API keys from providers**:
   - **Anthropic**: https://console.anthropic.com/settings/keys
   - **OpenAI**: https://platform.openai.com/api-keys
   - **Gemini**: https://makersuite.google.com/app/apikey

2. **Update Vault**:
   ```bash
   vault kv put secret/cryptofunk/production/llm \
     anthropic_api_key="<NEW_ANTHROPIC_KEY>" \
     openai_api_key="<NEW_OPENAI_KEY>" \
     gemini_api_key="<NEW_GEMINI_KEY>"
   ```

3. **Restart Bifrost**:
   ```bash
   kubectl rollout restart deployment/bifrost -n cryptofunk
   ```

4. **Restart agents** (to reload LLM keys):
   ```bash
   kubectl rollout restart deployment/orchestrator -n cryptofunk
   kubectl rollout restart deployment/technical-agent -n cryptofunk
   kubectl rollout restart deployment/orderbook-agent -n cryptofunk
   kubectl rollout restart deployment/sentiment-agent -n cryptofunk
   kubectl rollout restart deployment/trend-agent -n cryptofunk
   kubectl rollout restart deployment/reversion-agent -n cryptofunk
   kubectl rollout restart deployment/risk-agent -n cryptofunk
   ```

5. **Verify**:
   ```bash
   # Test LLM connectivity
   kubectl logs -f deployment/bifrost -n cryptofunk

   # Verify agents can make LLM calls
   kubectl logs -f deployment/technical-agent -n cryptofunk | grep -i "llm"
   ```

## Emergency Rotation

If you suspect credentials have been compromised:

1. **Immediate Actions**:
   ```bash
   # STEP 1: Disable compromised credentials immediately
   # For exchange keys - disable on exchange portal immediately
   # For database - change password immediately

   # STEP 2: Stop all trading activity
   kubectl scale deployment/orchestrator --replicas=0 -n cryptofunk

   # STEP 3: Review logs for suspicious activity
   kubectl logs deployment/orchestrator -n cryptofunk --since=24h > logs.txt
   kubectl logs deployment/order-executor-server -n cryptofunk --since=24h >> logs.txt

   # STEP 4: Check for unauthorized trades
   kubectl exec -it -n cryptofunk deployment/postgres -- \
     psql -U postgres -d cryptofunk -c \
     "SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '24 hours' ORDER BY created_at DESC;"
   ```

2. **Rotate all affected credentials** following procedures above

3. **Investigate root cause**:
   - Check application logs for anomalies
   - Review access logs
   - Scan for security vulnerabilities
   - Check if secrets were accidentally committed to version control

4. **Resume operations**:
   ```bash
   # After rotation and investigation
   kubectl scale deployment/orchestrator --replicas=1 -n cryptofunk
   ```

5. **Document incident** in incident log and update security procedures

## Vault Setup

### Initial Vault Configuration

1. **Enable KV secrets engine** (v2):
   ```bash
   vault secrets enable -path=secret kv-v2
   ```

2. **Create CryptoFunk secrets path**:
   ```bash
   vault kv put secret/cryptofunk/production/database \
     password="<initial-password>" \
     user="postgres"

   vault kv put secret/cryptofunk/production/redis \
     password="<initial-password>"

   vault kv put secret/cryptofunk/production/exchanges/binance \
     api_key="<binance-api-key>" \
     secret_key="<binance-secret-key>"

   vault kv put secret/cryptofunk/production/llm \
     anthropic_api_key="<anthropic-key>" \
     openai_api_key="<openai-key>" \
     gemini_api_key="<gemini-key>"
   ```

3. **Create Vault policy for CryptoFunk**:
   ```bash
   # Create policy file
   cat > cryptofunk-policy.hcl <<EOF
   # Allow reading all CryptoFunk secrets
   path "secret/data/cryptofunk/production/*" {
     capabilities = ["read", "list"]
   }

   # Allow reading secret metadata
   path "secret/metadata/cryptofunk/production/*" {
     capabilities = ["read", "list"]
   }
   EOF

   # Apply policy
   vault policy write cryptofunk cryptofunk-policy.hcl
   ```

4. **Configure Kubernetes authentication**:
   ```bash
   # Enable Kubernetes auth
   vault auth enable kubernetes

   # Configure Kubernetes auth
   vault write auth/kubernetes/config \
     kubernetes_host="https://kubernetes.default.svc:443" \
     kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
     token_reviewer_jwt=@/var/run/secrets/kubernetes.io/serviceaccount/token

   # Create role for CryptoFunk
   vault write auth/kubernetes/role/cryptofunk \
     bound_service_account_names=cryptofunk-vault \
     bound_service_account_namespaces=cryptofunk \
     policies=cryptofunk \
     ttl=24h
   ```

### Environment Variables

Set these environment variables in Kubernetes deployments:

```yaml
env:
  - name: VAULT_ENABLED
    value: "true"
  - name: VAULT_ADDR
    value: "https://vault.example.com:8200"
  - name: VAULT_AUTH_METHOD
    value: "kubernetes"
  - name: VAULT_MOUNT_PATH
    value: "secret"
  - name: VAULT_SECRET_PATH
    value: "cryptofunk/production"
  - name: VAULT_K8S_ROLE
    value: "cryptofunk"
```

## Verification

### Verify Vault Integration

```bash
# Check if Vault is accessible
vault status

# List secrets
vault kv list secret/cryptofunk/production

# Read a secret (without revealing value)
vault kv get secret/cryptofunk/production/database

# Test Kubernetes authentication
kubectl exec -it deployment/orchestrator -n cryptofunk -- \
  env | grep VAULT
```

### Verify Application Loads Secrets

```bash
# Check orchestrator logs for Vault messages
kubectl logs deployment/orchestrator -n cryptofunk | grep -i vault

# Should see messages like:
# "Vault client initialized successfully"
# "✓ Loaded database password from Vault"
# "✓ Loaded exchange API keys from Vault"
```

### Test Secret Rotation

```bash
# 1. Note current secret version
vault kv metadata get secret/cryptofunk/production/database

# 2. Rotate secret
vault kv put secret/cryptofunk/production/database \
  password="new-test-password" \
  user="postgres"

# 3. Restart application
kubectl rollout restart deployment/orchestrator -n cryptofunk

# 4. Verify new secret is loaded
kubectl logs deployment/orchestrator -n cryptofunk | tail -20
```

## Best Practices

1. **Never commit secrets to version control**
   - Use `.gitignore` for sensitive files
   - Use Vault for all production secrets
   - Use environment variables for development

2. **Use strong, unique passwords**
   - Minimum 32 characters for database/Redis
   - Use `openssl rand -base64 32` for generation
   - Never reuse passwords across systems

3. **Restrict Vault access**
   - Use least-privilege policies
   - Audit Vault access logs regularly
   - Rotate Vault tokens periodically

4. **Automate rotation**
   - Set calendar reminders for scheduled rotations
   - Consider Vault's dynamic secrets for auto-rotation
   - Document all rotations in change log

5. **Test rotation procedures**
   - Practice rotations in staging environment
   - Verify applications handle secret updates gracefully
   - Have rollback plan ready

6. **Monitor for suspicious activity**
   - Alert on failed authentication attempts
   - Monitor unusual API usage patterns
   - Track secret access in Vault audit logs

## Troubleshooting

### Vault Connection Errors

```bash
# Check Vault status
kubectl exec -it deployment/orchestrator -n cryptofunk -- \
  curl -v https://vault.example.com:8200/v1/sys/health

# Verify service account token
kubectl exec -it deployment/orchestrator -n cryptofunk -- \
  cat /var/run/secrets/kubernetes.io/serviceaccount/token
```

### Secret Not Found Errors

```bash
# List all secrets
vault kv list secret/cryptofunk/production

# Check specific path
vault kv get secret/cryptofunk/production/database

# Verify path format (KV v2 uses /data/ in path)
# Correct: secret/data/cryptofunk/production/database
# Incorrect: secret/cryptofunk/production/database
```

### Authentication Errors

```bash
# Check Kubernetes auth configuration
vault read auth/kubernetes/config

# Check role configuration
vault read auth/kubernetes/role/cryptofunk

# Verify service account exists
kubectl get serviceaccount cryptofunk-vault -n cryptofunk
```

## Additional Resources

- [HashiCorp Vault Documentation](https://www.vaultproject.io/docs)
- [Kubernetes Auth Method](https://www.vaultproject.io/docs/auth/kubernetes)
- [KV Secrets Engine](https://www.vaultproject.io/docs/secrets/kv/kv-v2)
- [Vault Best Practices](https://learn.hashicorp.com/tutorials/vault/production-hardening)

## CryptoFunk API Key Management

CryptoFunk uses API key authentication for the REST API. This section covers how to create, rotate, and revoke API keys for user authentication.

### Overview

API keys are used to authenticate requests to the CryptoFunk REST API. They provide:
- SHA-256 hashed storage (raw keys are never stored)
- Permission-based authorization
- Expiration support
- Usage tracking (last_used_at)
- Revocation capability

### Prerequisites

1. Run migration `009_api_keys.sql` to create the `api_keys` table:
   ```bash
   task db-migrate
   ```

2. Enable authentication in `config.yaml`:
   ```yaml
   api:
     auth:
       enabled: true
       header_name: "X-API-Key"
       require_https: true
   ```

### Creating API Keys

Use the `create_api_key()` PostgreSQL function to create new keys:

```bash
# Connect to PostgreSQL
task db-shell

# Create a new API key with specific permissions
SELECT create_api_key(
  'My Service Key',           -- name
  'system',                   -- user_id
  ARRAY['read', 'write'],     -- permissions
  NULL                        -- expires_at (NULL = never expires)
);

# Create an API key that expires in 90 days
SELECT create_api_key(
  'Temporary Key',
  'admin',
  ARRAY['read'],
  NOW() + INTERVAL '90 days'
);

# Create an admin key with full permissions
SELECT create_api_key(
  'Admin Key',
  'admin',
  ARRAY['admin'],  -- 'admin' or '*' grants all permissions
  NULL
);
```

The function returns the raw API key (e.g., `cfk_abc123...`). **Store this securely - it cannot be recovered!**

### Using API Keys

Include the API key in requests using either method:

```bash
# Method 1: X-API-Key header (preferred)
curl -H "X-API-Key: cfk_abc123..." \
  http://localhost:8080/api/v1/decisions

# Method 2: Authorization Bearer header
curl -H "Authorization: Bearer cfk_abc123..." \
  http://localhost:8080/api/v1/decisions
```

### API Key Rotation Workflow

**Recommended rotation frequency**: Every 90 days, or immediately if compromised.

**Zero-downtime rotation steps**:

1. **Create new API key** (before expiring old one):
   ```sql
   SELECT create_api_key(
     'My Service Key v2',
     'system',
     ARRAY['read', 'write'],
     NULL
   );
   ```

2. **Update applications** to use the new key.

3. **Verify new key works**:
   ```bash
   curl -H "X-API-Key: <new-key>" \
     http://localhost:8080/api/v1/health
   ```

4. **Revoke old key**:
   ```sql
   -- Find old key by name
   SELECT id, name, created_at FROM api_keys
   WHERE name LIKE 'My Service Key%'
   ORDER BY created_at DESC;

   -- Revoke the old key
   UPDATE api_keys SET revoked = true WHERE id = '<old-key-id>';
   ```

### Revoking API Keys

Revoke keys immediately if compromised:

```sql
-- Revoke by key ID
UPDATE api_keys SET revoked = true WHERE id = '<key-id>';

-- Revoke all keys for a user
UPDATE api_keys SET revoked = true WHERE user_id = '<user-id>';

-- Verify revocation
SELECT id, name, revoked FROM api_keys WHERE id = '<key-id>';
```

Revoked keys are immediately rejected by the auth middleware.

### Viewing API Keys

List and monitor API keys (the hash is never displayed):

```sql
-- List all active keys
SELECT
  id,
  name,
  user_id,
  permissions,
  last_used_at,
  created_at,
  expires_at,
  revoked
FROM api_keys
WHERE revoked = false
ORDER BY created_at DESC;

-- Find unused keys (potential security risk)
SELECT id, name, user_id, created_at
FROM api_keys
WHERE last_used_at IS NULL
  AND created_at < NOW() - INTERVAL '30 days';

-- Find expired keys (cleanup candidates)
SELECT id, name, user_id, expires_at
FROM api_keys
WHERE expires_at < NOW();
```

### Permission System

Available permissions:
- `read` - Read-only access to all GET endpoints
- `write` - Create/update/delete operations
- `admin` - Full access (equivalent to `*`)
- `*` - Wildcard, grants all permissions
- Custom permissions can be checked with `RequirePermission("custom")` middleware

Permission examples:
```sql
-- Read-only dashboard access
SELECT create_api_key('Dashboard Reader', 'user1', ARRAY['read'], NULL);

-- Trading bot with write access
SELECT create_api_key('Trading Bot', 'bot1', ARRAY['read', 'write'], NULL);

-- Admin access
SELECT create_api_key('Admin Console', 'admin', ARRAY['admin'], NULL);
```

### Security Best Practices

1. **Never log or expose raw API keys** - Only the SHA-256 hash is stored
2. **Use HTTPS in production** - Set `api.auth.require_https: true`
3. **Set expiration dates** for temporary access
4. **Audit key usage** - Monitor `last_used_at` for anomalies
5. **Revoke unused keys** - Remove keys that haven't been used in 90+ days
6. **Use minimal permissions** - Grant only necessary permissions
7. **Rotate regularly** - At least every 90 days for long-lived keys

### Cleanup Old Keys

Run periodically to clean up expired and revoked keys:

```sql
-- Delete keys that have been revoked for more than 30 days
DELETE FROM api_keys
WHERE revoked = true
  AND updated_at < NOW() - INTERVAL '30 days';

-- Delete expired keys older than 30 days
DELETE FROM api_keys
WHERE expires_at < NOW() - INTERVAL '30 days';
```

### Emergency Key Revocation

If a key is compromised:

```bash
# 1. Immediately revoke the key
psql -d cryptofunk -c "UPDATE api_keys SET revoked = true WHERE key_hash = '$(echo -n '<raw-key>' | sha256sum | cut -d' ' -f1)';"

# 2. Check audit logs for unauthorized access
grep '<key-id>' /var/log/cryptofunk/audit.log

# 3. Review recent API activity
psql -d cryptofunk -c "SELECT * FROM audit_logs WHERE api_key_id = '<key-id>' ORDER BY created_at DESC LIMIT 100;"

# 4. Create new key if needed
psql -d cryptofunk -c "SELECT create_api_key('Replacement Key', 'user', ARRAY['read', 'write'], NULL);"
```

## Support

For questions or issues with secret rotation:
- Create an issue in GitHub: [cryptofunk/issues](https://github.com/yourusername/cryptofunk/issues)
- Consult `docs/ALERT_RUNBOOK.md` for operational procedures
- Review `CLAUDE.md` for architecture details
