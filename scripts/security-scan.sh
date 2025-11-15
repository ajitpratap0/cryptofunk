#!/bin/bash

# Security scanning script for CryptoFunk
# Performs automated security checks based on OWASP guidelines

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=================================================="
echo "CryptoFunk Security Scan"
echo "=================================================="
echo ""

PASS_COUNT=0
FAIL_COUNT=0
WARN_COUNT=0

# Function to print test results
pass() {
    echo -e "${GREEN}✅ PASS${NC}: $1"
    ((PASS_COUNT++))
}

fail() {
    echo -e "${RED}❌ FAIL${NC}: $1"
    ((FAIL_COUNT++))
}

warn() {
    echo -e "${YELLOW}⚠️  WARN${NC}: $1"
    ((WARN_COUNT++))
}

echo "1. Checking for hardcoded secrets..."
if grep -r -i "password.*=" --include="*.go" --include="*.yaml" . | grep -v "// " | grep -v "POSTGRES_PASSWORD" | grep -v "Password string" | grep -v "testing" | grep -q .; then
    fail "Found potential hardcoded passwords"
    grep -r -i "password.*=" --include="*.go" --include="*.yaml" . | grep -v "// " | grep -v "POSTGRES_PASSWORD" | grep -v "Password string" | grep -v "testing" | head -5
else
    pass "No hardcoded passwords found"
fi

echo ""
echo "2. Checking for API keys in code..."
if grep -r -E "(api_key|apikey|api-key).*=.*['\"][a-zA-Z0-9]{20,}" --include="*.go" --include="*.yaml" . | grep -v "// " | grep -v "api_key string" | grep -q .; then
    fail "Found potential hardcoded API keys"
else
    pass "No hardcoded API keys found"
fi

echo ""
echo "3. Checking for SQL injection vulnerabilities..."
if grep -r "Exec\|Query" --include="*.go" . | grep -v "ctx," | grep -v "// " | grep "+" | grep -q .; then
    warn "Found potential SQL concatenation (review manually)"
else
    pass "No obvious SQL concatenation found"
fi

echo ""
echo "4. Checking for unsafe error handling..."
if grep -r "panic(" --include="*.go" . | grep -v "// " | grep -v "_test.go" | grep -q .; then
    warn "Found panic() calls (should use error returns)"
    grep -r "panic(" --include="*.go" . | grep -v "// " | grep -v "_test.go" | head -3
else
    pass "No unsafe panic() calls in production code"
fi

echo ""
echo "5. Checking for default credentials in docker-compose..."
if grep -E "(POSTGRES_PASSWORD|GRAFANA_ADMIN_PASSWORD|JWT_SECRET):-" docker-compose.yml | grep -q "changeme\|password\|admin\|secret123"; then
    fail "Found weak default credentials in docker-compose.yml"
else
    pass "No weak default credentials in docker-compose.yml"
fi

echo ""
echo "6. Checking for TLS/SSL configuration..."
if grep -r "sslmode=disable" --include="*.go" --include="*.yaml" . | grep -v "development" | grep -q .; then
    warn "Found sslmode=disable in non-development context"
else
    pass "SSL mode configured appropriately"
fi

echo ""
echo "7. Checking for verbose error messages..."
if grep -r "Error().*return.*err" --include="*.go" . | grep -v "_test.go" | wc -l | awk '{if ($1 > 50) print "many"; else print "few"}' | grep -q "many"; then
    warn "Many error messages may expose internal details (review manually)"
else
    pass "Error handling appears reasonable"
fi

echo ""
echo "8. Checking for proper input validation..."
if grep -r "json.Unmarshal\|json.Decoder" --include="*.go" . | wc -l | awk '{if ($1 > 0) print "found"}' | grep -q "found"; then
    if grep -r "Validate(" --include="*.go" . | wc -l | awk '{if ($1 > 5) print "many"; else print "few"}' | grep -q "many"; then
        pass "Input validation functions found"
    else
        warn "JSON unmarshaling found but limited validation (review manually)"
    fi
else
    pass "No JSON unmarshaling found (N/A)"
fi

echo ""
echo "9. Checking for secrets in environment variables..."
if env | grep -i "password\|secret\|key" | grep -v "PATH\|HOME" | grep -q .; then
    warn "Secrets found in environment variables (expected for development)"
else
    pass "No secrets in current environment"
fi

echo ""
echo "10. Checking Go vulnerability database..."
if command -v govulncheck &> /dev/null; then
    if govulncheck ./... 2>&1 | grep -q "No vulnerabilities found"; then
        pass "No known vulnerabilities in dependencies"
    else
        warn "Vulnerabilities found (see govulncheck output)"
        govulncheck ./... | grep -A 5 "Vulnerability"
    fi
else
    warn "govulncheck not installed (install with: go install golang.org/x/vuln/cmd/govulncheck@latest)"
fi

echo ""
echo "11. Checking for gosec security issues..."
if command -v gosec &> /dev/null; then
    if gosec -quiet ./... 2>&1 | grep -q "Issues : 0"; then
        pass "No gosec security issues found"
    else
        warn "gosec found potential issues (review manually)"
        gosec -quiet ./... | grep -A 2 "Summary:"
    fi
else
    warn "gosec not installed (install with: go install github.com/securego/gosec/v2/cmd/gosec@latest)"
fi

echo ""
echo "12. Checking for exposed debug endpoints..."
if grep -r "pprof" --include="*.go" . | grep -v "// " | grep -v "_test.go" | grep -q .; then
    warn "pprof endpoints found (ensure disabled in production)"
    grep -r "pprof" --include="*.go" . | grep -v "// " | grep -v "_test.go" | head -3
else
    pass "No pprof endpoints found"
fi

echo ""
echo "13. Checking for non-root Docker users..."
if grep -r "USER" --include="Dockerfile*" deployments/docker/ | grep -q "USER"; then
    pass "Non-root USER directive found in Dockerfiles"
else
    fail "No USER directive in Dockerfiles (containers run as root)"
fi

echo ""
echo "14. Checking Kubernetes security context..."
if grep -r "securityContext" --include="*.yaml" deployments/k8s/ | grep -q "runAsNonRoot"; then
    pass "Security context configured in Kubernetes manifests"
else
    fail "No securityContext found in Kubernetes manifests"
fi

echo ""
echo "15. Checking for network policies..."
if find deployments/k8s/ -name "*network-policy*" -o -name "*netpol*" | grep -q .; then
    pass "Network policy files found"
else
    fail "No network policy files found"
fi

echo ""
echo "=================================================="
echo "Security Scan Summary"
echo "=================================================="
echo -e "${GREEN}PASS${NC}: $PASS_COUNT"
echo -e "${YELLOW}WARN${NC}: $WARN_COUNT"
echo -e "${RED}FAIL${NC}: $FAIL_COUNT"
echo ""

if [ $FAIL_COUNT -gt 0 ]; then
    echo -e "${RED}Security scan FAILED. Please address the issues above.${NC}"
    exit 1
elif [ $WARN_COUNT -gt 5 ]; then
    echo -e "${YELLOW}Security scan PASSED with warnings. Review warnings before production deployment.${NC}"
    exit 0
else
    echo -e "${GREEN}Security scan PASSED.${NC}"
    exit 0
fi
