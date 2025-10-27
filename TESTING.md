# CryptoFunk Testing Infrastructure

Comprehensive testing and validation system with automated pre-commit hooks.

## Quick Start

```bash
# 1. Install Git hooks (one-time setup)
./scripts/install-hooks.sh

# 2. Make changes to code
# ... edit files ...

# 3. Commit (hooks run automatically)
git add .
git commit -m "feat: your feature"
```

## Testing Agent Overview

The testing infrastructure consists of three main components:

### 1. Validation Script (`scripts/test-all.sh`)

Comprehensive validation script that runs 11 different checks:

```bash
# Run all checks (includes linting and full tests)
./scripts/test-all.sh

# Run fast checks (used by pre-commit hook)
./scripts/test-all.sh --fast
```

**Checks performed:**

| # | Check | Description | Time |
|---|-------|-------------|------|
| 1 | Go Environment | Verifies Go installation | <1s |
| 2 | Module Verification | Validates go.mod, runs go mod tidy | 2-3s |
| 3 | Code Formatting | Checks with gofmt | 1s |
| 4 | Go Vet | Static analysis | 2-3s |
| 5 | Linting | golangci-lint (optional, full mode only) | 10-30s |
| 6 | Package Builds | Builds all packages | 3-5s |
| 7 | Binary Builds | Builds all executables | 3-5s |
| 8 | Unit Tests | Tests with race detector (full mode only) | 5-10s |
| 9 | Common Issues | Checks for TODOs, debug prints, etc. | <1s |
| 10 | Docker Config | Validates docker-compose.yml | <1s |
| 11 | Database Schema | Validates SQL migrations | <1s |

**Total time:**
- Fast mode: ~10-15 seconds
- Full mode: ~30-60 seconds

### 2. Git Hooks

Automatically installed with `./scripts/install-hooks.sh`:

#### Pre-Commit Hook
- Runs before every `git commit`
- Executes `./scripts/test-all.sh --fast`
- Blocks commit if checks fail
- Can bypass with `git commit --no-verify`

#### Commit-Msg Hook
- Validates commit message format
- Enforces Conventional Commits standard
- Example: `feat(scope): description`

### 3. Task Integration

All scripts integrated into Taskfile.yml:

```bash
# Install hooks
task install-hooks

# Run all validation checks
task test-all

# Run fast validation checks
task test-fast

# Run pre-commit checks manually
task pre-commit

# Validate code quality only
task validate
```

## Validation Checks Details

### 1. Go Environment
```bash
✓ Checks Go is installed
✓ Displays Go version
```

### 2. Module Verification
```bash
✓ Runs go mod verify
✓ Runs go mod tidy
✓ Checks for uncommitted changes in go.mod/go.sum
```

### 3. Code Formatting
```bash
✓ Runs gofmt -l to find unformatted files
✓ Suggests running go fmt ./...
```

### 4. Static Analysis
```bash
✓ Runs go vet ./...
✓ Reports suspicious constructs
```

### 5. Linting (Optional)
```bash
✓ Runs golangci-lint if available
✓ Skipped if not installed
✓ Only in full mode (--fast skips this)
```

### 6. Package Builds
```bash
✓ Builds all Go packages
✓ go build -v ./...
```

### 7. Binary Builds
```bash
✓ Builds market-data-server
✓ Builds test-client
✓ Reports binary sizes
```

### 8. Unit Tests
```bash
✓ Runs go test -short -race -cover ./...
✓ Race detector enabled
✓ Coverage reported
✓ Only in full mode
```

### 9. Common Issues
```bash
✓ Checks for new TODO/FIXME comments
✓ Checks for debug fmt.Print statements
✓ Checks for excessive commented code
```

### 10. Docker Configuration
```bash
✓ Validates docker-compose.yml syntax
✓ Runs docker compose config
```

### 11. Database Schema
```bash
✓ Checks migration files exist
✓ Validates SQL syntax
```

## Usage Examples

### Development Workflow

```bash
# 1. Install hooks (first time only)
./scripts/install-hooks.sh

# 2. Make changes
vim internal/config/config.go

# 3. Test locally (optional)
./scripts/test-all.sh --fast

# 4. Commit (hooks run automatically)
git add .
git commit -m "feat(config): add new configuration option"

# Hook output:
# 🔍 Running pre-commit checks...
# ✓ Go environment verified
# ✓ Modules verified
# ✓ Code formatting passed
# ✓ go vet passed
# ✓ All packages built
# ✓ Binaries built
# ✓ Common issues check passed
# ✓ Docker config validated
# ✓ Database schema validated
# ✓ All checks passed! Ready to commit.

# 5. Push
git push origin master
```

### Manual Testing

```bash
# Run all checks before pushing
./scripts/test-all.sh

# Run quick checks during development
./scripts/test-all.sh --fast

# Via Task
task test-all
task test-fast
```

