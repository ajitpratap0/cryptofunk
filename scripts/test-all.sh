#!/usr/bin/env bash
#
# CryptoFunk - Comprehensive Test & Validation Script
# Runs all checks before committing code
#
# Usage: ./scripts/test-all.sh [--fast]
#   --fast: Skip slow checks (linting, full tests)

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Flags
FAST_MODE=false
if [[ "${1:-}" == "--fast" ]]; then
    FAST_MODE=true
fi

# Track overall status
FAILED=0

# Helper functions
print_header() {
    echo -e "${BLUE}===================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}===================================================${NC}"
}

print_step() {
    echo -e "${YELLOW}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
    FAILED=1
}

# Get project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

print_header "CryptoFunk - Running Pre-Commit Checks"
echo ""

# ============================================================================
# 1. Environment Check
# ============================================================================
print_step "1. Checking Go environment..."

if ! command -v go &> /dev/null; then
    print_error "Go is not installed"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
print_success "Go version: $GO_VERSION"
echo ""

# ============================================================================
# 2. Module Verification
# ============================================================================
print_step "2. Verifying Go modules..."

if go mod verify &> /dev/null; then
    print_success "Go modules verified"
else
    print_error "Go module verification failed"
fi

print_step "   Running go mod tidy..."
if go mod tidy &> /dev/null; then
    print_success "Go modules tidied"
else
    print_error "go mod tidy failed"
fi

# Check for changes after tidy
if ! git diff --exit-code go.mod go.sum &> /dev/null; then
    print_error "go.mod or go.sum has uncommitted changes after tidy"
    echo "   Run 'go mod tidy' and commit the changes"
fi
echo ""

# ============================================================================
# 3. Code Formatting
# ============================================================================
print_step "3. Checking code formatting..."

UNFORMATTED=$(gofmt -l . 2>&1 | grep -v vendor/ | grep '\.go$' || true)
if [ -z "$UNFORMATTED" ]; then
    print_success "All Go files are properly formatted"
else
    print_error "The following files are not formatted:"
    echo "$UNFORMATTED" | while read -r file; do
        echo "     - $file"
    done
    echo "   Run 'go fmt ./...' or 'gofmt -w .' to fix"
fi
echo ""

# ============================================================================
# 4. Go Vet
# ============================================================================
print_step "4. Running go vet..."

if go vet ./... 2>&1 | grep -v "no Go files"; then
    print_error "go vet found issues"
else
    print_success "go vet passed"
fi
echo ""

# ============================================================================
# 5. Linting (if golangci-lint is available)
# ============================================================================
if ! $FAST_MODE; then
    print_step "5. Running golangci-lint..."
    
    if command -v golangci-lint &> /dev/null; then
        if golangci-lint run --timeout 5m ./... 2>&1; then
            print_success "Linting passed"
        else
            print_error "Linting found issues"
        fi
    else
        echo "   ℹ golangci-lint not installed, skipping"
        echo "   Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    fi
    echo ""
fi

# ============================================================================
# 6. Build All Packages
# ============================================================================
print_step "6. Building all packages..."

BUILD_FAILED=0
BUILD_OUTPUT=$(mktemp)

# Build all packages
if go build -v ./... 2>&1 | tee "$BUILD_OUTPUT" | tail -5; then
    print_success "All packages built successfully"
else
    print_error "Build failed"
    cat "$BUILD_OUTPUT"
    BUILD_FAILED=1
fi
rm -f "$BUILD_OUTPUT"
echo ""

# ============================================================================
# 7. Build Main Binaries
# ============================================================================
print_step "7. Building main binaries..."

mkdir -p bin

# List of binaries to build
declare -A BINARIES=(
    ["market-data-server"]="./cmd/mcp-servers/market-data"
    ["test-client"]="./cmd/test-mcp-client"
)

for binary in "${!BINARIES[@]}"; do
    path="${BINARIES[$binary]}"
    echo -n "   Building $binary... "

    if go build -o "bin/$binary" "$path" 2>&1; then
        SIZE=$(ls -lh "bin/$binary" | awk '{print $5}')
        echo -e "${GREEN}✓${NC} ($SIZE)"
    else
        echo -e "${RED}✗${NC}"
        print_error "Failed to build $binary"
    fi
done
echo ""

# ============================================================================
# 8. Unit Tests
# ============================================================================
if ! $FAST_MODE; then
    print_step "8. Running unit tests..."
    
    if go test -short -race -cover ./... 2>&1 | grep -E "(PASS|FAIL|coverage:)" | tail -10; then
        print_success "Unit tests passed"
    else
        print_error "Unit tests failed"
    fi
    echo ""
fi

# ============================================================================
# 9. Check for Common Issues
# ============================================================================
print_step "9. Checking for common issues..."

# Check for TODO/FIXME comments in new code
if git diff --cached --name-only | grep '\.go$' &> /dev/null; then
    TODOS=$(git diff --cached | grep -E '^\+.*\/\/(TODO|FIXME)' | wc -l || true)
    if [ "$TODOS" -gt 0 ]; then
        echo "   ⚠ Found $TODOS new TODO/FIXME comments"
        git diff --cached | grep -E '^\+.*\/\/(TODO|FIXME)' | head -5
    fi
fi

# Check for debug prints
if git diff --cached --name-only | grep '\.go$' &> /dev/null; then
    DEBUG_PRINTS=$(git diff --cached | grep -E '^\+.*fmt\.Print(ln|f)?\(' | wc -l || true)
    if [ "$DEBUG_PRINTS" -gt 0 ]; then
        print_error "Found $DEBUG_PRINTS new fmt.Print statements (use logger instead)"
        git diff --cached | grep -E '^\+.*fmt\.Print(ln|f)?\(' | head -5
    fi
fi

# Check for commented code
if git diff --cached --name-only | grep '\.go$' &> /dev/null; then
    COMMENTED_CODE=$(git diff --cached | grep -E '^\+\s*//.*[{}();]' | wc -l || true)
    if [ "$COMMENTED_CODE" -gt 10 ]; then
        echo "   ⚠ Found large amount of commented code ($COMMENTED_CODE lines)"
    fi
fi

print_success "Common issues check completed"
echo ""

# ============================================================================
# 10. Docker Configuration Validation
# ============================================================================
print_step "10. Validating Docker configuration..."

if command -v docker &> /dev/null; then
    if docker compose -f docker-compose.yml config > /dev/null 2>&1; then
        print_success "Docker Compose configuration is valid"
    else
        print_error "Docker Compose configuration is invalid"
    fi
else
    echo "   ℹ Docker not installed, skipping validation"
fi
echo ""

# ============================================================================
# 11. Database Schema Validation
# ============================================================================
print_step "11. Validating database schema..."

if [ -f "migrations/001_initial_schema.sql" ]; then
    # Check for basic SQL syntax issues
    if grep -qE "(CREATE|ALTER|DROP|INSERT|UPDATE|DELETE)" migrations/001_initial_schema.sql; then
        print_success "Database schema file exists and contains SQL statements"
    else
        print_error "Database schema appears to be empty or invalid"
    fi
else
    print_error "Database schema file not found"
fi
echo ""

# ============================================================================
# Summary
# ============================================================================
print_header "Check Summary"

if [ $FAILED -eq 0 ] && [ $BUILD_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed! Ready to commit.${NC}"
    echo ""
    exit 0
else
    echo -e "${RED}✗ Some checks failed. Please fix the issues above.${NC}"
    echo ""
    echo "To bypass these checks (not recommended), use:"
    echo "  git commit --no-verify"
    echo ""
    exit 1
fi
