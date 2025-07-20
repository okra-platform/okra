# Test Coverage Report

Generate a comprehensive test coverage report with code metrics.

## Command

```bash
# Run full test coverage with detailed report
gotestsum --format testname ./... -- -coverprofile=coverage.out && \
echo "=== OKRA TEST COVERAGE REPORT ===" && \
echo "" && \
go tool cover -func=coverage.out | grep "total:" | awk '{print "🎯 OVERALL COVERAGE: " $3}' && \
echo "" && \
echo "📊 PACKAGE BREAKDOWN:" && \
go test ./... -coverprofile=temp_coverage.out 2>/dev/null | grep -E "coverage: [0-9.]+%" | sed 's/github.com\/okra-platform\/okra\///g' | awk '{print "  • " $2 ": " $4}' | sort -k2 -nr && \
echo "" && \
echo "📈 CODE METRICS:" && \
PROD_LINES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}') && \
TEST_LINES=$(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}') && \
TOTAL_LINES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}') && \
TEST_FILES=$(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" | wc -l | awk '{print $1}') && \
PROD_FILES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" | wc -l | awk '{print $1}') && \
echo "  • Production code: $PROD_LINES lines ($PROD_FILES files)" && \
echo "  • Test code: $TEST_LINES lines ($TEST_FILES files)" && \
echo "  • Total code: $TOTAL_LINES lines" && \
if [ "$PROD_LINES" -gt 0 ] 2>/dev/null; then \
  RATIO=$(echo "scale=1; $TEST_LINES * 100 / $PROD_LINES" | bc) && \
  echo "  • Test/Prod ratio: ${RATIO}%" && \
  echo "  • Test coverage ratio: 1:$(echo "scale=1; $PROD_LINES / $TEST_LINES" | bc)" ; \
fi && \
echo "" && \
echo "🧪 TEST EXECUTION:" && \
TEST_COUNT=$(gotestsum --format dots ./... 2>&1 | grep -E "DONE|RUN" | grep DONE | awk '{print $2}') && \
SKIP_COUNT=$(gotestsum --format dots ./... 2>&1 | grep -c "SKIP" || echo "0") && \
echo "  • Tests run: ${TEST_COUNT:-"N/A"}" && \
echo "  • Tests skipped: $SKIP_COUNT" && \
echo "  • Status: ✅ All tests passing" && \
echo "" && \
echo "🎯 TOP PERFORMERS (>90%):" && \
go tool cover -func=coverage.out | awk '$3 >= 90.0 && $3 != "100.0%" {gsub(/github.com\/okra-platform\/okra\//, "", $1); print "  • " $1 ": " $3}' | head -5 && \
echo "" && \
echo "⚠️  NEEDS ATTENTION (<70%):" && \
go tool cover -func=coverage.out | awk '$3 < 70.0 && $3 != "0.0%" {gsub(/github.com\/okra-platform\/okra\//, "", $1); print "  • " $1 ": " $3}' | head -5 && \
rm -f temp_coverage.out coverage.out
```

## Quick Coverage Check

For a fast coverage overview without running all tests:

```bash
# Quick coverage check (uses cached results if available)
./.claude/scripts/quick-metrics.sh
```

## Coverage with HTML Report

To generate an HTML coverage report you can open in a browser:

```bash
# Generate HTML coverage report
gotestsum ./... -- -coverprofile=coverage.out && \
go tool cover -html=coverage.out -o coverage.html && \
echo "📊 Coverage report generated: coverage.html" && \
echo "Open with: open coverage.html" && \
rm coverage.out
```

## Package-Specific Coverage

To check coverage for a specific package:

```bash
# Replace 'internal/serve' with your target package
go test ./internal/serve -coverprofile=serve_coverage.out && \
go tool cover -func=serve_coverage.out && \
rm serve_coverage.out
```