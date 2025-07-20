# Test Coverage Report

Generate a comprehensive test coverage report with code metrics.

## Command

```bash
# Run full test coverage with detailed report
./.claude/scripts/coverage-report.sh
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