#!/usr/bin/env bash
#
# Install Git Hooks for CryptoFunk
# Run this script to set up pre-commit hooks

set -e

# Get project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}Installing Git hooks for CryptoFunk...${NC}"
echo ""

# Create hooks directory if it doesn't exist
mkdir -p "$HOOKS_DIR"

# Install pre-commit hook
echo -e "${YELLOW}▶ Installing pre-commit hook...${NC}"

cat > "$HOOKS_DIR/pre-commit" << 'HOOK_EOF'
#!/usr/bin/env bash
#
# CryptoFunk Pre-Commit Hook
# Automatically runs validation checks before each commit
#
# To bypass this hook (not recommended), use:
#   git commit --no-verify

set -e

# Get project root
PROJECT_ROOT="$(git rev-parse --show-toplevel)"

# Colors
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 Running pre-commit checks...${NC}"
echo ""

# Run the comprehensive test script
if "$PROJECT_ROOT/scripts/test-all.sh" --fast; then
    exit 0
else
    echo ""
    echo -e "${RED}❌ Pre-commit checks failed!${NC}"
    echo ""
    echo "Fix the issues above, or bypass with:"
    echo "  git commit --no-verify"
    echo ""
    exit 1
fi
HOOK_EOF

chmod +x "$HOOKS_DIR/pre-commit"

echo -e "${GREEN}✓ Pre-commit hook installed${NC}"
echo ""

# Install commit-msg hook for commit message validation
echo -e "${YELLOW}▶ Installing commit-msg hook...${NC}"

cat > "$HOOKS_DIR/commit-msg" << 'HOOK_EOF'
#!/usr/bin/env bash
#
# CryptoFunk Commit Message Hook
# Validates commit message format
#
# Conventional Commits format:
#   type(scope): subject
#
# Types: feat, fix, docs, style, refactor, test, chore

set -e

COMMIT_MSG_FILE=$1
COMMIT_MSG=$(cat "$COMMIT_MSG_FILE")

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Skip merge commits and revert commits
if echo "$COMMIT_MSG" | grep -qE '^(Merge|Revert)'; then
    exit 0
fi

# Conventional commit pattern
PATTERN='^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(\(.+\))?: .{1,72}'

if ! echo "$COMMIT_MSG" | grep -qE "$PATTERN"; then
    echo -e "${RED}❌ Invalid commit message format${NC}"
    echo ""
    echo "Commit messages should follow Conventional Commits format:"
    echo ""
    echo -e "${GREEN}  type(scope): subject${NC}"
    echo ""
    echo "Valid types:"
    echo "  feat     - New feature"
    echo "  fix      - Bug fix"
    echo "  docs     - Documentation changes"
    echo "  style    - Code style changes (formatting, etc.)"
    echo "  refactor - Code refactoring"
    echo "  test     - Adding or updating tests"
    echo "  chore    - Maintenance tasks"
    echo "  perf     - Performance improvements"
    echo "  ci       - CI/CD changes"
    echo "  build    - Build system changes"
    echo ""
    echo "Examples:"
    echo "  ${GREEN}feat(agents): add technical analysis agent${NC}"
    echo "  ${GREEN}fix(api): resolve connection timeout issue${NC}"
    echo "  ${GREEN}docs: update installation instructions${NC}"
    echo ""
    echo "Your commit message:"
    echo -e "${YELLOW}  $COMMIT_MSG${NC}"
    echo ""
    echo "To bypass this check, use: git commit --no-verify"
    exit 1
fi

exit 0
HOOK_EOF

chmod +x "$HOOKS_DIR/commit-msg"

echo -e "${GREEN}✓ Commit-msg hook installed${NC}"
echo ""

# Summary
echo -e "${BLUE}===================================================${NC}"
echo -e "${GREEN}✓ Git hooks installed successfully!${NC}"
echo -e "${BLUE}===================================================${NC}"
echo ""
echo "Installed hooks:"
echo "  1. pre-commit  - Runs tests before each commit"
echo "  2. commit-msg  - Validates commit message format"
echo ""
echo "To bypass hooks (not recommended):"
echo "  git commit --no-verify"
echo ""
echo "To run tests manually:"
echo "  ./scripts/test-all.sh"
echo "  ./scripts/test-all.sh --fast"
echo ""
