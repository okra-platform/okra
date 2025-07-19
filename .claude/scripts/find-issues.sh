#!/bin/bash
# find-issues.sh - Find common code issues quickly

echo "=== Common Issues Check ==="

# Check for missing error handling
echo "Checking for ignored errors..."
IGNORED_ERRORS=$(grep -r "_, err :=" . --include="*.go" --exclude-dir=vendor --exclude-dir=.git --exclude="*_test.go" | wc -l)
if [ $IGNORED_ERRORS -gt 0 ]; then
    echo "⚠️  Found $IGNORED_ERRORS ignored errors"
    grep -r "_, err :=" . --include="*.go" --exclude-dir=vendor --exclude-dir=.git --exclude="*_test.go" | head -5
else
    echo "✅ No ignored errors found"
fi

# Find TODOs without owners
echo ""
echo "Checking for TODOs without owners..."
TODOS=$(grep -r "TODO" . --include="*.go" --exclude-dir=vendor --exclude-dir=.git | grep -v "@" | wc -l)
if [ $TODOS -gt 0 ]; then
    echo "⚠️  Found $TODOS TODOs without owners"
    grep -r "TODO" . --include="*.go" --exclude-dir=vendor --exclude-dir=.git | grep -v "@" | head -5
else
    echo "✅ All TODOs have owners"
fi

# Check for complex functions
if command -v gocyclo &> /dev/null; then
    echo ""
    echo "Checking for complex functions..."
    COMPLEX=$(gocyclo -over 10 . 2>/dev/null | wc -l)
    if [ $COMPLEX -gt 0 ]; then
        echo "⚠️  Found $COMPLEX functions with cyclomatic complexity >10"
        gocyclo -over 10 . 2>/dev/null | head -5
    else
        echo "✅ No overly complex functions"
    fi
fi

# Memory leak indicators
echo ""
echo "Checking for potential memory leaks..."
LEAKS=$(grep -r "go func()" . --include="*.go" --exclude-dir=vendor --exclude-dir=.git | grep -v "Done()" | wc -l)
if [ $LEAKS -gt 0 ]; then
    echo "⚠️  Found $LEAKS goroutines without obvious termination"
else
    echo "✅ No obvious goroutine leaks"
fi