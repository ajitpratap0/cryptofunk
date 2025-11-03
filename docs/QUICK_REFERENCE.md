# Quick Reference Guide

Essential commands for CryptoFunk development.

---

## Installation

```bash
# Install Task (if not already installed)
brew install go-task/tap/go-task  # macOS/Linux
winget install Task.Task           # Windows

# Clone and setup
git clone https://github.com/ajitpratap0/cryptofunk.git
cd cryptofunk
cp .env.example .env              # Add your API keys
task init                          # Initialize everything
```

---

## Essential Commands

### Getting Started

```bash
task --list              # Show all available tasks
task init               # Initialize project (first time setup)
task dev                # Setup development environment
task check-env          # Verify environment variables
```

### Building

```bash
task build              # Build everything
task build-orchestrator # Build orchestrator only
task build-agents       # Build all agents
task build-servers      # Build all MCP servers
```

### Running Services

```bash
# Infrastructure
task docker-up          # Start Postgres, Redis, NATS
task docker-down        # Stop all Docker services
task docker-status      # Show service status

# Application
task run-orchestrator   # Start orchestrator
task run-agent-technical # Start technical agent
task run-agent-trend    # Start trend agent
task run-agent-risk     # Start risk agent

# Quick start (paper trading)
task run-paper          # Start in paper trading mode
```

### Testing

```bash
task test               # Run all tests with coverage
task test-unit          # Unit tests only
task test-integration   # Integration tests
task test-watch         # Watch and test on changes
task lint               # Run linter
task fmt                # Format code
task check              # Run fmt, lint, test
```

### Database

```bash
task db-migrate         # Run migrations
task db-shell           # Open PostgreSQL shell
task db-backup          # Backup database
task db-reset           # Reset database (⚠️  deletes data)
```

### Monitoring

```bash
task status             # System status
task metrics            # Current metrics
task positions          # Current positions
task agents             # Agent status
task monitor            # Open Grafana/Prometheus
```

### Development Workflow

```bash
task dev-watch          # Watch and rebuild on changes
task dev-clean          # Clean development environment
```

### Cleanup

```bash
task clean              # Remove binaries
task clean-all          # Remove everything
```

---

## Common Workflows

### First Time Setup

```bash
# 1. Install prerequisites
brew install go-task/tap/go-task
brew install docker

# 2. Clone and configure
git clone <repo>
cd cryptofunk
cp .env.example .env
# Edit .env with your API keys

# 3. Initialize
task init

# 4. Verify
task check-env
```

### Daily Development

```bash
# Morning: Start services
task docker-up
task run-orchestrator    # Terminal 1
task run-agent-technical # Terminal 2
task run-agent-trend     # Terminal 3

# During development
task dev-watch           # Auto-rebuild on changes
task test-watch          # Auto-test on changes

# Before committing
task check               # fmt + lint + test

# Evening: Cleanup
task docker-down
```

### Testing a Strategy

```bash
# 1. Build
task build

# 2. Backtest
./bin/backtest -symbol BTCUSDT -from 2024-01-01 -to 2024-06-01

# 3. If good, try paper trading
task run-paper

# 4. Monitor
task status
task positions
```

### Adding a New Agent

```bash
# 1. Create agent code
# cmd/agents/my-agent/main.go

# 2. Add build task to Taskfile.yml
# (See existing agents as template)

# 3. Build and test
task build-agent-my-agent
./bin/my-agent

# 4. Integrate with orchestrator
# Update configs/agents.yaml
```

### Troubleshooting

```bash
# Check Docker services
task docker-status
task docker-logs

# Check environment
task check-env

# Verify API keys
task verify-keys

# Reset everything (nuclear option)
task clean-all
task docker-clean
task init
```

---

## Environment Variables

Required in `.env`:

```bash
# Exchange
BINANCE_API_KEY=your_key
BINANCE_SECRET_KEY=your_secret

# Database
POSTGRES_PASSWORD=your_password

# Redis
REDIS_PASSWORD=your_password

# Optional
NEWS_API_KEY=your_key
LOG_LEVEL=debug
```

Verify with:
```bash
task check-env
```

---

## Directory Structure Quick Reference

```
cryptofunk/
├── cmd/                    # Main applications
│   ├── orchestrator/       # Central coordinator
│   ├── agents/            # Trading agents
│   └── mcp-servers/       # MCP servers
├── internal/              # Private code
│   ├── orchestrator/      # Orchestration logic
│   ├── agents/           # Agent implementations
│   └── indicators/       # Technical indicators
├── configs/              # Configuration files
├── bin/                  # Built binaries (gitignored)
└── Taskfile.yml         # Task definitions
```

---

## API Endpoints

