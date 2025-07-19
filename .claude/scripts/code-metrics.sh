#!/bin/bash
# code-metrics.sh - Generate comprehensive code metrics for OKRA project

echo "=== OKRA Code Metrics Report ==="
echo "Generated: $(date)"
echo ""

# Test Coverage with increased timeout
echo "### Test Coverage ###"
echo "Running tests with coverage (this may take a few minutes)..."

# Use timeout command with 10 minutes (600 seconds) instead of default 2 minutes
if command -v gtimeout &> /dev/null; then
    # macOS with GNU coreutils
    TIMEOUT_CMD="gtimeout 600"
elif command -v timeout &> /dev/null; then
    # Linux
    TIMEOUT_CMD="timeout 600"
else
    # No timeout command available
    TIMEOUT_CMD=""
fi

# Run tests with coverage
if [ -n "$TIMEOUT_CMD" ]; then
    $TIMEOUT_CMD go test ./... -coverprofile=coverage.out -timeout=10m > test_output.tmp 2>&1
else
    go test ./... -coverprofile=coverage.out -timeout=10m > test_output.tmp 2>&1
fi

TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -eq 0 ]; then
    go tool cover -func=coverage.out | tail -1 | awk '{print "Total Coverage: " $3}'
    echo ""
    
    # Per-package coverage
    echo "Package Coverage Breakdown:"
    go test ./... -cover -timeout=10m 2>/dev/null | grep -E "^ok|^FAIL" | awk '{print $2 ": " $5}' | sort
else
    echo "Tests failed or timed out. Exit code: $TEST_EXIT_CODE"
    echo "Attempting quick coverage calculation..."
    go test ./... -cover -short -timeout=5m 2>/dev/null | grep -E "^ok" | awk '{sum+=$5; count++} END {if(count>0) print "Approximate Coverage: " sum/count "%"; else print "Coverage: N/A"}'
fi

# Clean up
rm -f test_output.tmp

echo ""
echo "### Code Statistics ###"
echo "Total Go files: $(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | wc -l | xargs)"
echo "Total lines: $(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')"
echo "Production code: $(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}') lines"
echo "Test code: $(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}') lines"

# Calculate ratio
PROD_LINES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
TEST_LINES=$(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
if [ "$PROD_LINES" -gt 0 ] 2>/dev/null; then
    RATIO=$(echo "scale=2; $TEST_LINES * 100 / $PROD_LINES" | bc)
    echo "Test/Prod ratio: ${RATIO}%"
fi

echo ""
echo "### Top 10 Largest Files ###"
find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -exec wc -l {} + 2>/dev/null | sort -nr | head -11 | tail -10

echo ""
echo "### Test File Distribution ###"
echo "Packages with tests: $(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" -type f -exec dirname {} \; | sort | uniq | wc -l | xargs)"
echo "Packages without tests: $(comm -23 <(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" -type f -exec dirname {} \; | sort | uniq) <(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" -type f -exec dirname {} \; | sort | uniq) 2>/dev/null | wc -l | xargs)"

# Clean up coverage file if user doesn't want to keep it
if [ "$1" != "--keep-coverage" ]; then
    rm -f coverage.out
fi