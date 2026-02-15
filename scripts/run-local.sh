#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# Check prerequisites
for cmd in docker docker-compose; do
  command -v "$cmd" >/dev/null 2>&1 || error "$cmd is required but not installed"
done
info "Prerequisites OK"

# Setup .env
if [ ! -f .env ]; then
  if [ -f .env.example ]; then
    cp .env.example .env
    info "Created .env from .env.example"
  else
    touch .env
  fi
fi

# Helper: set var in .env if not already set
ensure_secret() {
  local key="$1" default="$2"
  if ! grep -q "^${key}=" .env 2>/dev/null || grep -q "^${key}=CHANGE_ME" .env 2>/dev/null; then
    local value
    read -rp "Enter ${key} [auto-generate]: " value
    value="${value:-$default}"
    if grep -q "^${key}=" .env; then
      sed -i.bak "s|^${key}=.*|${key}=${value}|" .env && rm -f .env.bak
    else
      echo "${key}=${value}" >> .env
    fi
    info "Set ${key}"
  fi
}

ensure_secret "POSTGRES_PASSWORD" "$(openssl rand -base64 24 | tr -d '/+=')"
ensure_secret "JWT_SECRET" "$(openssl rand -base64 48 | tr -d '/+=')"
ensure_secret "GRAFANA_ADMIN_PASSWORD" "$(openssl rand -base64 16 | tr -d '/+=')"

# Start services
info "Starting CryptoFunk services..."
docker-compose up -d

# Wait for health
info "Waiting for services to be healthy..."
for i in $(seq 1 60); do
  if docker-compose ps | grep -q "(unhealthy)"; then
    sleep 2
  elif docker-compose ps | grep -q "(health: starting)"; then
    sleep 2
  else
    break
  fi
done

# Status
echo ""
info "=== CryptoFunk Status ==="
docker-compose ps
echo ""
info "=== Service URLs ==="
echo "  Dashboard:   http://localhost:3001"
echo "  API:         http://localhost:8082"
echo "  Grafana:     http://localhost:3000"
echo "  Prometheus:  http://localhost:9090"
echo "  NATS Monitor: http://localhost:8222"
echo "  Vault:       http://localhost:8200"
echo ""
info "Done! 🚀"
