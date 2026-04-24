#!/usr/bin/env bash
# Purpose
#   Produce a merged coverage profile and a short inventory (total %, 0% functions
#   sample, coarse grouping by path prefix).
# Usage
#   ./scripts/coverage-inventory.sh
#   COVERAGE_PKGS=./... ./scripts/coverage-inventory.sh
# Options
#   COVERAGE_PKGS — comma-separated packages for -coverpkg (default ./...).
# What It Reads
#   go.mod (optional toolchain line sets GOTOOLCHAIN).
# What It Affects / Does
#   Writes coverage.out in the repo root; prints inventory to stdout.
# Exit Behavior
#   0 on success; non-zero if go test or go tool cover fails.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if grep -q '^toolchain ' go.mod 2>/dev/null; then
  export GOTOOLCHAIN
  GOTOOLCHAIN="$(awk '/^toolchain /{print $2; exit}' go.mod)"
fi

COVERAGE_PKGS="${COVERAGE_PKGS:-./...}"

go test -count=1 -short \
  -coverprofile=coverage.out \
  -covermode=atomic \
  -coverpkg="${COVERAGE_PKGS}" \
  ./...

go tool cover -func=coverage.out > coverage-func.txt

echo "=== Merged statement coverage (total) ==="
grep '^total:' coverage-func.txt || true

echo ""
echo "=== Sample of functions at exactly 0.0% (first 40 lines) ==="
awk 'BEGIN{FS="\t"} NF>=2 && $NF=="0.0%" {print}' coverage-func.txt | head -40 || true

echo ""
echo "=== Unique .go files with at least one 0% function (first 50) ==="
awk '
BEGIN { FS="\t" }
/^total:/ { next }
NF >= 2 {
  f = $1
  sub(/:[0-9]+:$/, "", f)
  if (f != "" && seen[f]++ == 0) print f
}
' coverage-func.txt | head -50

echo ""
echo "=== Row counts by top-level segment after module path ==="
awk '
BEGIN { FS="\t" }
/^total:/ { next }
NF >= 2 {
  f = $1
  sub(/:[0-9]+:$/, "", f)
  n = split(f, a, "/")
  key = (n >= 4 ? a[4] : f)
  for (i = 5; i <= n && i <= 7; i++) key = key "/" a[i]
  c[key]++
}
END {
  for (k in c) print c[k], k
}' coverage-func.txt | sort -nr | head -30

echo ""
echo "Full function list: coverage-func.txt"
