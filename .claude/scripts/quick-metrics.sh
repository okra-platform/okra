#!/bin/bash
# quick-metrics.sh - Get instant code metrics without running full tests

echo "=== Quick Code Metrics ==="

# Try to get coverage from most recent test run or run quick tests
if [ -f "coverage.out" ] && [ "$(find coverage.out -mmin -60 2>/dev/null)" ]; then
    echo "Using cached coverage data (less than 60 minutes old)"
    go tool cover -func=coverage.out | tail -1 | awk '{print "Coverage: " $3}'
else
    echo "Running quick test coverage..."
    go test ./... -cover -short -timeout=2m 2>/dev/null | grep -E "^ok" | awk '{sum+=$5; count++} END {if(count>0) print "Coverage: " sum/count "%"; else print "Coverage: N/A"}'
fi

# Code statistics
echo "Total lines: $(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')"
echo "Prod code: $(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}') lines"
echo "Test code: $(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}') lines"

# Calculate ratio
PROD_LINES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
TEST_LINES=$(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
if [ "$PROD_LINES" -gt 0 ] 2>/dev/null; then
    RATIO=$(echo "scale=1; $TEST_LINES * 100 / $PROD_LINES" | bc)
    echo "Test/Prod ratio: ${RATIO}%"
fi