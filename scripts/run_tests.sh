#!/usr/bin/env bash
# run_tests.sh — Run all unit tests for paycloudhelper from repository root.
# Usage: ./scripts/run_tests.sh [options]
# Options: -v verbose, -race run with race detector, -cover show coverage, -coverprofile write coverage file

set -e

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

VERBOSE=""
RACE=""
COVER=""
SHORT=""
COVERPROFILE=""

for arg in "$@"; do
  case "$arg" in
    -v|--verbose)   VERBOSE="-v" ;;
    -race)          RACE="-race" ;;
    -cover)         COVER="-cover" ;;
    -coverprofile)  COVERPROFILE="-coverprofile=coverage.out -covermode=atomic" ;;
    -short)         SHORT="-short" ;;
    -h|--help)
      echo "Usage: $0 [-v|--verbose] [-race] [-cover] [-coverprofile] [-short]"
      echo "  -v, --verbose   Verbose test output"
      echo "  -race           Run tests with race detector"
      echo "  -cover          Print coverage per package"
      echo "  -coverprofile   Write coverage.out and show summary (go tool cover -func)"
      echo "  -short          Skip long-running tests"
      exit 0
      ;;
  esac
done

echo "Running tests in $ROOT_DIR (go test ./...)..."
go test ./... $VERBOSE $RACE $COVER $COVERPROFILE $SHORT

if [ -n "$COVERPROFILE" ] && [ -f coverage.out ]; then
  echo ""
  echo "Coverage summary:"
  go tool cover -func=coverage.out
fi

echo "Tests completed successfully."
