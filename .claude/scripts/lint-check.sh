#!/bin/bash
# lint-check.sh - Run linting and formatting checks

echo "=== Linting and Formatting Check ==="

# Check formatting
echo "Checking formatting..."
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo "❌ The following files need formatting:"
    echo "$UNFORMATTED" | sed 's/^/  - /'
    echo "Run: go fmt ./..."
else
    echo "✅ All files properly formatted"
fi

# Run go vet
echo ""
echo "Running go vet..."
if go vet ./... 2>&1; then
    echo "✅ go vet passed"
else
    echo "❌ go vet found issues"
fi

# Run golangci-lint if available
if command -v golangci-lint &> /dev/null; then
    echo ""
    echo "Running golangci-lint..."
    if golangci-lint run --timeout=5m; then
        echo "✅ golangci-lint passed"
    else
        echo "❌ golangci-lint found issues"
    fi
else
    echo ""
    echo "ℹ️  golangci-lint not installed. Install with:"
    echo "    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s latest"
fi

# Check for common issues
echo ""
echo "Checking for common issues..."

# Ignored errors
echo -n "Ignored errors (_, err :=): "
COUNT=$(grep -r "_, err :=" . --include="*.go" --exclude-dir=vendor --exclude-dir=.git | wc -l)
if [ $COUNT -gt 0 ]; then
    echo "⚠️  Found $COUNT instances"
else
    echo "✅ None found"
fi

# TODOs without owners
echo -n "TODOs without owners: "
COUNT=$(grep -r "TODO" . --include="*.go" --exclude-dir=vendor --exclude-dir=.git | grep -v "@" | wc -l)
if [ $COUNT -gt 0 ]; then
    echo "⚠️  Found $COUNT instances"
else
    echo "✅ None found"
fi