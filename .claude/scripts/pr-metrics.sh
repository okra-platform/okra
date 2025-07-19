#!/bin/bash
# pr-metrics.sh - Get PR-specific metrics for code review

echo "=== PR Metrics ==="
echo "Files changed: $(git diff --name-only origin/main | grep -E "\.go$" | wc -l)"
echo "Lines changed: $(git diff --stat origin/main | tail -1)"
echo "Test files: $(git diff --name-only origin/main | grep "_test\.go$" | wc -l)"

# Try to get coverage before changes (with error handling)
echo "Calculating coverage before changes..."
git stash push -q
if go test ./... -cover -short -timeout=2m > /tmp/coverage_before.out 2>&1; then
    COVERAGE_BEFORE=$(grep -E "^ok" /tmp/coverage_before.out | awk '{sum+=$5; count++} END {if(count>0) print sum/count "%"; else print "N/A"}')
    echo "Coverage before: $COVERAGE_BEFORE"
else
    echo "Coverage before: N/A (test failed)"
fi
git stash pop -q
rm -f /tmp/coverage_before.out

# Show which packages are affected
echo ""
echo "Affected packages:"
git diff --name-only origin/main | grep -E "\.go$" | xargs dirname | sort | uniq | sed 's/^/  - /'