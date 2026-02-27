#!/bin/bash
# Sequential Docker build for all CryptoFunk services
# Builds one at a time to avoid OOM on constrained Docker Desktop (7.6GB RAM)
# BuildKit cache mounts mean go mod download is only slow on first build

set -e
cd "$(dirname "$0")/.."

export DOCKER_BUILDKIT=1

SERVICES=(
  migrate
  api
  orchestrator
  market-data-server
  risk-analyzer-server
  order-executor-server
  technical-indicators-server
  technical-agent
  sentiment-agent
  reversion-agent
  risk-agent
  orderbook-agent
)

FAILED=()
SUCCEEDED=()

for svc in "${SERVICES[@]}"; do
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "🔨 Building: $svc"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  if docker compose build "$svc" 2>&1; then
    SUCCEEDED+=("$svc")
    echo "✅ $svc built successfully"
  else
    FAILED+=("$svc")
    echo "❌ $svc FAILED"
  fi
  echo ""
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "BUILD REPORT"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Succeeded (${#SUCCEEDED[@]}): ${SUCCEEDED[*]}"
echo "❌ Failed (${#FAILED[@]}): ${FAILED[*]}"

if [ ${#FAILED[@]} -gt 0 ]; then
  exit 1
fi
