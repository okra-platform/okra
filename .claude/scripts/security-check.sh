#!/bin/bash
# security-check.sh - Run security checks on Go code

echo "=== Security Check ==="

# Check if gosec is installed
if ! command -v gosec &> /dev/null; then
    echo "gosec not installed. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"
    exit 1
fi

# Run gosec
echo "Running gosec security scanner..."
if gosec -fmt=json -out=security-report.json ./... 2>/dev/null; then
    # Parse results
    if [ -f security-report.json ]; then
        ISSUES=$(cat security-report.json | grep -o '"Issues":\[[^]]*\]' | grep -o '\[.*\]' | tr -d '[]')
        if [ -z "$ISSUES" ] || [ "$ISSUES" = "" ]; then
            echo "✅ No security issues found"
        else
            echo "⚠️  Security issues found. See security-report.json for details"
            # Show summary
            cat security-report.json | grep -o '"rule_id":"[^"]*"' | sort | uniq -c | sort -nr
        fi
    fi
else
    echo "❌ Security scan failed"
fi

# Additional security checks
echo ""
echo "Checking for common security patterns..."

# Check for hardcoded credentials
echo -n "Hardcoded credentials: "
if grep -r -E '(password|secret|key|token)\s*[:=]\s*"[^"]+"' . --include="*.go" --exclude-dir=vendor --exclude-dir=.git --exclude="*_test.go" 2>/dev/null | grep -v "// " > /tmp/creds_check.txt; then
    COUNT=$(wc -l < /tmp/creds_check.txt)
    if [ $COUNT -gt 0 ]; then
        echo "⚠️  Found $COUNT potential instances"
        head -5 /tmp/creds_check.txt
    else
        echo "✅ None found"
    fi
else
    echo "✅ None found"
fi
rm -f /tmp/creds_check.txt

# Check for SQL injection risks
echo -n "SQL injection risks: "
if grep -r -E 'fmt\.Sprintf.*"(SELECT|INSERT|UPDATE|DELETE)' . --include="*.go" --exclude-dir=vendor --exclude-dir=.git 2>/dev/null > /tmp/sql_check.txt; then
    COUNT=$(wc -l < /tmp/sql_check.txt)
    if [ $COUNT -gt 0 ]; then
        echo "⚠️  Found $COUNT potential instances"
        head -5 /tmp/sql_check.txt
    else
        echo "✅ None found"
    fi
else
    echo "✅ None found"
fi
rm -f /tmp/sql_check.txt