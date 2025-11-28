#!/bin/bash
# Test AlertManager Integration End-to-End
# This script tests alert delivery through AlertManager to configured notification channels

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
ALERTMANAGER_URL="${ALERTMANAGER_URL:-http://localhost:9093}"
TIMEOUT=30

echo -e "${BLUE}==============================================================================${NC}"
echo -e "${BLUE}CryptoFunk AlertManager Integration Test${NC}"
echo -e "${BLUE}==============================================================================${NC}"
echo ""

# Function to check service health
check_service() {
    local service_name=$1
    local url=$2
    local health_path=$3

    echo -n "Checking ${service_name}... "
    if curl -sf "${url}${health_path}" > /dev/null 2>&1; then
        echo -e "${GREEN}OK${NC}"
        return 0
    else
        echo -e "${RED}FAILED${NC}"
        return 1
    fi
}

# Function to wait for alert
wait_for_alert() {
    local alert_name=$1
    local timeout=$2

    echo -n "Waiting for alert '${alert_name}' to fire (timeout: ${timeout}s)... "

    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        local alerts=$(curl -sf "${ALERTMANAGER_URL}/api/v2/alerts" 2>/dev/null || echo "[]")

        if echo "$alerts" | jq -e ".[] | select(.labels.alertname == \"${alert_name}\")" > /dev/null 2>&1; then
            echo -e "${GREEN}FIRED${NC}"
            return 0
        fi

        sleep 2
        elapsed=$((elapsed + 2))
        echo -n "."
    done

    echo -e "${RED}TIMEOUT${NC}"
    return 1
}

# 1. Check service health
echo -e "${YELLOW}Step 1: Checking service health${NC}"
echo ""

SERVICES_OK=true

check_service "Prometheus" "$PROMETHEUS_URL" "/-/healthy" || SERVICES_OK=false
check_service "AlertManager" "$ALERTMANAGER_URL" "/-/healthy" || SERVICES_OK=false

echo ""

if [ "$SERVICES_OK" = false ]; then
    echo -e "${RED}ERROR: One or more services are not healthy. Please start services:${NC}"
    echo -e "  ${YELLOW}docker-compose up -d prometheus alertmanager${NC}"
    echo -e "  ${YELLOW}# OR${NC}"
    echo -e "  ${YELLOW}kubectl get pods -n cryptofunk${NC}"
    exit 1
fi

# 2. Check AlertManager configuration
echo -e "${YELLOW}Step 2: Verifying AlertManager configuration${NC}"
echo ""

echo -n "Checking AlertManager config... "
CONFIG=$(curl -sf "${ALERTMANAGER_URL}/api/v2/status" 2>/dev/null || echo "{}")

if echo "$CONFIG" | jq -e '.config' > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"

    # Display configured receivers
    echo ""
    echo "Configured receivers:"
    echo "$CONFIG" | jq -r '.config.receivers[]?.name' | sed 's/^/  - /'
    echo ""
else
    echo -e "${RED}FAILED${NC}"
    echo -e "${RED}ERROR: Could not retrieve AlertManager configuration${NC}"
    exit 1
fi

# 3. Check Prometheus alert rules
echo -e "${YELLOW}Step 3: Checking Prometheus alert rules${NC}"
echo ""

echo -n "Retrieving loaded alert rules... "
RULES=$(curl -sf "${PROMETHEUS_URL}/api/v1/rules" 2>/dev/null || echo '{"data":{}}')

RULE_COUNT=$(echo "$RULES" | jq '.data.groups[]?.rules[]? | select(.type == "alerting") | .name' | wc -l | tr -d ' ')

if [ "$RULE_COUNT" -gt 0 ]; then
    echo -e "${GREEN}${RULE_COUNT} rules loaded${NC}"
    echo ""
    echo "Sample alert rules:"
    echo "$RULES" | jq -r '.data.groups[]?.rules[]? | select(.type == "alerting") | .name' | head -10 | sed 's/^/  - /'
    echo ""
else
    echo -e "${RED}FAILED${NC}"
    echo -e "${RED}ERROR: No alert rules loaded in Prometheus${NC}"
    exit 1
fi

# 4. Check current firing alerts
echo -e "${YELLOW}Step 4: Checking current alerts${NC}"
echo ""

ALERTS=$(curl -sf "${PROMETHEUS_URL}/api/v1/alerts" 2>/dev/null || echo '{"data":{}}')
FIRING_COUNT=$(echo "$ALERTS" | jq '[.data.alerts[]? | select(.state == "firing")] | length')
PENDING_COUNT=$(echo "$ALERTS" | jq '[.data.alerts[]? | select(.state == "pending")] | length')

echo "Alert status:"
echo "  - Firing: ${FIRING_COUNT}"
echo "  - Pending: ${PENDING_COUNT}"
echo ""

if [ "$FIRING_COUNT" -gt 0 ]; then
    echo "Currently firing alerts:"
    echo "$ALERTS" | jq -r '.data.alerts[]? | select(.state == "firing") | "  - \(.labels.alertname) (severity: \(.labels.severity))"'
    echo ""
