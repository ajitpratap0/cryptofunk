# TLS/SSL Setup Guide for CryptoFunk Production

This guide explains how to set up TLS/SSL encryption for PostgreSQL and Redis in production environments.

## Overview

In production, all database and cache connections MUST use TLS/SSL encryption to protect:
- Database credentials
- Trading signals and strategies
- API keys and secrets
- User session data
- Performance metrics

**Security Impact**: Without TLS, all database traffic is sent in plaintext and can be intercepted.

## Quick Start

### Development (No TLS)
```bash
docker-compose up -d
```

### Production (TLS Enforced)
```bash
# Generate certificates (see below)
./scripts/generate-certs.sh

# Start with production overrides
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

## Certificate Generation

### 1. PostgreSQL Certificates

Create self-signed certificates for development/staging:

```bash
#!/bin/bash
# scripts/generate-certs.sh

# Create certificate directories
mkdir -p certs/postgres certs/redis

# Generate PostgreSQL CA certificate
openssl req -new -x509 -days 365 -nodes \
  -out certs/postgres/root.crt \
  -keyout certs/postgres/root.key \
  -subj "/CN=CryptoFunk PostgreSQL CA"

# Generate PostgreSQL server certificate
openssl req -new -nodes \
  -out certs/postgres/server.csr \
  -keyout certs/postgres/server.key \
  -subj "/CN=postgres"

# Sign server certificate with CA
openssl x509 -req -in certs/postgres/server.csr \
  -CA certs/postgres/root.crt \
  -CAkey certs/postgres/root.key \
  -CAcreateserial \
  -out certs/postgres/server.crt \
  -days 365

# Set permissions (PostgreSQL requires strict permissions)
chmod 600 certs/postgres/server.key
chmod 644 certs/postgres/server.crt
chmod 644 certs/postgres/root.crt

echo "✓ PostgreSQL certificates generated in certs/postgres/"
```

### 2. Redis Certificates

```bash
# Generate Redis CA certificate
openssl req -new -x509 -days 365 -nodes \
  -out certs/redis/ca.crt \
  -keyout certs/redis/ca.key \
  -subj "/CN=CryptoFunk Redis CA"

# Generate Redis server certificate
openssl req -new -nodes \
  -out certs/redis/redis.csr \
  -keyout certs/redis/redis.key \
  -subj "/CN=redis"

# Sign server certificate with CA
openssl x509 -req -in certs/redis/redis.csr \
  -CA certs/redis/ca.crt \
  -CAkey certs/redis/ca.key \
  -CAcreateserial \
  -out certs/redis/redis.crt \
  -days 365

# Set permissions
chmod 600 certs/redis/redis.key
chmod 644 certs/redis/redis.crt
chmod 644 certs/redis/ca.crt

echo "✓ Redis certificates generated in certs/redis/"
```

### 3. Run Certificate Generation Script

```bash
chmod +x scripts/generate-certs.sh
./scripts/generate-certs.sh
```

## Production Certificate Setup

For production, use certificates from a trusted Certificate Authority (CA):

### Option 1: Let's Encrypt (Free, Automated)

```bash
# Install certbot
sudo apt-get install certbot

# Generate certificates for your domain
sudo certbot certonly --standalone -d postgres.cryptofunk.com -d redis.cryptofunk.com

# Copy certificates to certs directory
cp /etc/letsencrypt/live/postgres.cryptofunk.com/fullchain.pem certs/postgres/server.crt
cp /etc/letsencrypt/live/postgres.cryptofunk.com/privkey.pem certs/postgres/server.key
cp /etc/letsencrypt/live/postgres.cryptofunk.com/chain.pem certs/postgres/root.crt

# Set permissions
chmod 600 certs/postgres/server.key
```

### Option 2: Commercial CA (e.g., DigiCert, GlobalSign)

1. Generate CSR (Certificate Signing Request)
2. Submit CSR to CA
3. Receive signed certificate
4. Place certificates in `certs/postgres/` and `certs/redis/`

### Option 3: Internal PKI (e.g., HashiCorp Vault)

```bash
# Using Vault's PKI secrets engine
vault write pki/issue/cryptofunk-postgres \
  common_name="postgres.cryptofunk.internal" \
  ttl="8760h"

