#!/usr/bin/env bash
# Purpose
#   Run merged Go test coverage and enforce a minimum total (COVERAGE_MIN).
# Usage
#   ./scripts/coverage-check.sh
#   COVERAGE_MIN=90 COVERAGE_PKGS=./... ./scripts/coverage-check.sh
# Options
#   COVERAGE_MIN   — fail if merged total % is below this (default: 90 if unset; Makefile sets 55).
#   COVERAGE_GOAL  — informational project target (default: 90); warns if below.
#   COVERAGE_PKGS  — comma-separated packages for -coverpkg (default: ./...).
#                    The Makefile defaults to all packages except phaudittrailv0.
# What It Reads
#   go.mod (optional `toolchain` line sets GOTOOLCHAIN for consistent builds).
# What It Affects / Does
#   Writes coverage.out in the repo root; prints total % and pass/fail.
# Exit Behavior
#   0 if merged coverage >= COVERAGE_MIN; 1 if below COVERAGE_MIN or on error.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if grep -q '^toolchain ' go.mod 2>/dev/null; then
  export GOTOOLCHAIN
  GOTOOLCHAIN="$(awk '/^toolchain /{print $2; exit}' go.mod)"
fi

COVERAGE_MIN="${COVERAGE_MIN:-90}"
COVERAGE_GOAL="${COVERAGE_GOAL:-90}"
COVERAGE_PKGS="${COVERAGE_PKGS:-./...}"

go test -count=1 -short \
  -coverprofile=coverage.out \
  -covermode=atomic \
  -coverpkg="${COVERAGE_PKGS}" \
  ./...

pct="$(go tool cover -func=coverage.out | awk '/^total:/ {
  for (i = 1; i <= NF; i++) {
    if ($i ~ /^[0-9]+(\.[0-9]+)?%$/) {
      gsub(/%/, "", $i)
      print $i
      exit
    }
  }
}')"

if [[ -z "${pct}" ]]; then
  echo "coverage-check: could not parse total from go tool cover -func" >&2
  exit 1
fi

echo "coverage-check: merged statements=${pct}% (min=${COVERAGE_MIN}% goal=${COVERAGE_GOAL}%)"

if awk -v p="${pct}" -v min="${COVERAGE_MIN}" 'BEGIN { exit !(p + 0 < min + 0) }'; then
  echo "coverage-check: FAIL — ${pct}% is below COVERAGE_MIN=${COVERAGE_MIN}%" >&2
  exit 1
fi

if awk -v p="${pct}" -v goal="${COVERAGE_GOAL}" 'BEGIN { exit !(p + 0 < goal + 0) }'; then
  echo "coverage-check: PASS min gate; note: still below COVERAGE_GOAL=${COVERAGE_GOAL}% (project target)" >&2
else
  echo "coverage-check: PASS (meets COVERAGE_GOAL)" >&2
fi
