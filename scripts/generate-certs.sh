#!/bin/bash

# Certificate generation script for CryptoFunk
# Generates self-signed TLS certificates for PostgreSQL and Redis
# For production, use certificates from a trusted CA (Let's Encrypt, DigiCert, etc.)

set -e

echo "=========================================="
echo "CryptoFunk Certificate Generation"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Create certificate directories
echo "Creating certificate directories..."
mkdir -p certs/postgres certs/redis

# ==============================================
# PostgreSQL Certificates
# ==============================================

echo ""
echo "Generating PostgreSQL certificates..."

# Generate PostgreSQL CA certificate
openssl req -new -x509 -days 365 -nodes \
  -out certs/postgres/root.crt \
  -keyout certs/postgres/root.key \
  -subj "/C=US/ST=California/L=San Francisco/O=CryptoFunk/OU=Database/CN=CryptoFunk PostgreSQL CA" \
  2>/dev/null

# Create OpenSSL config for SANs (Subject Alternative Names)
cat > certs/postgres/postgres.cnf <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
CN = postgres

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = postgres
DNS.2 = localhost
DNS.3 = cryptofunk-postgres
DNS.4 = postgres.cryptofunk.svc.cluster.local
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

# Generate PostgreSQL server certificate
openssl req -new -nodes \
  -out certs/postgres/server.csr \
  -keyout certs/postgres/server.key \
  -config certs/postgres/postgres.cnf \
  -subj "/C=US/ST=California/L=San Francisco/O=CryptoFunk/OU=Database/CN=postgres" \
  2>/dev/null

# Sign server certificate with CA
openssl x509 -req -in certs/postgres/server.csr \
  -CA certs/postgres/root.crt \
  -CAkey certs/postgres/root.key \
  -CAcreateserial \
  -out certs/postgres/server.crt \
  -days 365 \
  -extensions v3_req \
  -extfile certs/postgres/postgres.cnf \
  2>/dev/null

# Create client certificate (for applications)
openssl req -new -nodes \
  -out certs/postgres/client.csr \
  -keyout certs/postgres/client.key \
  -subj "/C=US/ST=California/L=San Francisco/O=CryptoFunk/OU=Application/CN=cryptofunk-client" \
  2>/dev/null

openssl x509 -req -in certs/postgres/client.csr \
  -CA certs/postgres/root.crt \
  -CAkey certs/postgres/root.key \
  -CAcreateserial \
  -out certs/postgres/client.crt \
  -days 365 \
  2>/dev/null

# Set permissions (PostgreSQL requires strict permissions)
chmod 600 certs/postgres/server.key certs/postgres/root.key certs/postgres/client.key
chmod 644 certs/postgres/server.crt certs/postgres/root.crt certs/postgres/client.crt

# Copy CA cert for client connections
cp certs/postgres/root.crt certs/postgres/ca.crt

