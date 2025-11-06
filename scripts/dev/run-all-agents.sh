#!/usr/bin/env bash
#
# run-all-agents.sh - Start all trading agents in the background
#
# This script starts all trading agents as background processes and manages them:
# - Technical Analysis Agent
# - Trend Following Agent
# - Mean Reversion Agent
# - Risk Management Agent
# - (Optional) Orderbook Agent
# - (Optional) Sentiment Agent
# - (Optional) Arbitrage Agent
#
# Usage:
#   ./scripts/dev/run-all-agents.sh start    # Start all agents
#   ./scripts/dev/run-all-agents.sh stop     # Stop all agents
#   ./scripts/dev/run-all-agents.sh restart  # Restart all agents
#   ./scripts/dev/run-all-agents.sh status   # Show status of all agents
#   ./scripts/dev/run-all-agents.sh logs     # Tail logs from all agents

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# PID file directory
PID_DIR="./tmp/pids"
LOG_DIR="./tmp/logs"

# Create directories
mkdir -p "$PID_DIR" "$LOG_DIR"

# Agent binaries (add more as needed)
declare -A AGENTS=(
    ["technical"]="./bin/technical-agent"
    ["trend"]="./bin/trend-agent"
    ["risk"]="./bin/risk-agent"
    ["reversion"]="./bin/reversion-agent"
    # Uncomment to enable optional agents:
    # ["orderbook"]="./bin/orderbook-agent"
    # ["sentiment"]="./bin/sentiment-agent"
    # ["arbitrage"]="./bin/arbitrage-agent"
)

# Start a single agent
start_agent() {
    local name=$1
    local binary=$2
    local pid_file="$PID_DIR/$name.pid"
    local log_file="$LOG_DIR/$name.log"

    if [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
        echo -e "${YELLOW}  $name agent already running (PID: $(cat "$pid_file"))${NC}"
        return 0
    fi

    if [[ ! -f "$binary" ]]; then
        echo -e "${RED}  ✗ $name agent binary not found: $binary${NC}"
        echo -e "${YELLOW}    Run: go build -o $binary ./cmd/agents/${name}-agent${NC}"
        return 1
    fi

    echo -e "${BLUE}  Starting $name agent...${NC}"
    $binary >> "$log_file" 2>&1 &
    local pid=$!
    echo $pid > "$pid_file"
    sleep 0.5

    if kill -0 $pid 2>/dev/null; then
        echo -e "${GREEN}  ✓ $name agent started (PID: $pid)${NC}"
        return 0
    else
        echo -e "${RED}  ✗ $name agent failed to start${NC}"
        echo -e "${YELLOW}    Check logs: tail -f $log_file${NC}"
        rm -f "$pid_file"
        return 1
    fi
}

# Stop a single agent
stop_agent() {
    local name=$1
    local pid_file="$PID_DIR/$name.pid"

    if [[ ! -f "$pid_file" ]]; then
        echo -e "${YELLOW}  $name agent not running${NC}"
        return 0
    fi

    local pid=$(cat "$pid_file")
    if kill -0 $pid 2>/dev/null; then
        echo -e "${BLUE}  Stopping $name agent (PID: $pid)...${NC}"
        kill $pid 2>/dev/null || true
        sleep 1

        # Force kill if still running
        if kill -0 $pid 2>/dev/null; then
            echo -e "${YELLOW}  Force killing $name agent...${NC}"
            kill -9 $pid 2>/dev/null || true
        fi

        rm -f "$pid_file"
        echo -e "${GREEN}  ✓ $name agent stopped${NC}"
    else
        echo -e "${YELLOW}  $name agent not running (stale PID file)${NC}"
        rm -f "$pid_file"
    fi
}

# Check agent status
check_status() {
    local name=$1
    local pid_file="$PID_DIR/$name.pid"

    if [[ ! -f "$pid_file" ]]; then
        echo -e "  $name: ${RED}STOPPED${NC}"
        return 1
    fi

    local pid=$(cat "$pid_file")
    if kill -0 $pid 2>/dev/null; then
        echo -e "  $name: ${GREEN}RUNNING${NC} (PID: $pid)"
        return 0
    else
        echo -e "  $name: ${RED}STOPPED${NC} (stale PID)"
        rm -f "$pid_file"
        return 1
    fi
}

# Main commands
cmd_start() {
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}Starting All Trading Agents${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""

    for name in "${!AGENTS[@]}"; do
        start_agent "$name" "${AGENTS[$name]}"
    done

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}All agents started${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "View logs:"
    echo "  ./scripts/dev/watch-logs.sh"
    echo ""
    echo "Stop agents:"
    echo "  ./scripts/dev/run-all-agents.sh stop"
    echo ""
}

cmd_stop() {
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}Stopping All Trading Agents${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""

    for name in "${!AGENTS[@]}"; do
        stop_agent "$name"
    done

    echo ""
    echo -e "${GREEN}All agents stopped${NC}"
    echo ""
}

cmd_restart() {
    cmd_stop
    sleep 1
    cmd_start
}

cmd_status() {
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}Agent Status${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""

    local running=0
    local stopped=0

    for name in "${!AGENTS[@]}"; do
        if check_status "$name"; then
            ((running++))
        else
            ((stopped++))
        fi
    done

    echo ""
    echo "Summary: ${GREEN}$running running${NC}, ${RED}$stopped stopped${NC}"
    echo ""
}

cmd_logs() {
    echo -e "${YELLOW}Tailing logs from all agents...${NC}"
    echo -e "${YELLOW}Press Ctrl+C to exit${NC}"
    echo ""

    local log_files=()
    for name in "${!AGENTS[@]}"; do
        local log_file="$LOG_DIR/$name.log"
        if [[ -f "$log_file" ]]; then
            log_files+=("$log_file")
        fi
    done

    if [[ ${#log_files[@]} -eq 0 ]]; then
        echo -e "${YELLOW}No log files found${NC}"
        exit 0
    fi

    tail -f "${log_files[@]}"
}

# Parse command
case "${1:-}" in
    start)
        cmd_start
        ;;
    stop)
        cmd_stop
        ;;
    restart)
        cmd_restart
        ;;
    status)
        cmd_status
        ;;
    logs)
        cmd_logs
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|logs}"
        echo ""
        echo "Commands:"
        echo "  start    - Start all trading agents"
        echo "  stop     - Stop all trading agents"
        echo "  restart  - Restart all trading agents"
        echo "  status   - Show status of all agents"
        echo "  logs     - Tail logs from all agents"
        exit 1
        ;;
esac
