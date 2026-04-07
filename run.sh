#!/usr/bin/env bash
# run.sh — library module (no main): runs tests with race detector.
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