# Export certificates
vault read -field=certificate pki/issue/cryptofunk-postgres > certs/postgres/server.crt
vault read -field=private_key pki/issue/cryptofunk-postgres > certs/postgres/server.key
vault read -field=ca_chain pki/issue/cryptofunk-postgres > certs/postgres/root.crt
```

## Connection String Updates

### PostgreSQL with TLS

**Development (no TLS)**:
```
postgresql://postgres:password@localhost:5432/cryptofunk?sslmode=disable
```

**Production (TLS required)**:
```
postgresql://postgres:password@localhost:5432/cryptofunk?sslmode=require&sslrootcert=/path/to/root.crt
```

**Production (TLS with certificate verification)**:
```
postgresql://postgres:password@postgres.example.com:5432/cryptofunk?sslmode=verify-full&sslrootcert=/certs/root.crt
```

### Redis with TLS

**Development (no TLS)**:
```
redis://localhost:6379
```

**Production (TLS required)**:
```
rediss://:password@localhost:6380?tls_insecure_skip_verify=false&tls_ca_cert=/certs/ca.crt
```

## Environment Variables

Update `.env` for production:

```bash
# PostgreSQL TLS
DATABASE_URL=postgresql://postgres:${POSTGRES_PASSWORD}@postgres:5432/cryptofunk?sslmode=require&sslrootcert=/certs/postgres-ca.crt

# Redis TLS (note: rediss:// instead of redis://)
REDIS_URL=rediss://:${REDIS_PASSWORD}@redis:6380?tls_insecure_skip_verify=false&tls_ca_cert=/certs/redis-ca.crt
REDIS_PASSWORD=your_secure_redis_password

# Vault (handles secrets in production)
VAULT_ENABLED=true
VAULT_ADDR=https://vault.example.com:8200
VAULT_AUTH_METHOD=kubernetes
```

## Kubernetes TLS Setup

For Kubernetes deployments, use Secrets to store certificates:

```bash
# Create TLS secrets
kubectl create secret generic postgres-tls \
  --from-file=server.crt=certs/postgres/server.crt \
  --from-file=server.key=certs/postgres/server.key \
  --from-file=ca.crt=certs/postgres/root.crt \
  -n cryptofunk

kubectl create secret generic redis-tls \
  --from-file=redis.crt=certs/redis/redis.crt \
  --from-file=redis.key=certs/redis/redis.key \
  --from-file=ca.crt=certs/redis/ca.crt \
  -n cryptofunk
```

Update deployments to mount secrets:

```yaml
# deployments/k8s/base/deployment-postgres.yaml
spec:
  template:
    spec:
      volumes:
      - name: postgres-tls
        secret:
          secretName: postgres-tls
          defaultMode: 0600
      containers:
      - name: postgres
        volumeMounts:
        - name: postgres-tls
          mountPath: /var/lib/postgresql/server.crt
          subPath: server.crt
          readOnly: true
        - name: postgres-tls
          mountPath: /var/lib/postgresql/server.key
          subPath: server.key
          readOnly: true
        - name: postgres-tls
          mountPath: /var/lib/postgresql/root.crt
          subPath: ca.crt
          readOnly: true
```

## Verification

### Test PostgreSQL TLS Connection

```bash
# Connect with psql (should show "SSL connection")
PGSSLMODE=require PGSSLROOTCERT=certs/postgres/root.crt \
  psql "postgresql://postgres:password@localhost:5432/cryptofunk"

# Inside psql, check SSL status
\conninfo
# Should show: SSL connection (protocol: TLSv1.3, cipher: TLS_AES_256_GCM_SHA384, bits: 256)
```

### Test Redis TLS Connection

```bash
# Connect with redis-cli
redis-cli --tls \
  --cert certs/redis/redis.crt \
  --key certs/redis/redis.key \
  --cacert certs/redis/ca.crt \
  -h localhost -p 6380

