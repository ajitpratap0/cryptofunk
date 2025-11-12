#!/usr/bin/env bash
#
# watch-logs.sh - Watch logs from CryptoFunk services
#
# This script provides easy log viewing for development:
# - All services (orchestrator + agents + docker)
# - Individual services
# - Filtered logs (errors only, specific agent, etc.)
#
# Usage:
#   ./scripts/dev/watch-logs.sh                    # All logs
#   ./scripts/dev/watch-logs.sh orchestrator       # Orchestrator only
#   ./scripts/dev/watch-logs.sh agents             # All agents
#   ./scripts/dev/watch-logs.sh technical          # Technical agent only
#   ./scripts/dev/watch-logs.sh docker             # Docker services
#   ./scripts/dev/watch-logs.sh --errors           # Errors only
#   ./scripts/dev/watch-logs.sh --grep "BUY"       # Filter by pattern

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Log directories
AGENT_LOG_DIR="./tmp/logs"
ORCHESTRATOR_LOG_DIR="./tmp/logs"

# Parse arguments
SERVICE="${1:-all}"
FILTER="${2:-}"
PATTERN="${3:-}"

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}CryptoFunk Log Viewer${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# Function to watch Docker service logs
watch_docker() {
    local service=$1
    echo -e "${BLUE}Watching Docker logs for: $service${NC}"
    echo -e "${YELLOW}Press Ctrl+C to exit${NC}"
    echo ""
    docker-compose logs -f "$service"
}

# Function to watch local log files
watch_local() {
    local pattern=$1
    echo -e "${BLUE}Watching local logs: $pattern${NC}"
    echo -e "${YELLOW}Press Ctrl+C to exit${NC}"
    echo ""

    local log_files=($(find "$AGENT_LOG_DIR" -name "$pattern" 2>/dev/null || true))

    if [[ ${#log_files[@]} -eq 0 ]]; then
        echo -e "${RED}No log files found matching: $pattern${NC}"
        echo ""
        echo "Available logs:"
        ls -1 "$AGENT_LOG_DIR" 2>/dev/null || echo "  (no logs yet)"
        exit 1
    fi

    tail -f "${log_files[@]}"
}

# Function to watch all logs
watch_all() {
    echo -e "${BLUE}Watching ALL logs (Docker + Local)${NC}"
    echo -e "${YELLOW}Press Ctrl+C to exit${NC}"
    echo ""

    # Check if docker-compose is running
    if docker-compose ps | grep -q "Up"; then
        echo -e "${GREEN}Docker services detected, including in tail...${NC}"
        # Use multitail if available, otherwise fall back to tail
        if command -v multitail &> /dev/null; then
            multitail \
                -l "docker-compose logs -f orchestrator" \
                -l "docker-compose logs -f postgres" \
                -l "docker-compose logs -f redis" \
                -l "docker-compose logs -f nats" \
                $(find "$AGENT_LOG_DIR" -name "*.log" -exec echo "-l cat {} \;" 2>/dev/null)
        else
            # Fallback: show docker logs and local logs separately
            (docker-compose logs -f orchestrator postgres redis nats 2>/dev/null &)
            sleep 1
            local log_files=($(find "$AGENT_LOG_DIR" -name "*.log" 2>/dev/null || true))
            if [[ ${#log_files[@]} -gt 0 ]]; then
                tail -f "${log_files[@]}"
            else
                echo -e "${YELLOW}No local agent logs yet${NC}"
                wait
            fi
        fi
    else
        echo -e "${YELLOW}Docker services not running${NC}"
        watch_local "*.log"
    fi
}

# Function to filter logs for errors
watch_errors() {
    echo -e "${RED}Filtering for ERRORS only${NC}"
    echo -e "${YELLOW}Press Ctrl+C to exit${NC}"
    echo ""

    # Watch both Docker and local logs, filter for errors
    if docker-compose ps | grep -q "Up"; then
        (docker-compose logs -f 2>&1 | grep -i "error\|fatal\|panic" &)
    fi

    local log_files=($(find "$AGENT_LOG_DIR" -name "*.log" 2>/dev/null || true))
    if [[ ${#log_files[@]} -gt 0 ]]; then
        tail -f "${log_files[@]}" | grep -i "error\|fatal\|panic\|ERR\|FTL"
    else
        wait
    fi
}

# Function to grep logs
watch_grep() {
    local pattern=$1
    echo -e "${BLUE}Filtering logs for pattern: ${GREEN}$pattern${NC}"
    echo -e "${YELLOW}Press Ctrl+C to exit${NC}"
    echo ""

    # Watch both Docker and local logs, filter for pattern
    if docker-compose ps | grep -q "Up"; then
        (docker-compose logs -f 2>&1 | grep -i "$pattern" &)
    fi

    local log_files=($(find "$AGENT_LOG_DIR" -name "*.log" 2>/dev/null || true))
    if [[ ${#log_files[@]} -gt 0 ]]; then
        tail -f "${log_files[@]}" | grep -i "$pattern"
    else
        wait
    fi
}

# Main logic
case "$SERVICE" in
    all)
        watch_all
        ;;
    --errors)
        watch_errors
        ;;
    --grep)
        if [[ -z "$PATTERN" ]]; then
            echo -e "${RED}Error: --grep requires a pattern${NC}"
            echo "Usage: $0 --grep PATTERN"
            exit 1
        fi
        watch_grep "$PATTERN"
        ;;
    orchestrator)
        if docker-compose ps | grep -q "orchestrator.*Up"; then
            watch_docker "orchestrator"
        else
            watch_local "orchestrator.log"
        fi
        ;;
    agents)
        watch_local "*-agent.log"
        ;;
    technical|trend|risk|reversion|orderbook|sentiment|arbitrage)
        watch_local "${SERVICE}*.log"
        ;;
    docker)
        echo -e "${BLUE}Watching all Docker service logs${NC}"
        echo -e "${YELLOW}Press Ctrl+C to exit${NC}"
        echo ""
        docker-compose logs -f
        ;;
    postgres|redis|nats|api|prometheus|grafana)
        watch_docker "$SERVICE"
        ;;
    *)
        echo -e "${RED}Unknown service: $SERVICE${NC}"
        echo ""
        echo "Usage: $0 [SERVICE] [OPTIONS]"
        echo ""
        echo "Services:"
        echo "  all            - All logs (default)"
        echo "  orchestrator   - Orchestrator logs"
        echo "  agents         - All agent logs"
        echo "  technical      - Technical agent logs"
        echo "  trend          - Trend agent logs"
        echo "  risk           - Risk agent logs"
        echo "  docker         - All Docker service logs"
        echo "  postgres       - PostgreSQL logs"
        echo "  redis          - Redis logs"
        echo "  nats           - NATS logs"
        echo ""
        echo "Options:"
        echo "  --errors       - Show errors only"
        echo "  --grep PATTERN - Filter by pattern"
        echo ""
        echo "Examples:"
        echo "  $0                          # All logs"
        echo "  $0 orchestrator             # Orchestrator only"
        echo "  $0 agents                   # All agents"
        echo "  $0 --errors                 # Errors only"
        echo "  $0 --grep 'BUY.*BTC'        # Filter for BTC buy signals"
        exit 1
        ;;
esac