```bash
# Status
GET  http://localhost:8080/api/v1/status

# Agents
GET  http://localhost:8080/api/v1/agents

# Positions
GET  http://localhost:8080/api/v1/positions

# Orders
GET  http://localhost:8080/api/v1/orders

# Decisions (Explainability)
GET  http://localhost:8080/api/v1/decisions
GET  http://localhost:8080/api/v1/decisions/:id
GET  http://localhost:8080/api/v1/decisions/:id/similar
GET  http://localhost:8080/api/v1/decisions/stats

# Control
POST http://localhost:8080/api/v1/trade/start
POST http://localhost:8080/api/v1/trade/stop

# Metrics
GET  http://localhost:8080/api/v1/metrics

# WebSocket
WS   ws://localhost:8080/api/v1/ws
```

Test with:
```bash
task status      # curl + jq
task metrics     # curl + jq
task positions   # curl + jq
```

---

## Monitoring Dashboards

```bash
# Open dashboards
task monitor

# URLs
Grafana:    http://localhost:3000  (admin/admin)
Prometheus: http://localhost:9090
API:        http://localhost:8080
```

---

## Task Advanced Usage

### Parallel Execution

Tasks with `deps:` run in parallel automatically:

```bash
task build  # Builds all components in parallel
```

### Watch Mode

Any task can be watched:

```bash
task --watch build       # Rebuild on file changes
task test-watch          # Re-test on file changes
```

### Dry Run

See what would run:

```bash
task --dry build
```

### Force Run

Ignore timestamp checks:

```bash
task --force build
```

### List Tasks

```bash
task --list              # All tasks with descriptions
task --list-all          # Include internal tasks
```

### Task Info

```bash
task --summary build     # Show task details
```

---

## Configuration Files

| File | Purpose |
|------|---------|
| `Taskfile.yml` | Build automation |
| `.env` | Environment variables (not in git) |
| `configs/config.yaml` | Application config |
| `configs/agents.yaml` | Agent configuration |
| `configs/mcp-servers.yaml` | MCP server config |
| `docker-compose.yml` | Docker services |

---

## Keyboard Shortcuts (for Taskfile)

In your shell, add aliases:

```bash
# ~/.bashrc or ~/.zshrc
alias t='task'
alias tb='task build'
alias tt='task test'
alias tl='task --list'
alias tw='task --watch'
alias tds='task docker-status'
```

Then use:
```bash
t build      # Instead of task build
tb           # Build
tt           # Test
tl           # List tasks
```

---

## Git Pre-commit Hook (Optional)

`.git/hooks/pre-commit`:

```bash
#!/bin/bash
task check
```

Make executable:
```bash
chmod +x .git/hooks/pre-commit
```

---

## VS Code Integration

`.vscode/tasks.json`:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Build",
      "type": "shell",
      "command": "task build",
      "group": {
        "kind": "build",
        "isDefault": true
      }
    },
    {
      "label": "Test",
      "type": "shell",
      "command": "task test",
      "group": {
        "kind": "test",
        "isDefault": true
      }
    }
  ]
}
```

Then: `Cmd+Shift+B` to build, `Cmd+Shift+T` to test.

---

## Troubleshooting Task Issues

### "task: command not found"

```bash
# Install Task
brew install go-task/tap/go-task  # macOS/Linux
winget install Task.Task          # Windows
```

### "Task version mismatch"

```bash
task --version  # Check version (need v3+)
brew upgrade go-task/tap/go-task
```

### "Cannot connect to Docker"

```bash
# Check Docker is running
docker ps

# Start Docker services
task docker-up
```

### "Environment variable not set"

```bash
# Check .env exists
cat .env

# Verify variables loaded
task check-env
```

---

## Performance Tips

1. **Use watch mode** for faster development:
   ```bash
   task dev-watch
   ```

2. **Parallel builds** are automatic:
   ```bash
   task build  # Everything builds in parallel
   ```

3. **Incremental builds** via `sources:`/`generates:`
   - Only rebuilds when source files change

4. **Use specific tasks** instead of full rebuilds:
   ```bash
   task build-agent-technical  # Instead of task build
   ```

---

## CI/CD Quick Reference

### GitHub Actions

```yaml
- name: Install Task
  run: |
    sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d ~/.local/bin

- name: Build
  run: task build

- name: Test
  run: task test
```

### GitLab CI

```yaml
test:
  before_script:
    - sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d /usr/local/bin
  script:
    - task check
```

---

## Getting Help

```bash
# Task help
task --help
task --list

# Project help
cat README.md
cat docs/GETTING_STARTED.md
cat docs/TASK_VS_MAKE.md

# Online resources
# - Task docs: https://taskfile.dev
# - Project docs: https://github.com/ajitpratap0/cryptofunk
```

---

**Quick Reference Version**: 1.0
**Updated**: 2025-10-27
**For**: CryptoFunk Trading Platform