# Inside redis-cli
INFO server
# Should show connection is using TLS
```

### Test Application TLS Connection

```bash
# Start services with production config
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Check orchestrator logs for TLS confirmation
docker-compose logs orchestrator | grep -i ssl
# Should show: "Connected to database with SSL"

# Check health endpoint
curl http://localhost:8081/health
# Should show: "status": "healthy"
```

## Common Issues

### Issue 1: Certificate Permissions

**Error**: `private key file has group or world access`

**Solution**:
```bash
chmod 600 certs/postgres/server.key
chmod 600 certs/redis/redis.key
```

### Issue 2: Certificate Hostname Mismatch

**Error**: `certificate is valid for postgres, not postgres.example.com`

**Solution**: Regenerate certificate with correct CN (Common Name) or use Subject Alternative Names (SANs):

```bash
# Create config file with SANs
cat > postgres.cnf <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
CN = postgres

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = postgres
DNS.2 = postgres.cryptofunk.com
DNS.3 = postgres.cryptofunk.internal
IP.1 = 10.0.0.1
EOF

# Generate certificate with SANs
openssl req -new -nodes \
  -out certs/postgres/server.csr \
  -keyout certs/postgres/server.key \
  -config postgres.cnf

openssl x509 -req -in certs/postgres/server.csr \
  -CA certs/postgres/root.crt \
  -CAkey certs/postgres/root.key \
  -CAcreateserial \
  -out certs/postgres/server.crt \
  -days 365 \
  -extensions v3_req \
  -extfile postgres.cnf
```

### Issue 3: Connection Refused

**Error**: `connection refused on port 6380`

**Solution**: Verify Redis is listening on TLS port:

```bash
# Check Redis logs
docker-compose logs redis | grep -i tls
# Should show: "Ready to accept connections on port 6380 (TLS)"

# Check port binding
netstat -tln | grep 6380
```

### Issue 4: Certificate Expiry

**Error**: `certificate has expired`

**Solution**: Regenerate certificates (see scripts above) or use Vault for automatic rotation.

## Certificate Rotation

### Manual Rotation

```bash
# 1. Generate new certificates
./scripts/generate-certs.sh

# 2. Restart services one at a time (zero-downtime)
docker-compose restart postgres
docker-compose restart redis
docker-compose restart orchestrator
# ... restart other services
```

### Automated Rotation with Vault

See `docs/SECRET_ROTATION.md` for Vault-based certificate rotation procedures.

## Security Best Practices

1. **Never commit certificates to git**: Add `certs/` to `.gitignore`
2. **Use strong key sizes**: Minimum 2048-bit RSA or 256-bit ECDSA
3. **Set expiry dates**: 90 days for development, 365 days for production
4. **Automate rotation**: Use Vault or cert-manager for automatic renewal
5. **Verify certificates**: Always use `sslmode=verify-full` in production
6. **Restrict permissions**: Certificates 644, private keys 600
7. **Monitor expiry**: Set up alerts 30 days before expiration

## Production Checklist

- [ ] Generate production certificates (not self-signed)
- [ ] Store certificates in Kubernetes Secrets or Vault
- [ ] Set `sslmode=require` or `sslmode=verify-full` in all connection strings
- [ ] Enable TLS for Redis (use `rediss://` protocol)
- [ ] Set strict certificate permissions (600 for keys, 644 for certs)
- [ ] Configure certificate rotation (Vault or cert-manager)
- [ ] Test TLS connections (psql, redis-cli, application health checks)
- [ ] Monitor certificate expiry (Prometheus alerts)
- [ ] Document certificate renewal procedures
- [ ] Add TLS verification to CI/CD pipeline

## References

- [PostgreSQL SSL Documentation](https://www.postgresql.org/docs/current/ssl-tcp.html)
- [Redis TLS Documentation](https://redis.io/docs/manual/security/encryption/)
- [HashiCorp Vault PKI Secrets Engine](https://developer.hashicorp.com/vault/docs/secrets/pki)
- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
- [OpenSSL Cookbook](https://www.feistyduck.com/library/openssl-cookbook/)

---

**Last Updated**: 2025-01-15
**Security Level**: P0 (Production Critical)
