#!/usr/bin/env bash
set -euo pipefail

SCAN_ROOTS=(
  "/Users/natan/go/src/paycloud-be-adminft-manager"
  "/Users/natan/go/src/paycloud-be-adminpg-manager"
  "/Users/natan/go/src/paycloud-be-clientpg-manager"
  "/Users/natan/go/src/paycloud-be-dashboard-manager"
  "/Users/natan/go/src/paycloud-be-merchantft-manager"
  "/Users/natan/go/src/paycloud-be-merchantpg-manager"
  "/Users/natan/go/src/paycloud-be-reporting-manager"
  "/Users/natan/go/src/paycloud-be-s3minio-manager"
  "/Users/natan/Projects/_htdocs_pc/paycloudhelper"
)

DENY_PATTERN='Http_Minio|/api/v2/download|/api/v2/generate_view_url|/api/v2/download_file|/api/v2/view'
ALLOW_PATH_PATTERN='^(/Users/natan/go/src/paycloud-be-s3minio-manager/|/Users/natan/Projects/_htdocs_pc/paycloudhelper/sdk/services/s3minio/http/)'

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
