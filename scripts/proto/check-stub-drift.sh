#!/usr/bin/env bash
# check-stub-drift.sh
#
# Purpose:
# - Detect drift between the local S3MinIO proto snapshot and the canonical source proto
#   by running update + generation workflow and comparing checksums.
# - Intended for CI/local guardrail to ensure committed snapshot consistency.
#
# Usage:
# - ./scripts/proto/check-stub-drift.sh
#
# Options:
# - No CLI options.
# - Environment inherited by delegated scripts (for example S3MINIO_MANAGER_PROTO).
#
# What It Reads:
# - Existing snapshot file: sdk/services/s3minio/proto/s3minio.proto (if present).
# - sha256 checksum utility (shasum).
# - Delegated scripts:
#   - scripts/proto/update-s3minio-proto.sh
#   - scripts/proto/gen-s3minio-client.sh
#
# What It Affects / Does:
# - May refresh snapshot and regenerate/validate SDK packages via delegated scripts.
# - Fails when snapshot content changes after sync/generation (indicating drift needing commit).
#
# Exit Behavior:
# - 0: no drift detected.
# - 1: drift detected after refresh (snapshot changed).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROTO_SNAPSHOT="$ROOT_DIR/sdk/services/s3minio/proto/s3minio.proto"

before_sum=""
if [[ -f "$PROTO_SNAPSHOT" ]]; then
  before_sum="$(shasum -a 256 "$PROTO_SNAPSHOT" | awk '{print $1}')"
fi

"$ROOT_DIR/scripts/proto/update-s3minio-proto.sh"
"$ROOT_DIR/scripts/proto/gen-s3minio-client.sh"

if [[ -n "$before_sum" ]] && [[ -f "$PROTO_SNAPSHOT" ]]; then
  after_sum="$(shasum -a 256 "$PROTO_SNAPSHOT" | awk '{print $1}')"
  if [[ "$before_sum" != "$after_sum" ]]; then
    echo "S3MinIO proto snapshot drift vs canonical manager proto." >&2
    echo "Run scripts/proto/update-s3minio-proto.sh with S3MINIO_MANAGER_PROTO set, then commit sdk/services/s3minio/proto/s3minio.proto." >&2
    exit 1
  fi
fi

echo "No S3MinIO stub drift detected"
