#!/usr/bin/env bash
# run.sh
#
# Purpose:
# - Convenience entrypoint for this library repository to run unit tests with race detection.
# - Provides an optional debug mode that enables Go scheduler/debug behavior flags.
#
# Usage:
# - ./run.sh [--debug] [--help]
#
# Options:
# - --debug    Sets GODEBUG=asyncpreemptoff=0 before running tests.
# - -h, --help Show usage.
#
# What It Reads:
# - Current repository Go module/packages via go test.
# - CLI arguments.
#
# What It Affects / Does:
# - Executes: go test -race ./...
# - Replaces current shell process with go test via exec.
# - In debug mode, exports GODEBUG for this process.
#
# Exit Behavior:
# - Non-zero when tests fail or invalid arguments are supplied.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

DEBUG=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --debug) DEBUG=true; shift ;;
    -h|--help)
      echo "Usage: $0 [--debug] [--help]"
      echo "  Library repo: runs go test -race ./... (optionally GODEBUG with --debug)"
      exit 0
      ;;
    *) echo "ERROR: unknown option: $1" >&2; exit 1 ;;
  esac
done

echo "Running unit tests (library module, race detector)..."
if [[ "$DEBUG" == "true" ]]; then
  export GODEBUG=asyncpreemptoff=0
fi
exec go test -race ./...
