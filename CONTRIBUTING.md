# Contributing to CryptoFunk

Thank you for your interest in contributing to CryptoFunk! This document provides guidelines and instructions for contributing to this multi-agent AI trading system.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Code Style Guidelines](#code-style-guidelines)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Project Structure](#project-structure)
- [Adding New Features](#adding-new-features)

## Code of Conduct

### Our Pledge

We are committed to providing a welcoming and inclusive environment for all contributors, regardless of experience level, background, or identity.

### Expected Behavior

- Be respectful and constructive in communication
- Focus on what is best for the project and community
- Show empathy towards other contributors
- Accept constructive criticism gracefully
- Report unacceptable behavior to project maintainers

### Unacceptable Behavior

- Harassment, discrimination, or offensive comments
- Trolling, insulting/derogatory comments, or personal attacks
- Publishing others' private information without permission
- Any conduct that could reasonably be considered inappropriate

## Getting Started

### Prerequisites

- **Go 1.21+** (currently using Go 1.25.3)
- **Docker** and **Docker Compose** for local development
- **Task** (taskfile.dev) for running build commands
- **PostgreSQL 15+** with TimescaleDB and pgvector extensions
- **Redis** for caching
- **NATS** for event messaging

### Installation

1. **Fork and clone the repository**:
   ```bash
   git clone https://github.com/YOUR_USERNAME/cryptofunk.git
   cd cryptofunk
   ```

2. **Install Task** (if not already installed):
   ```bash
   # macOS
   brew install go-task/tap/go-task

   # Linux
   sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin

   # Or via Go
   go install github.com/go-task/task/v3/cmd/task@latest
   ```

3. **Set up environment**:
   ```bash
   # Copy environment template
   cp .env.example .env

   # Edit .env with your configuration
   # At minimum, set DATABASE_URL and TRADING_MODE=PAPER
   ```

4. **Start development environment**:
   ```bash
   # Start all infrastructure (PostgreSQL, Redis, NATS, etc.)
   task dev

   # Or manually:
   task docker-up    # Start infrastructure
   task db-migrate   # Run migrations
   task build        # Build all binaries
   ```

5. **Verify setup**:
   ```bash
   task test         # Run all tests
   task lint         # Run linter
   task check        # Run fmt, lint, and test
   ```

## Development Workflow

### 1. Create a Feature Branch

```bash
# Always branch from main
git checkout main
git pull origin main

# Create feature branch (use descriptive names)
git checkout -b feature/your-feature-name
# OR for bug fixes:
git checkout -b fix/bug-description
```

### 2. Make Your Changes

- Write clear, self-documenting code
- Add tests for new functionality
- Update documentation as needed
- Follow the code style guidelines below

### 3. Test Your Changes

```bash
# Run all tests
task test

# Run specific package tests
go test -v ./internal/orchestrator/...

# Run with race detector
go test -race ./...

# Check test coverage
task test  # Shows coverage report
```

### 4. Format and Lint

```bash
# Format code (required before commit)
task fmt

# Run linter
task lint

# Run all checks
task check
```

### 5. Commit Your Changes

Follow the [Commit Message Guidelines](#commit-message-guidelines) below.

### 6. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

## Code Style Guidelines

### Go Code Style

We follow standard Go conventions with some project-specific guidelines:

#### General Principles

1. **Use `gofmt`**: All code must be formatted with `gofmt -s`
   ```bash
   task fmt  # Formats all code
   ```

2. **Pass `golangci-lint`**: No linting errors allowed
   ```bash
   task lint
   ```

3. **Exported names must have comments**:
   ```go
   // Good
   // DB wraps the PostgreSQL connection pool
   type DB struct {
       pool *pgxpool.Pool
   }

   // Bad
   type DB struct {  // Missing comment
       pool *pgxpool.Pool
   }
   ```

4. **Use descriptive variable names**:
   ```go
   // Good
   exchangeService := exchange.NewService(database)

   // Bad
   es := exchange.NewService(db)  // Too terse
   ```

#### Error Handling

1. **Always check errors**:
   ```go
   // Good
   pool, err := pgxpool.NewWithConfig(ctx, config)
   if err != nil {
       return nil, fmt.Errorf("failed to create connection pool: %w", err)
   }
   ```

2. **Wrap errors with context** using `%w`:
   ```go
   if err != nil {
       return fmt.Errorf("failed to execute order: %w", err)
   }
   ```

3. **Use early returns** to reduce nesting:
   ```go
   // Good
   if err != nil {
       return err
   }
   // Continue happy path

   // Bad
   if err == nil {
       // Many nested lines
   }
   ```

#### Logging

Use **zerolog** for all logging:

```go
import "github.com/rs/zerolog/log"

// Info logging
log.Info().Msg("Database connection pool created successfully")

// Error logging with fields
log.Error().
    Err(err).
    Str("symbol", symbol).
    Msg("Failed to fetch price")

// IMPORTANT: For MCP servers, output to stderr only
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
```

**CRITICAL**: MCP servers communicate via JSON-RPC 2.0 over stdio. All logs, debug output, and errors **must go to stderr**. Never use `fmt.Printf()`, `log.Println()`, or `println()` in MCP server code.

#### Context Usage

1. **Always pass context** as the first parameter:
   ```go
   func (db *DB) Ping(ctx context.Context) error
   ```

2. **Use context for cancellation and timeouts**:
   ```go
   ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
   defer cancel()
   ```

#### Database Access

1. **Use the connection pool** from `internal/db`:
   ```go
   database, err := db.New(ctx)
   defer database.Close()
   ```

2. **Use parameterized queries** (pgx handles this):
   ```go
   // Good
   row := db.Pool().QueryRow(ctx, "SELECT * FROM positions WHERE id = $1", positionID)

   // Bad - SQL injection risk
   query := fmt.Sprintf("SELECT * FROM positions WHERE id = %s", positionID)
   ```

#### MCP Server Patterns

All MCP servers must follow this pattern:

```go
// 1. Logging to stderr only
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

// 2. Initialize dependencies
database, err := db.New(ctx)
exchangeService := exchange.NewService(database)

// 3. Implement stdio JSON-RPC 2.0
decoder := json.NewDecoder(os.Stdin)
encoder := json.NewEncoder(os.Stdout)

// 4. Never print to stdout except for MCP protocol
// stdout is reserved for JSON-RPC messages only
```

### Testing Guidelines

1. **Table-driven tests** for multiple scenarios:
   ```go
   func TestPlaceOrder(t *testing.T) {
       tests := []struct {
           name    string
           input   OrderRequest
           want    *Order
           wantErr bool
       }{
           {"valid market order", marketReq, expectedOrder, false},
           {"invalid symbol", invalidReq, nil, true},
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               got, err := PlaceOrder(tt.input)
               if (err != nil) != tt.wantErr {
                   t.Errorf("PlaceOrder() error = %v, wantErr %v", err, tt.wantErr)
               }
               // ... assertions
           })
       }
   }
   ```

2. **Use testify for assertions**:
   ```go
   import "github.com/stretchr/testify/assert"

   assert.NoError(t, err)
   assert.Equal(t, expected, actual)
   assert.NotNil(t, result)
   ```

3. **Mock external dependencies**:
   - Use `internal/exchange/mock.go` for exchange mocking
   - Use `testcontainers` for database integration tests
   - Mock MCP servers for agent tests

4. **Test coverage requirements**:
   - Overall project coverage: **>80%**
   - New packages: **>90%** coverage
   - Critical trading logic: **100%** coverage

## Testing Requirements

### Running Tests

```bash
# All tests with coverage
task test

# Unit tests only
task test-unit

# Integration tests
task test-integration

# Specific package
go test -v ./internal/orchestrator/...

# With race detector
go test -race ./...

# Verbose output
go test -v -race ./internal/exchange/...
```

### Test Organization

```
tests/
├── unit/           # Unit tests (fast, no external deps)
├── integration/    # Integration tests (database, Redis, etc.)
├── e2e/           # End-to-end tests (full system)
└── fixtures/      # Test data and fixtures
```

### Writing Tests

1. **File naming**: `*_test.go` alongside source files
2. **Test naming**: `TestFunctionName` or `TestType_Method`
3. **Benchmark naming**: `BenchmarkFunctionName`
4. **Example naming**: `ExampleFunctionName`

### Test Requirements for PRs

- All new code must have tests
- Tests must pass: `task test`
- No race conditions: `go test -race ./...`
- Coverage must not decrease
- Integration tests for database changes
- E2E tests for new features

## Pull Request Process

### Before Submitting

1. **Create an issue** (for non-trivial changes):
   - Describe the problem or feature
   - Discuss approach with maintainers
   - Get feedback before investing time

2. **Update documentation**:
   - Update README.md if needed
   - Add/update comments in code
   - Update CLAUDE.md for architectural changes
   - Update API.md for API changes

3. **Run all checks**:
   ```bash
   task check  # Runs fmt, lint, and test
   ```

4. **Update TASKS.md** if completing a task:
   - Mark task as complete: `- [x] **TXXX**`
   - Add implementation notes

### PR Checklist

- [ ] Code follows style guidelines
- [ ] Self-reviewed code
- [ ] Added/updated tests
- [ ] All tests pass (`task test`)
- [ ] Linter passes (`task lint`)
- [ ] Documentation updated
- [ ] TASKS.md updated (if applicable)
- [ ] Commit messages follow guidelines

### PR Title Format

Use conventional commit format:

```
feat: Add semantic memory system for agents
fix: Correct slippage calculation in mock exchange
docs: Update MCP integration guide
test: Add E2E tests for orchestrator
refactor: Simplify order execution logic
```

### PR Description Template

```markdown
## Description
Brief description of changes

## Motivation
Why is this change needed?

## Changes
- Bullet list of changes
- Be specific

## Testing
How was this tested?
- [ ] Unit tests
- [ ] Integration tests
- [ ] Manual testing

## Screenshots (if applicable)
Add screenshots for UI changes

## Related Issues
Closes #123
Related to #456
```

### Review Process

1. **At least one approval** required from maintainers
2. **All CI checks must pass** (once CI is set up)
3. **Address review comments** promptly
4. **Squash commits** if requested
5. Maintainers will merge when ready

## Commit Message Guidelines

### Format

```
<type>: <subject>

<body (optional)>

<footer (optional)>
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, missing semicolons, etc.)
- **refactor**: Code refactoring (no functional changes)
- **perf**: Performance improvements
- **test**: Adding or updating tests
- **build**: Build system changes
- **ci**: CI/CD changes
- **chore**: Other changes (dependencies, etc.)

### Subject Line Rules

- Use imperative mood ("Add feature" not "Added feature")
- Don't capitalize first letter
- No period at the end
- Maximum 72 characters
- Include task ID for phase work: `(TXXX)`

### Examples

```bash
# Good
git commit -m "feat: Phase 7 Position Tracking & P&L Calculation (T144-T146)"
git commit -m "fix: correct slippage calculation in mock exchange"
git commit -m "docs: update MCP integration guide with new tools"
git commit -m "test: add E2E tests for orchestrator coordination"

# Bad
git commit -m "Fixed stuff"  # Too vague
git commit -m "Added new feature."  # Has period, past tense
git commit -m "WIP"  # Not descriptive
```

### Body (Optional)

- Explain what and why (not how)
- Wrap at 72 characters
- Separate from subject with blank line

```
feat: add semantic memory system for agents

Implements pgvector-based semantic memory to store and retrieve
agent knowledge. Includes 5 knowledge types (fact, pattern,
experience, strategy, risk) with relevance scoring based on
confidence, importance, success rate, and recency.

Related to Phase 11 (T201)
```

## Project Structure

Understanding the codebase organization:

```
cryptofunk/
├── cmd/                      # Executable entry points
│   ├── orchestrator/         # MCP orchestrator
│   ├── mcp-servers/         # MCP tool servers
│   ├── agents/              # Trading agents
│   ├── api/                 # REST/WebSocket API
│   └── migrate/             # Migration tool
├── internal/                # Private application code
│   ├── db/                  # Database layer
│   ├── exchange/            # Exchange abstraction
│   ├── risk/                # Risk management
│   ├── indicators/          # Technical indicators
│   ├── orchestrator/        # Orchestrator logic
│   └── agents/              # Agent infrastructure
├── migrations/              # SQL migrations
├── configs/                 # Configuration files
├── deployments/             # Docker/K8s manifests
├── docs/                    # Documentation
├── scripts/                 # Utility scripts
└── tests/                   # Organized test suites
```

### Key Conventions

- **cmd/**: One directory per executable
- **internal/**: Private packages (not importable)
- **pkg/**: Public libraries (if any)
- **migrations/**: Numbered SQL files (001_*.sql)
- **configs/**: YAML configuration files

## Adding New Features

### Adding a New MCP Tool

1. **Add tool handler** to `cmd/mcp-servers/*/main.go`
2. **Register tool** in `handleInitialize()` response
3. **Implement logic** in `internal/` service layer
4. **Add tests** in `*_test.go`
5. **Update documentation** (MCP_GUIDE.md)

### Adding a New Agent

1. **Create directory**: `cmd/agents/your-agent/`
2. **Implement MCP client**: Connect to orchestrator
3. **Define agent behavior**: Analysis or strategy logic
4. **Add to orchestrator**: Register in `configs/agents.yaml`
5. **Add tests**: Unit and integration tests
6. **Update documentation**: MCP_GUIDE.md

### Adding a Database Migration

1. **Create file**: `migrations/00X_description.sql`
2. **Write SQL**: Use PostgreSQL syntax
3. **Test migration**: `task db-migrate`
4. **Test rollback**: Include DOWN migration if needed
5. **Update schema docs**: Document new tables/columns

### Adding API Endpoints

1. **Define handler** in `cmd/api/main.go`
2. **Implement logic** in appropriate `internal/` package
3. **Add tests**: HTTP handler tests
4. **Update API.md**: Document endpoint
5. **Update OpenAPI spec**: Once API.md exists

## Additional Resources

- **TASKS.md**: Complete implementation plan (10 phases, 244 tasks)
- **README.md**: Project overview and quick start
- **CLAUDE.md**: Guidance for Claude Code (architecture deep-dive)
- **docs/OPEN_SOURCE_TOOLS.md**: Technology stack rationale
- **docs/MCP_INTEGRATION.md**: MCP architecture details
- **Taskfile.yml**: All available commands (50+ tasks)

## Questions?

- **Bugs**: Open an issue with `[BUG]` prefix
- **Features**: Open an issue with `[FEATURE]` prefix
- **Questions**: Open an issue with `[QUESTION]` prefix
- **Security**: Email maintainers directly (see SECURITY.md)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

**Thank you for contributing to CryptoFunk!** Your contributions help build a better AI-powered trading platform.
