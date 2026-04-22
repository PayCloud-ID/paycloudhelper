#!/usr/bin/env bash
# run_tests.sh
#
# Purpose:
# - Run Go test suites for the full paycloudhelper module from repository root.
# - Provide simple flags for verbose output, race detector, short mode, and coverage modes.
#
# Usage:
# - ./scripts/run_tests.sh [options]
#
# Options:
# - -v, --verbose     Enable verbose go test output.
# - -race             Enable race detector.
# - -cover            Enable per-package coverage in test output.
# - -coverprofile     Generate coverage.out with atomic mode and print summary.
# - -short            Pass -short to skip long-running tests.
# - -h, --help        Show usage.
#
# What It Reads:
# - Go module metadata and package sources under repository root.
# - Existing CLI arguments.
#
# What It Affects / Does:
# - Executes go test ./... with selected flags.
# - May create/overwrite coverage.out when -coverprofile is provided.
# - Prints coverage summary via go tool cover when coverage.out exists.
#
# Exit Behavior:
# - Non-zero if any tests fail or go tool commands fail.

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
