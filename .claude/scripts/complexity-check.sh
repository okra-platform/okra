#!/bin/bash
# complexity-check.sh - Check code complexity metrics

echo "=== Complexity Analysis ==="

# Check if gocyclo is installed
if ! command -v gocyclo &> /dev/null; then
    echo "gocyclo not installed. Install with: go install github.com/fzipp/gocyclo/cmd/gocyclo@latest"
    echo ""
fi

# Run cyclomatic complexity check
if command -v gocyclo &> /dev/null; then
    echo "Functions with high cyclomatic complexity (>10):"
    gocyclo -over 10 . 2>/dev/null | head -20
    
    COMPLEX_COUNT=$(gocyclo -over 10 . 2>/dev/null | wc -l)
    echo ""
    echo "Total functions with complexity >10: $COMPLEX_COUNT"
    
    if [ $COMPLEX_COUNT -eq 0 ]; then
        echo "✅ No overly complex functions found"
    else
        echo "⚠️  Consider refactoring complex functions"
    fi
fi

# Check for long functions
echo ""
echo "Checking for long functions (>50 lines)..."
find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -exec awk '
    /^func/ { 
        start = NR
        name = $0
        gsub(/^[[:space:]]*/, "", name)
    }
    /^}/ && start { 
        if (NR - start > 50) {
            printf "%s:%d: %s (%d lines)\n", FILENAME, start, name, NR - start
        }
        start = 0
    }
' {} + | head -10

# Check for long files
echo ""
echo "Files with >500 lines:"
find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -exec wc -l {} + | awk '$1 > 500 {print $2 ": " $1 " lines"}' | sort -k3 -nr | head -10