### Bypass Hooks (Emergency Only)

```bash
# Bypass all hooks (not recommended)
git commit --no-verify -m "emergency fix"

# Bypass commit-msg only
git commit --no-edit

# Bypass pre-commit only (still validates message)
# Not possible, must use --no-verify
```

## Commit Message Format

Enforced by commit-msg hook:

```
type(scope): subject

[optional body]

[optional footer]
```

**Valid types:**
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation changes
- `style` - Code style (formatting, missing semi colons, etc.)
- `refactor` - Code refactoring
- `test` - Adding or updating tests
- `chore` - Maintenance tasks
- `perf` - Performance improvements
- `ci` - CI/CD changes
- `build` - Build system changes
- `revert` - Reverting changes

**Examples:**

```bash
# Good
git commit -m "feat(agents): add technical analysis agent"
git commit -m "fix(api): resolve connection timeout issue"
git commit -m "docs: update installation instructions"
git commit -m "test: add unit tests for config loader"

# Bad (will be rejected)
git commit -m "added stuff"
git commit -m "WIP"
git commit -m "fixed bug"
```

## Troubleshooting

### Hook not running

```bash
# Check hook exists and is executable
ls -la .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit

# Reinstall hooks
./scripts/install-hooks.sh
```

### golangci-lint not found

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Or skip linting (it's optional)
# The script will continue without it
```

### Checks failing unexpectedly

```bash
# Run script manually to see detailed output
./scripts/test-all.sh --fast

# Check specific failure
go fmt ./...          # Fix formatting
go vet ./...          # Fix static analysis issues
go mod tidy           # Fix module issues
```

### Need to commit urgently

```bash
# Bypass hooks (not recommended)
git commit --no-verify -m "urgent: fix production issue"

# Better approach: fix issues first
./scripts/test-all.sh --fast
# ... fix reported issues ...
git commit -m "fix: resolve production issue"
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Validate

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Install dependencies
        run: go mod download
      
      - name: Run validation
        run: ./scripts/test-all.sh
```

### GitLab CI

```yaml
test:
  image: golang:1.21
  script:
    - ./scripts/test-all.sh
  only:
    - merge_requests
    - master
```

## Configuration

### Customize checks

Edit `scripts/test-all.sh` to:
- Add new validation checks
- Modify existing checks
- Adjust error handling
- Change output formatting

### Disable specific checks

Comment out sections in `scripts/test-all.sh`:

```bash
# # ============================================================================
# # 5. Linting (if golangci-lint is available)
# # ============================================================================
# print_step "5. Running golangci-lint..."
# # ... rest of linting section ...
```

### Customize commit message format

Edit `.git/hooks/commit-msg` to modify the regex pattern.

## Performance Tips

### Speed up checks

```bash
# Use fast mode for pre-commit
./scripts/test-all.sh --fast

# Skip specific checks by editing the script
# Comment out slow sections
```

### Parallel execution

The script runs checks sequentially. For faster execution:
- Build binaries in parallel (future enhancement)
- Run tests in parallel with `-parallel` flag

## Best Practices

1. ✅ **Always install hooks** after cloning
2. ✅ **Run `task test-fast`** before committing
3. ✅ **Run `task test-all`** before pushing
4. ✅ **Fix issues** instead of bypassing hooks
5. ✅ **Follow Conventional Commits** format
6. ✅ **Keep commits atomic** and focused
7. ❌ **Don't use `--no-verify`** except emergencies
8. ❌ **Don't commit untested code**
9. ❌ **Don't commit debug statements**
10. ❌ **Don't commit with failing tests**

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All checks passed |
| 1 | One or more checks failed |
| 2 | Missing dependency |
| 130 | User interrupted (Ctrl+C) |

## Related Documentation

- [scripts/README.md](scripts/README.md) - Detailed script documentation
- [Taskfile.yml](Taskfile.yml) - Task automation
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines (if exists)
- [.github/workflows/](.github/workflows/) - CI/CD workflows (if exists)

## Getting Help

If you encounter issues:

1. Check [Troubleshooting](#troubleshooting) section above
2. Run `./scripts/test-all.sh --fast` manually for details
3. Open an issue on GitHub
4. Check commit logs for working examples

## Future Enhancements

- [ ] Add integration tests
- [ ] Add benchmark tests
- [ ] Add security scanning
- [ ] Add dependency vulnerability checking
- [ ] Add code coverage thresholds
- [ ] Add performance regression tests
- [ ] Add parallel test execution
- [ ] Add custom lint rules
- [ ] Add auto-fixing for common issues
- [ ] Add commit template

---

**Note:** This testing infrastructure ensures code quality and prevents broken commits. Always run checks before committing and pushing!
