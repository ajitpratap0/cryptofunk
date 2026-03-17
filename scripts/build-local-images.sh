#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

PULL_INFRA=false
for arg in "$@"; do
  case "$arg" in
    --pull-infra) PULL_INFRA=true ;;
  esac
done

echo "==> Building CryptoFunk local images"
cd "${PROJECT_ROOT}"

export DOCKER_BUILDKIT=1

if [ "$PULL_INFRA" = true ]; then
  echo "==> Pulling infrastructure images"
  docker pull timescale/timescaledb:latest-pg17 &
  docker pull redis:7-alpine &
  docker pull nats:2-alpine &
  docker pull prom/prometheus:latest &
  docker pull grafana/grafana:latest &
  docker pull prom/alertmanager:latest &
  wait
  echo "==> Infrastructure images pulled"
fi

# Tier 1: Build all Go services in parallel
echo "==> Building Go services (parallel)"
build_pids=()

# Orchestrator
docker build -f deployments/docker/Dockerfile.orchestrator -t cryptofunk/orchestrator:local . &
build_pids+=($!)

# API
docker build -f deployments/docker/Dockerfile.api -t cryptofunk/api:local . &
build_pids+=($!)

# Migrate
docker build -f deployments/docker/Dockerfile.migrate -t cryptofunk/migrate:local . &
build_pids+=($!)

# MCP servers
for server in market-data technical-indicators risk-analyzer order-executor polymarket; do
  docker build -f deployments/docker/Dockerfile.mcp-server \
    --build-arg SERVER_NAME="${server}" \
    -t "cryptofunk/${server}-server:local" . &
  build_pids+=($!)
done

# Agents
for agent in technical-agent orderbook-agent sentiment-agent trend-agent reversion-agent risk-agent arbitrage-agent polymarket-agent; do
  docker build -f deployments/docker/Dockerfile.agent \
    --build-arg AGENT_NAME="${agent}" \
    -t "cryptofunk/${agent}:local" . &
  build_pids+=($!)
done

# Wait for all Tier 1 builds
echo "==> Waiting for Go service builds..."
failed=0
for pid in "${build_pids[@]+"${build_pids[@]}"}"; do
  if ! wait "$pid"; then
    echo "ERROR: A build job failed (pid $pid)"
    failed=1
  fi
done

if [ "$failed" -eq 1 ]; then
  echo "ERROR: One or more builds failed"
  exit 1
fi
echo "==> Go services built successfully"

# Tier 2: Dashboard (different build context)
echo "==> Building dashboard"
docker build -f deployments/docker/Dockerfile.dashboard \
  --build-arg NEXT_PUBLIC_API_URL=http://localhost \
  -t cryptofunk/dashboard:local \
  web/dashboard/
echo "==> Dashboard built"

echo ""
echo "==> All images built successfully!"
echo "Images tagged as cryptofunk/*:local"
docker images | grep "cryptofunk.*local"
