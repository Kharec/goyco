#!/bin/bash

set -e

COVERAGE_DIR="coverage"
PACKAGE="${1:-./internal/e2e/...}"

mkdir -p "$COVERAGE_DIR"

echo "Running tests with coverage for: $PACKAGE"
go test -v -coverprofile="$COVERAGE_DIR/coverage.out" -covermode=atomic "$PACKAGE"

echo "Generating HTML coverage report..."
go tool cover -html="$COVERAGE_DIR/coverage.out" -o "$COVERAGE_DIR/coverage.html"

echo "Generating coverage summary..."
go tool cover -func="$COVERAGE_DIR/coverage.out" | tee "$COVERAGE_DIR/coverage.txt"

echo ""
echo "Coverage report generated:"
echo "  HTML: $COVERAGE_DIR/coverage.html"
echo "  Text: $COVERAGE_DIR/coverage.txt"
echo "  Raw:  $COVERAGE_DIR/coverage.out"