fi

# 5. Send test alert to AlertManager
echo -e "${YELLOW}Step 5: Sending test alert to AlertManager${NC}"
echo ""

TEST_ALERT_NAME="TestAlert_$(date +%s)"

echo "Sending test alert: ${TEST_ALERT_NAME}"

TEST_ALERT_PAYLOAD='[
  {
    "labels": {
      "alertname": "'${TEST_ALERT_NAME}'",
      "severity": "info",
      "component": "test",
      "environment": "testing"
    },
    "annotations": {
      "summary": "Test alert from AlertManager integration test",
      "description": "This is a test alert to verify AlertManager is receiving and routing alerts correctly."
    },
    "startsAt": "'$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")'"
  }
]'

RESPONSE=$(curl -sf -X POST \
    -H "Content-Type: application/json" \
    -d "$TEST_ALERT_PAYLOAD" \
    "${ALERTMANAGER_URL}/api/v2/alerts" 2>&1 || echo "FAILED")

if [ "$RESPONSE" != "FAILED" ]; then
    echo -e "${GREEN}Test alert sent successfully${NC}"
else
    echo -e "${RED}Failed to send test alert${NC}"
    echo "Error: $RESPONSE"
    exit 1
fi

echo ""

# 6. Verify test alert appears in AlertManager
echo -e "${YELLOW}Step 6: Verifying test alert in AlertManager${NC}"
echo ""

sleep 2  # Give AlertManager time to process

ALERTMANAGER_ALERTS=$(curl -sf "${ALERTMANAGER_URL}/api/v2/alerts" 2>/dev/null || echo "[]")

if echo "$ALERTMANAGER_ALERTS" | jq -e ".[] | select(.labels.alertname == \"${TEST_ALERT_NAME}\")" > /dev/null 2>&1; then
    echo -e "${GREEN}Test alert found in AlertManager${NC}"

    # Display alert details
    echo ""
    echo "Alert details:"
    echo "$ALERTMANAGER_ALERTS" | jq ".[] | select(.labels.alertname == \"${TEST_ALERT_NAME}\")"
    echo ""
else
    echo -e "${RED}Test alert not found in AlertManager${NC}"
    echo "This may indicate issues with alert routing or processing."
    exit 1
fi

# 7. Check alert routing
echo -e "${YELLOW}Step 7: Checking alert routing${NC}"
echo ""

echo "Alert should be routed to receiver(s):"
echo "$ALERTMANAGER_ALERTS" | jq -r ".[] | select(.labels.alertname == \"${TEST_ALERT_NAME}\") | .receivers[]?.name" | sed 's/^/  - /'
echo ""

# 8. Silence test alert
echo -e "${YELLOW}Step 8: Cleaning up - silencing test alert${NC}"
echo ""

SILENCE_PAYLOAD='{
  "matchers": [
    {
      "name": "alertname",
      "value": "'${TEST_ALERT_NAME}'",
      "isRegex": false,
      "isEqual": true
    }
  ],
  "startsAt": "'$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")'",
  "endsAt": "'$(date -u -d '+1 hour' +"%Y-%m-%dT%H:%M:%S.000Z" 2>/dev/null || date -u -v+1H +"%Y-%m-%dT%H:%M:%S.000Z")'",
  "createdBy": "test-alerts.sh",
  "comment": "Silencing test alert after integration test"
}'

SILENCE_RESPONSE=$(curl -sf -X POST \
    -H "Content-Type: application/json" \
    -d "$SILENCE_PAYLOAD" \
    "${ALERTMANAGER_URL}/api/v2/silences" 2>&1 || echo "FAILED")

if [ "$SILENCE_RESPONSE" != "FAILED" ]; then
    SILENCE_ID=$(echo "$SILENCE_RESPONSE" | jq -r '.silenceID // empty')
    if [ -n "$SILENCE_ID" ]; then
        echo -e "${GREEN}Test alert silenced (ID: ${SILENCE_ID})${NC}"
    else
        echo -e "${YELLOW}Alert may already be silenced or expired${NC}"
    fi
else
    echo -e "${YELLOW}Warning: Could not silence test alert${NC}"
fi

echo ""

# 9. Summary
echo -e "${BLUE}==============================================================================${NC}"
echo -e "${GREEN}AlertManager Integration Test: PASSED${NC}"
echo -e "${BLUE}==============================================================================${NC}"
echo ""
echo "Summary:"
echo "  - Prometheus: Healthy"
echo "  - AlertManager: Healthy"
echo "  - Alert rules loaded: ${RULE_COUNT}"
echo "  - Test alert sent and verified: Success"
echo ""
echo "Next steps:"
echo "  1. Check notification channels (Slack, Email) for test alert"
echo "  2. Review alert runbook: docs/ALERT_RUNBOOK.md"
echo "  3. Configure production alert channels in .env or K8s secrets"
echo ""
echo "AlertManager UI: ${ALERTMANAGER_URL}"
echo "Prometheus UI: ${PROMETHEUS_URL}"
echo ""
