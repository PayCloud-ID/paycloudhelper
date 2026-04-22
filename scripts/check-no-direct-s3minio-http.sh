#!/usr/bin/env bash
# check-no-direct-s3minio-http.sh
#
# Purpose:
# - Enforce architecture boundaries by detecting disallowed direct S3MinIO HTTP/API usage
#   across selected Go repositories.
# - Allows only explicitly approved paths while failing CI/local checks for violations.
#
# Usage:
# - From paycloudhelper root (or a parent directory containing the listed scan roots):
#     ./scripts/check-no-direct-s3minio-http.sh
#
# Options:
# - No CLI options.
#
# What It Reads:
# - Go source files (*.go) under SCAN_ROOTS.
# - Optional tools from PATH:
#   - rg (preferred fast scanner)
#   - grep (fallback scanner)
# - Script constants:
#   - SCAN_ROOTS: repository directories to scan
#   - DENY_PATTERN: forbidden string/regex patterns
#   - ALLOW_PATH_PATTERN: allowlisted file path regex
#
# What It Affects / Does:
# - Read-only scan; does not modify files.
# - Prints violations with file:line references.
# - Exits non-zero when disallowed usage is found.
#
# Exit Behavior:
# - 0: no violations, or only allowlisted matches.
# - 1: at least one disallowed direct usage detected.
set -euo pipefail

SCAN_ROOTS=(
  "./paycloud-be-adminft-manager"
  "./paycloud-be-adminpg-manager"
  "./paycloud-be-clientpg-manager"
  "./paycloud-be-dashboard-manager"
  "./paycloud-be-merchantft-manager"
  "./paycloud-be-merchantpg-manager"
  "./paycloud-be-reporting-manager"
  "./paycloud-be-s3minio-manager"
  "./paycloudhelper"
)

DENY_PATTERN='Http_Minio|/api/v2/download|/api/v2/generate_view_url|/api/v2/download_file|/api/v2/view'
ALLOW_PATH_PATTERN='^(./paycloud-be-s3minio-manager/|./paycloudhelper/sdk/services/s3minio/http/)'

collect_go_files() {
  for root in "${SCAN_ROOTS[@]}"; do
    if [[ -d "$root" ]]; then
      find "$root" \
        -type f -name '*.go' \
        ! -path '*/.git/*' \
        ! -path '*/vendor/*' \
        ! -path '*/bin/*' \
        ! -path '*/download/*' \
        ! -path '*/examples/*' \
        ! -path '*/docs/*' \
        ! -path '*/doc/*' \
        ! -path '*/.agents/*' \
        ! -path '*/.cursor/*' \
        ! -path '*/.github/*' \
        ! -path '*/grpc_client/services_grpc/*' \
        ! -path '*/grpc/pb/*' \
        ! -path '*/app/adapters/protobuf/*' \
        ! -path '*/helminit/*'
    fi
  done
}

MATCHES=""
if command -v rg >/dev/null 2>&1; then
  set +e
  while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    FILE_MATCHES=$(rg -n "$DENY_PATTERN" "$file" 2>/dev/null || true)
    if [[ -n "$FILE_MATCHES" ]]; then
      MATCHES+="$FILE_MATCHES"$'\n'
    fi
  done < <(collect_go_files)
  set -e
else
  set +e
  while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    FILE_MATCHES=$(grep -nE "$DENY_PATTERN" "$file" 2>/dev/null || true)
    if [[ -n "$FILE_MATCHES" ]]; then
      while IFS= read -r line; do
        MATCHES+="$file:$line"$'\n'
      done <<< "$FILE_MATCHES"
    fi
  done < <(collect_go_files)
  set -e
fi

if [[ -z "$MATCHES" ]]; then
  echo "No direct S3MinIO HTTP usage found"
  exit 0
fi

if command -v rg >/dev/null 2>&1; then
  VIOLATIONS=$(echo "$MATCHES" | rg -v "$ALLOW_PATH_PATTERN" || true)
else
  VIOLATIONS=$(echo "$MATCHES" | grep -Ev "$ALLOW_PATH_PATTERN" || true)
fi
if [[ -n "$VIOLATIONS" ]]; then
  echo "Disallowed direct S3MinIO HTTP usage found:"
  echo "$VIOLATIONS"
  exit 1
fi

echo "S3MinIO HTTP usage is within allowed boundaries"
