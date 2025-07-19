#!/bin/bash
# quick-check.sh - Quick quality check for code review

echo "=== Quick Quality Check ==="

# Format check
echo "Running go fmt..."
if [ -z "$(gofmt -l .)" ]; then
    echo "✅ Code is properly formatted"
else
    echo "❌ Code needs formatting. Run: go fmt ./..."
fi

# Vet check
echo ""
echo "Running go vet..."
if go vet ./... 2>&1; then
    echo "✅ go vet passed"
else
    echo "❌ go vet found issues"
fi

# Quick test with race detector
echo ""
echo "Running quick tests with race detector..."
if go test -short -race ./... -timeout=2m > /dev/null 2>&1; then
    echo "✅ Quick tests passed"
else
    echo "❌ Quick tests failed"
fi