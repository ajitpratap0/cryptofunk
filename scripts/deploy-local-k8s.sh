#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
OVERLAY_DIR="${PROJECT_ROOT}/deployments/k8s/overlays/local"

GEN_SECRETS=false
API_KEY=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --gen-secrets) GEN_SECRETS=true; shift ;;
    --api-key) API_KEY="$2"; shift 2 ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

NAMESPACE="cryptofunk"

echo "==> Creating namespace (idempotent)"
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

if [ "$GEN_SECRETS" = true ]; then
  echo "==> Generating secrets"
  POSTGRES_PASSWORD=$(openssl rand -base64 32 | tr -d '/+=\n' | head -c 32)
  REDIS_PASSWORD=$(openssl rand -base64 24 | tr -d '/+=\n' | head -c 24)
  JWT_SECRET=$(openssl rand -base64 48 | tr -d '/+=\n' | head -c 48)
  NATS_USER="cryptofunk"
  NATS_PASSWORD=$(openssl rand -base64 24 | tr -d '/+=\n' | head -c 24)

  cat > "${OVERLAY_DIR}/secret-local.yaml" << EOF
apiVersion: v1
kind: Secret
metadata:
  name: cryptofunk-secrets
  namespace: ${NAMESPACE}
type: Opaque
stringData:
  POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"
  REDIS_PASSWORD: "${REDIS_PASSWORD}"
  JWT_SECRET: "${JWT_SECRET}"
  LLM_API_KEY: "${API_KEY}"
  ANTHROPIC_API_KEY: "${API_KEY}"
  NATS_USER: "${NATS_USER}"
  NATS_PASSWORD: "${NATS_PASSWORD}"
EOF
  echo "==> Secrets generated at ${OVERLAY_DIR}/secret-local.yaml"
fi

echo "==> Applying Kustomize overlay"
kubectl apply -k "${OVERLAY_DIR}"

echo "==> Waiting for infrastructure (postgres, redis, nats)..."
kubectl rollout status deployment/postgres -n "${NAMESPACE}" --timeout=120s || true
kubectl rollout status deployment/redis -n "${NAMESPACE}" --timeout=60s || true
kubectl rollout status deployment/nats -n "${NAMESPACE}" --timeout=60s || true

echo "==> Waiting for migrate job to complete..."
kubectl wait --for=condition=complete job/migrate -n "${NAMESPACE}" --timeout=120s || \
  echo "WARN: migrate job not found or timed out"

echo "==> Waiting for MCP servers..."
for svc in market-data-server technical-indicators-server risk-analyzer-server order-executor-server; do
  kubectl rollout status deployment/${svc} -n "${NAMESPACE}" --timeout=120s || true
done

echo "==> Waiting for orchestrator and API..."
kubectl rollout status deployment/orchestrator -n "${NAMESPACE}" --timeout=120s || true
kubectl rollout status deployment/api -n "${NAMESPACE}" --timeout=120s || true

echo "==> Waiting for agents..."
for agent in technical-agent orderbook-agent sentiment-agent trend-agent reversion-agent risk-agent arbitrage-agent polymarket-agent; do
  kubectl rollout status deployment/${agent} -n "${NAMESPACE}" --timeout=120s || true
done

echo ""
echo "==> Deployment complete!"
echo ""
echo "Port-forward commands:"
echo "  kubectl port-forward svc/orchestrator-service 8081:8081 -n ${NAMESPACE} &"
echo "  kubectl port-forward svc/api-service 18080:80 -n ${NAMESPACE} &"
echo "  kubectl port-forward svc/prometheus-service 19090:9090 -n ${NAMESPACE} &"
echo "  kubectl port-forward svc/grafana-service 13000:3000 -n ${NAMESPACE} &"
echo ""
echo "Health checks (after port-forwarding):"
echo "  curl http://localhost:8081/health"
echo "  curl http://localhost:8081/readiness"
echo "  curl http://localhost:18080/api/v1/health"
