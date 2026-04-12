#!/usr/bin/env bash
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