# Clean up CSR files
rm -f certs/postgres/*.csr certs/postgres/*.cnf certs/postgres/*.srl

echo -e "${GREEN}✓ PostgreSQL certificates generated in certs/postgres/${NC}"
echo "  - CA: root.crt"
echo "  - Server: server.crt, server.key"
echo "  - Client: client.crt, client.key"

# ==============================================
# Redis Certificates
# ==============================================

echo ""
echo "Generating Redis certificates..."

# Generate Redis CA certificate
openssl req -new -x509 -days 365 -nodes \
  -out certs/redis/ca.crt \
  -keyout certs/redis/ca.key \
  -subj "/C=US/ST=California/L=San Francisco/O=CryptoFunk/OU=Cache/CN=CryptoFunk Redis CA" \
  2>/dev/null

# Create OpenSSL config for Redis SANs
cat > certs/redis/redis.cnf <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
CN = redis

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = redis
DNS.2 = localhost
DNS.3 = cryptofunk-redis
DNS.4 = redis.cryptofunk.svc.cluster.local
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

# Generate Redis server certificate
openssl req -new -nodes \
  -out certs/redis/redis.csr \
  -keyout certs/redis/redis.key \
  -config certs/redis/redis.cnf \
  -subj "/C=US/ST=California/L=San Francisco/O=CryptoFunk/OU=Cache/CN=redis" \
  2>/dev/null

# Sign server certificate with CA
openssl x509 -req -in certs/redis/redis.csr \
  -CA certs/redis/ca.crt \
  -CAkey certs/redis/ca.key \
  -CAcreateserial \
  -out certs/redis/redis.crt \
  -days 365 \
  -extensions v3_req \
  -extfile certs/redis/redis.cnf \
  2>/dev/null

# Generate client certificate
openssl req -new -nodes \
  -out certs/redis/client.csr \
  -keyout certs/redis/client.key \
  -subj "/C=US/ST=California/L=San Francisco/O=CryptoFunk/OU=Application/CN=cryptofunk-client" \
  2>/dev/null

openssl x509 -req -in certs/redis/client.csr \
  -CA certs/redis/ca.crt \
  -CAkey certs/redis/ca.key \
  -CAcreateserial \
  -out certs/redis/client.crt \
  -days 365 \
  2>/dev/null

# Set permissions
chmod 600 certs/redis/redis.key certs/redis/ca.key certs/redis/client.key
chmod 644 certs/redis/redis.crt certs/redis/ca.crt certs/redis/client.crt

# Clean up CSR files
rm -f certs/redis/*.csr certs/redis/*.cnf certs/redis/*.srl

echo -e "${GREEN}✓ Redis certificates generated in certs/redis/${NC}"
echo "  - CA: ca.crt"
echo "  - Server: redis.crt, redis.key"
echo "  - Client: client.crt, client.key"

# ==============================================
# Verification
# ==============================================

echo ""
echo "Verifying certificates..."

# Verify PostgreSQL certificate
openssl verify -CAfile certs/postgres/root.crt certs/postgres/server.crt > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ PostgreSQL server certificate is valid${NC}"
else
    echo -e "${RED}✗ PostgreSQL server certificate verification failed${NC}"
fi

openssl verify -CAfile certs/postgres/root.crt certs/postgres/client.crt > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ PostgreSQL client certificate is valid${NC}"
else
    echo -e "${RED}✗ PostgreSQL client certificate verification failed${NC}"
fi

# Verify Redis certificate
openssl verify -CAfile certs/redis/ca.crt certs/redis/redis.crt > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Redis server certificate is valid${NC}"
else
    echo -e "${RED}✗ Redis server certificate verification failed${NC}"
fi

openssl verify -CAfile certs/redis/ca.crt certs/redis/client.crt > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Redis client certificate is valid${NC}"
else
    echo -e "${RED}✗ Redis client certificate verification failed${NC}"
fi

# ==============================================
# Certificate Information
# ==============================================

echo ""
echo "Certificate Details:"
echo "===================="

echo ""
echo "PostgreSQL Server Certificate:"
openssl x509 -in certs/postgres/server.crt -noout -subject -issuer -dates -ext subjectAltName

echo ""
echo "Redis Server Certificate:"
openssl x509 -in certs/redis/redis.crt -noout -subject -issuer -dates -ext subjectAltName

# ==============================================
# Security Warnings
# ==============================================

echo ""
echo -e "${YELLOW}=========================================="
echo "Security Warnings"
echo -e "==========================================${NC}"
echo ""
echo -e "${YELLOW}⚠  These are SELF-SIGNED certificates for development/testing only${NC}"
echo -e "${YELLOW}⚠  For production, use certificates from a trusted CA:${NC}"
echo "   - Let's Encrypt (free, automated)"
echo "   - DigiCert, GlobalSign, etc. (commercial)"
echo "   - Internal PKI (e.g., HashiCorp Vault)"
echo ""
echo -e "${YELLOW}⚠  Certificate expiry: 365 days from today${NC}"
echo "   Set up monitoring and rotation procedures"
echo ""
echo -e "${YELLOW}⚠  NEVER commit certs/ directory to git${NC}"
echo "   Ensure certs/ is in .gitignore"
echo ""

# ==============================================
# Next Steps
# ==============================================

echo ""
echo "Next Steps:"
echo "==========="
echo ""
echo "1. For development (local Docker):"
echo "   docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d"
echo ""
echo "2. For Kubernetes:"
echo "   kubectl create secret generic postgres-tls \\"
echo "     --from-file=server.crt=certs/postgres/server.crt \\"
echo "     --from-file=server.key=certs/postgres/server.key \\"
echo "     --from-file=ca.crt=certs/postgres/ca.crt \\"
echo "     -n cryptofunk"
echo ""
echo "   kubectl create secret generic redis-tls \\"
echo "     --from-file=redis.crt=certs/redis/redis.crt \\"
echo "     --from-file=redis.key=certs/redis/redis.key \\"
echo "     --from-file=ca.crt=certs/redis/ca.crt \\"
echo "     -n cryptofunk"
echo ""
echo "3. Update connection strings to use TLS:"
echo "   DATABASE_URL=postgresql://...?sslmode=require&sslrootcert=/certs/ca.crt"
echo "   REDIS_URL=rediss://...?tls_ca_cert=/certs/ca.crt"
echo ""
echo "4. See docs/TLS_SETUP.md for detailed configuration"
echo ""

echo -e "${GREEN}=========================================="
echo "Certificate generation complete!"
echo -e "==========================================${NC}"
