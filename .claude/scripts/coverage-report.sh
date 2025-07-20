#!/bin/bash
# coverage-report.sh - Generate comprehensive test coverage report

set -e

echo "🧪 Running test suite with coverage..."
echo "=================================="

# Run tests with coverage
gotestsum --format testname ./... -- -coverprofile=coverage.out

echo ""
echo "📊 OKRA TEST COVERAGE REPORT"
echo "============================"
echo ""

# Overall coverage
OVERALL_COVERAGE=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}')
echo "🎯 OVERALL COVERAGE: $OVERALL_COVERAGE"
echo ""

# Package breakdown
echo "📈 PACKAGE BREAKDOWN:"
echo "--------------------"
go test ./... -coverprofile=temp_coverage.out 2>/dev/null | \
  grep -E "coverage: [0-9.]+%" | \
  sed 's/github.com\/okra-platform\/okra\///g' | \
  awk '{
    package = $2
    coverage = $4
    gsub(/internal\//, "", package)
    printf "  %-20s %s\n", package ":", coverage
  }' | \
  sort -k2 -nr

echo ""

# Code metrics (raw file counts)
echo "📊 CODE METRICS (Raw File Counts):"
echo "---------------------------------"
PROD_LINES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
TEST_LINES=$(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
TOTAL_LINES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
TEST_FILES=$(find . -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*" | wc -l | awk '{print $1}')
PROD_FILES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" | wc -l | awk '{print $1}')

echo "  Production code:   $PROD_LINES lines ($PROD_FILES files)"
echo "  Test code:         $TEST_LINES lines ($TEST_FILES files)"
echo "  Total code:        $TOTAL_LINES lines"

if [ "$PROD_LINES" -gt 0 ] 2>/dev/null; then
  RATIO=$(echo "scale=1; $TEST_LINES * 100 / $PROD_LINES" | bc 2>/dev/null || echo "N/A")
  TEST_COVERAGE_RATIO=$(echo "scale=1; $PROD_LINES / $TEST_LINES" | bc 2>/dev/null || echo "N/A")
  echo "  Test/Prod ratio:   ${RATIO}%"
  echo "  Coverage ratio:    1:${TEST_COVERAGE_RATIO}"
fi

echo ""

# Production code breakdown
echo "📂 PRODUCTION CODE BREAKDOWN:"
echo "-----------------------------"
GENERATED_LINES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -name "*_test.go" | xargs grep -l "Code generated\|DO NOT EDIT" 2>/dev/null | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
FIXTURE_LINES=$(find . -path "*/testdata/*" -name "*.go" -o -path "*/fixtures/*" -name "*.go" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
ACTUAL_PROD=$((PROD_LINES - GENERATED_LINES - FIXTURE_LINES))
echo "  Actual production: ~$ACTUAL_PROD lines"
echo "  Generated code:    ~$GENERATED_LINES lines"
echo "  Test fixtures:     ~$FIXTURE_LINES lines"
echo "  Total non-test:    $PROD_LINES lines"

echo ""

# Test execution stats
echo "🧪 TEST EXECUTION:"
echo "-----------------"
TEST_OUTPUT=$(gotestsum --format dots ./... 2>&1)
TEST_COUNT=$(echo "$TEST_OUTPUT" | grep -E "DONE" | tail -1 | awk '{print $2}' || echo "N/A")
SKIP_COUNT=$(echo "$TEST_OUTPUT" | grep -c "SKIP" || echo "0")
echo "  Tests run:         $TEST_COUNT"
echo "  Tests skipped:     $SKIP_COUNT"
echo "  Status:            ✅ All tests passing"

echo ""

# Top performers
echo "🏆 TOP PERFORMERS (≥90%):"
echo "-------------------------"
if [ -f coverage.out ]; then
  go tool cover -func=coverage.out | \
    awk '$3 >= 90.0 && $3 != "0.0%" && !/total:/ {
      gsub(/github.com\/okra-platform\/okra\//, "", $1)
      gsub(/internal\//, "", $1)
      printf "  %-30s %s\n", $1, $3
    }' | \
    head -8
else
  echo "  (Coverage data not available)"
fi

echo ""

# Needs attention
echo "⚠️  NEEDS ATTENTION (<70%):"
echo "---------------------------"
if [ -f coverage.out ]; then
  go tool cover -func=coverage.out | \
    awk '$3 < 70.0 && $3 != "0.0%" && !/total:/ {
      gsub(/github.com\/okra-platform\/okra\//, "", $1)
      gsub(/internal\//, "", $1)
      printf "  %-30s %s\n", $1, $3
    }' | \
    head -8
else
  echo "  (Coverage data not available)"
fi

echo ""

# Recent changes (if in git repo)
if git rev-parse --git-dir > /dev/null 2>&1; then
  echo "📋 RECENT TEST CHANGES:"
  echo "----------------------"
  RECENT_TEST_FILES=$(git log --oneline --since="7 days ago" --name-only | grep "_test.go" | sort | uniq | head -5)
  if [ -n "$RECENT_TEST_FILES" ]; then
    echo "$RECENT_TEST_FILES" | sed 's/^/  • /'
  else
    echo "  • No test files modified in last 7 days"
  fi
fi

# Cleanup
rm -f coverage.out temp_coverage.out

echo ""
echo "✅ Coverage report complete!"
echo ""
echo "💡 TIP: Run 'go tool cover -html=coverage.out -o coverage.html' for visual report"