#!/bin/bash
# test-runner.sh - Run tests with appropriate timeouts and options

# Default timeout (10 minutes)
TIMEOUT=${1:-"10m"}
MODE=${2:-"full"} # full, race, short

echo "=== Running Tests (timeout: $TIMEOUT, mode: $MODE) ==="

# Determine test flags based on mode
case $MODE in
    "race")
        FLAGS="-race -count=1"
        ;;
    "short")
        FLAGS="-short"
        ;;
    "coverage")
        FLAGS="-coverprofile=coverage.out"
        ;;
    *)
        FLAGS=""
        ;;
esac

# Use timeout command if available
if command -v gtimeout &> /dev/null; then
    TIMEOUT_CMD="gtimeout $(echo $TIMEOUT | sed 's/m/*60/' | bc)"
elif command -v timeout &> /dev/null; then
    TIMEOUT_CMD="timeout $(echo $TIMEOUT | sed 's/m/*60/' | bc)"
else
    TIMEOUT_CMD=""
fi

# Run tests
if [ -n "$TIMEOUT_CMD" ]; then
    $TIMEOUT_CMD go test ./... -timeout=$TIMEOUT $FLAGS
else
    go test ./... -timeout=$TIMEOUT $FLAGS
fi

EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ All tests passed"
elif [ $EXIT_CODE -eq 124 ] || [ $EXIT_CODE -eq 137 ]; then
    echo "⏱️  Tests timed out after $TIMEOUT"
else
    echo "❌ Tests failed with exit code: $EXIT_CODE"
fi

exit $EXIT_CODE