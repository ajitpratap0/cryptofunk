# CryptoFunk Scripts

This directory contains development and automation scripts for the CryptoFunk project.

## Available Scripts

### `test-all.sh`

Comprehensive validation script that runs all quality checks before committing code.

**Usage:**
```bash
# Full mode (all checks including linting and tests)
./scripts/test-all.sh

# Fast mode (skip slow checks, used by pre-commit hook)
./scripts/test-all.sh --fast
```

**Checks performed:**

1. **Go Environment** - Verifies Go installation
2. **Module Verification** - Validates go.mod and runs `go mod tidy`
3. **Code Formatting** - Checks with `gofmt`
4. **Go Vet** - Static analysis with `go vet`
5. **Linting** - Runs `golangci-lint` (if installed, full mode only)
6. **Package Build** - Builds all packages
7. **Binary Build** - Builds all executables
8. **Unit Tests** - Runs tests with race detector (full mode only)
9. **Common Issues** - Checks for TODOs, debug prints, commented code
10. **Docker Config** - Validates docker-compose.yml
11. **Database Schema** - Validates SQL migration files

**Exit codes:**
- `0` - All checks passed
- `1` - One or more checks failed

### `install-hooks.sh`

Installs Git hooks for automated quality checks.

**Usage:**
```bash
./scripts/install-hooks.sh
```

**Installed hooks:**

1. **pre-commit** - Runs fast validation before each commit
2. **commit-msg** - Validates commit message format (Conventional Commits)

**Bypass hooks (not recommended):**
```bash
git commit --no-verify
```

## Task Integration

All scripts can be run via Task commands:

```bash
# Install Git hooks
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

## Pre-Commit Hook

Once installed, the pre-commit hook automatically runs before every commit:

```bash
# Normal commit (runs pre-commit checks automatically)
git commit -m "feat: add new feature"

# Bypass checks (emergency only)
git commit --no-verify -m "feat: add new feature"
```

**What happens:**

1. Hook triggers on `git commit`
2. Runs `./scripts/test-all.sh --fast`
3. If checks pass → commit proceeds
4. If checks fail → commit is blocked

## Commit Message Format

The commit-msg hook enforces Conventional Commits format:

```
type(scope): subject

feat(agents): add technical analysis agent
fix(api): resolve connection timeout
docs: update installation guide
```

**Valid types:**
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation
- `style` - Code style (formatting)
- `refactor` - Code refactoring
- `test` - Tests
- `chore` - Maintenance
- `perf` - Performance
- `ci` - CI/CD
- `build` - Build system

## Development Workflow

### Initial Setup

```bash
# 1. Install hooks (one-time setup)
./scripts/install-hooks.sh

# Or using Task
task install-hooks
```

### Before Committing

```bash
# 2. Run fast checks manually (optional)
./scripts/test-all.sh --fast

# Or using Task
task test-fast
```

### Committing

```bash
# 3. Commit (hooks run automatically)
git add .
git commit -m "feat: add new feature"
```

### Before Pushing

```bash
# 4. Run full validation (recommended)
./scripts/test-all.sh

# Or using Task
task test-all
```

## Continuous Integration

The `test-all.sh` script is designed to be used in CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run validation checks
  run: ./scripts/test-all.sh
```

## Troubleshooting

### golangci-lint not installed

The script will skip linting if `golangci-lint` is not installed. To install:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Hook not running

If the pre-commit hook doesn't run:

1. Check it's executable:
   ```bash
   chmod +x .git/hooks/pre-commit
   ```

2. Verify it exists:
   ```bash
   ls -la .git/hooks/pre-commit
   ```

3. Reinstall:
   ```bash
   ./scripts/install-hooks.sh
   ```

### Hook failing unexpectedly

Run the test script manually to see detailed output:

```bash
./scripts/test-all.sh --fast
```

### Need to commit urgently

If you need to commit with failing checks (emergency only):

```bash
git commit --no-verify -m "your message"
```

**⚠️ Warning:** This bypasses all quality checks. Use sparingly!

## Best Practices

1. ✅ **Install hooks immediately** after cloning the repo
2. ✅ **Run `task test-fast`** before committing
3. ✅ **Run `task test-all`** before pushing
4. ✅ **Follow Conventional Commits** format
5. ✅ **Fix issues** instead of bypassing hooks
6. ❌ **Don't use `--no-verify`** except in emergencies
7. ❌ **Don't commit untested code**

## Script Maintenance

When adding new checks to `test-all.sh`:

1. Add a new numbered section
2. Use consistent output formatting
3. Set `FAILED=1` on errors
4. Add to this README
5. Test both fast and full modes

## Exit Codes

All scripts follow this convention:

- `0` - Success
- `1` - Validation failure
- `2` - Missing dependency
- `130` - User interruption (Ctrl+C)

## Related Documentation

- [Taskfile.yml](../Taskfile.yml) - Task automation
- [.git/hooks/](../.git/hooks/) - Git hooks
- [TASKS.md](../TASKS.md) - Project tasks
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Contribution guidelines
