#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="cryptofunk"
PORT_FORWARD=false
ORCH_PID=""
API_PID=""

for arg in "$@"; do
  case "$arg" in
    --port-forward) PORT_FORWARD=true ;;
  esac
done

cleanup() {
  echo "==> Cleaning up port-forwards"
  [ -n "$ORCH_PID" ] && kill "$ORCH_PID" 2>/dev/null || true
  [ -n "$API_PID" ] && kill "$API_PID" 2>/dev/null || true
}

if [ "$PORT_FORWARD" = true ]; then
  trap cleanup EXIT
  echo "==> Setting up port-forwards"
  kubectl port-forward svc/orchestrator-service 8081:8081 -n "${NAMESPACE}" &
  ORCH_PID=$!
  kubectl port-forward svc/api-service 18080:80 -n "${NAMESPACE}" &
  API_PID=$!
  sleep 3
fi

PASS=0
FAIL=0

check() {
  local name="$1"
  local cmd="$2"
  if eval "$cmd" > /dev/null 2>&1; then
    echo "  PASS: ${name}"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: ${name}"
    FAIL=$((FAIL + 1))
  fi
}

echo "==> Checking pods"
check "All pods Running/Completed" \
  "[ \"\$(kubectl get pods -n ${NAMESPACE} --no-headers | grep -v -E '(Running|Completed)' | wc -l | tr -d ' ')\" = '0' ]"

echo "==> Checking orchestrator endpoints"
check "Orchestrator /health" "curl -sf http://localhost:8081/health"
check "Orchestrator /liveness" "curl -sf http://localhost:8081/liveness"
check "Orchestrator /readiness" "curl -sf http://localhost:8081/readiness"
check "Orchestrator /api/v1/status" "curl -sf http://localhost:8081/api/v1/status"

echo "==> Checking API endpoints"
check "API /api/v1/health" "curl -sf http://localhost:18080/api/v1/health"

echo ""
echo "==> Results: ${PASS} passed, ${FAIL} failed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
echo "==> All checks passed!"
