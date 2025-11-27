#!/bin/bash

# End-to-end test script for Technical Indicators MCP Server
# This script tests all 5 indicators via stdio protocol

SERVER_BIN="./bin/technical-indicators"

# Check if server binary exists
if [ ! -f "$SERVER_BIN" ]; then
    echo "Error: Server binary not found at $SERVER_BIN"
    echo "Please run: go build -o bin/technical-indicators ./cmd/mcp-servers/technical-indicators/"
    exit 1
fi

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local request="$2"
    local expected_field="$3"

    echo -n "Testing $test_name... "

    # Send request and capture response (filter out logs)
    response=$(echo "$request" | $SERVER_BIN 2>/dev/null | tail -1)

    # Check if response contains expected field
    if echo "$response" | jq -e "$expected_field" > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}FAIL${NC}"
        echo "Response: $response"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

echo "========================================="
echo "Technical Indicators MCP Server E2E Test"
echo "========================================="
echo

# Test 1: Initialize
run_test "Initialize" \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
    '.result.serverInfo.name'

# Test 2: List Tools
run_test "List Tools" \
    '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
    '.result.tools | length == 5'

# Test 3: Calculate RSI
run_test "Calculate RSI" \
    '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"calculate_rsi","arguments":{"prices":[44.34,44.09,43.61,43.03,43.52,43.13,42.66,42.82,42.67,43.13,43.37,43.23,43.08,42.07,41.99,42.18,42.49,42.28,42.51,43.13],"period":14}}}' \
    '.result.value'

# Test 4: Calculate MACD
prices_50='[100,100.5,101,101.5,102,102.5,103,103.5,104,104.5,105,105.5,106,106.5,107,107.5,108,108.5,109,109.5,110,110.5,111,111.5,112,112.5,113,113.5,114,114.5,115,115.5,116,116.5,117,117.5,118,118.5,119,119.5,120,120.5,121,121.5,122,122.5,123,123.5,124,124.5]'
run_test "Calculate MACD" \
    "{\"jsonrpc\":\"2.0\",\"id\":4,\"method\":\"tools/call\",\"params\":{\"name\":\"calculate_macd\",\"arguments\":{\"prices\":$prices_50,\"fast_period\":12,\"slow_period\":26,\"signal_period\":9}}}" \
    '.result.macd'

# Test 5: Calculate Bollinger Bands
prices_30='[100,101,102,103,104,105,106,107,108,109,100,101,102,103,104,105,106,107,108,109,100,101,102,103,104,105,106,107,108,109]'
run_test "Calculate Bollinger Bands" \
    "{\"jsonrpc\":\"2.0\",\"id\":5,\"method\":\"tools/call\",\"params\":{\"name\":\"calculate_bollinger_bands\",\"arguments\":{\"prices\":$prices_30,\"period\":20,\"std_dev\":2}}}" \
    '.result.upper'

# Test 6: Calculate EMA
prices_20='[100,101,102,103,104,105,106,107,108,109,110,111,112,113,114,115,116,117,118,119]'
run_test "Calculate EMA" \
    "{\"jsonrpc\":\"2.0\",\"id\":6,\"method\":\"tools/call\",\"params\":{\"name\":\"calculate_ema\",\"arguments\":{\"prices\":$prices_20,\"period\":10}}}" \
    '.result.value'

# Test 7: Calculate ADX
high_30='[102,103,104,105,106,107,108,109,110,111,112,113,114,115,116,117,118,119,120,121,122,123,124,125,126,127,128,129,130,131]'
low_30='[98,99,100,101,102,103,104,105,106,107,108,109,110,111,112,113,114,115,116,117,118,119,120,121,122,123,124,125,126,127]'
close_30='[100,101,102,103,104,105,106,107,108,109,110,111,112,113,114,115,116,117,118,119,120,121,122,123,124,125,126,127,128,129]'
run_test "Calculate ADX" \
    "{\"jsonrpc\":\"2.0\",\"id\":7,\"method\":\"tools/call\",\"params\":{\"name\":\"calculate_adx\",\"arguments\":{\"high\":$high_30,\"low\":$low_30,\"close\":$close_30,\"period\":14}}}" \
    '.result.value'

# Test 8: Error Handling - Invalid Tool
run_test "Error: Invalid Tool" \
    '{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"invalid_tool","arguments":{}}}' \
    '.error.message'

# Test 9: Error Handling - Missing Prices
run_test "Error: Missing Prices" \
    '{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"calculate_rsi","arguments":{"period":14}}}' \
    '.error.message'

# Summary
echo
echo "========================================="
echo "Test Summary"
echo "========================================="
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
echo

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
