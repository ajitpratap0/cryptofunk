#!/bin/bash
#
# Load Test Script for Vector Search Endpoints
#
# This script uses 'hey' (HTTP load testing tool) to benchmark the vector search endpoints.
# Install hey: go install github.com/rakyll/hey@latest
#
# Usage:
#   ./scripts/load-test-vector-search.sh [API_URL]
#
# Example:
#   ./scripts/load-test-vector-search.sh http://localhost:8080
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default configuration
API_URL="${1:-http://localhost:8080}"
API_BASE="${API_URL}/api/v1"

# Test parameters
REQUESTS=100
CONCURRENCY=10
TIMEOUT=30

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}CryptoFunk Vector Search Load Tests${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${GREEN}API URL:${NC} ${API_BASE}"
echo -e "${GREEN}Requests:${NC} ${REQUESTS}"
echo -e "${GREEN}Concurrency:${NC} ${CONCURRENCY}"
echo -e "${GREEN}Timeout:${NC} ${TIMEOUT}s"
echo ""

# Check if 'hey' is installed
if ! command -v hey &> /dev/null; then
    echo -e "${RED}Error: 'hey' is not installed${NC}"
    echo "Install it with: go install github.com/rakyll/hey@latest"
    exit 1
fi

# Check if API is reachable
echo -e "${YELLOW}Checking API health...${NC}"
if curl -s -f "${API_URL}/health" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ API is reachable${NC}"
else
    echo -e "${RED}✗ API is not reachable at ${API_URL}${NC}"
    echo "Make sure the API server is running (task run-api)"
    exit 1
fi

echo ""

# Function to run a load test
run_load_test() {
    local name="$1"
    local method="$2"
    local url="$3"
    local data="$4"

    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Testing: ${name}${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    if [ "$method" = "POST" ]; then
        hey -n ${REQUESTS} -c ${CONCURRENCY} -t ${TIMEOUT} \
            -m POST \
            -H "Content-Type: application/json" \
            -d "${data}" \
            "${url}"
    else
        hey -n ${REQUESTS} -c ${CONCURRENCY} -t ${TIMEOUT} \
            "${url}"
    fi

    echo ""
}

# Get a valid decision ID for testing
echo -e "${YELLOW}Fetching a valid decision ID...${NC}"
DECISION_ID=$(curl -s "${API_BASE}/decisions?limit=1" | \
    python3 -c "import sys, json; data = json.load(sys.stdin); print(data['decisions'][0]['id'] if data.get('decisions') else '')" 2>/dev/null || echo "")

if [ -z "$DECISION_ID" ]; then
    echo -e "${YELLOW}⚠ No decisions found in database, skipping similar decisions test${NC}"
    SKIP_SIMILAR=true
else
    echo -e "${GREEN}✓ Found decision ID: ${DECISION_ID}${NC}"
    SKIP_SIMILAR=false
fi

echo ""

# Test 1: Text-based semantic search
run_load_test \
    "Text-based Semantic Search" \
    "POST" \
    "${API_BASE}/decisions/search" \
    '{"query": "BTC bullish signal RSI oversold", "limit": 10}'

# Test 2: Text search with symbol filter
run_load_test \
    "Semantic Search with Symbol Filter" \
    "POST" \
    "${API_BASE}/decisions/search" \
    '{"query": "bearish trend reversal", "symbol": "BTC/USDT", "limit": 20}'

# Test 3: Text search with larger limit
run_load_test \
    "Semantic Search (Large Result Set)" \
    "POST" \
    "${API_BASE}/decisions/search" \
    '{"query": "risk management position sizing", "limit": 50}'

# Test 4: Similar decisions (if we have a valid ID)
if [ "$SKIP_SIMILAR" = false ]; then
    run_load_test \
        "Similar Decisions (Vector Search)" \
        "GET" \
        "${API_BASE}/decisions/${DECISION_ID}/similar?limit=10" \
        ""

    run_load_test \
        "Similar Decisions (Large Result Set)" \
        "GET" \
        "${API_BASE}/decisions/${DECISION_ID}/similar?limit=30" \
        ""
fi

# Test 5: List decisions (baseline comparison)
run_load_test \
    "List Decisions (Baseline)" \
    "GET" \
    "${API_BASE}/decisions?limit=50" \
    ""

# Test 6: Decision stats (aggregation query)
run_load_test \
    "Decision Stats" \
    "GET" \
    "${API_BASE}/decisions/stats" \
    ""

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Load tests completed!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}Notes:${NC}"
echo "  - Text search tests use PostgreSQL full-text search (GIN index)"
echo "  - Similar decisions uses pgvector with IVFFlat index"
echo "  - Compare latencies across different query types"
echo "  - Monitor database CPU/memory during tests"
echo ""
echo -e "${BLUE}Monitoring:${NC}"
echo "  - View metrics: http://localhost:9090 (Prometheus)"
echo "  - View dashboards: http://localhost:3000 (Grafana)"
echo "  - Database stats: psql -h localhost -U postgres -d cryptofunk -c 'SELECT * FROM pg_stat_database;'"
echo ""
