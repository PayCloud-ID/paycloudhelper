#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_SOURCE="/Users/natan/go/src/paycloud-be-s3minio-manager/proto/s3minio.proto"
SOURCE_PROTO="${S3MINIO_MANAGER_PROTO:-$DEFAULT_SOURCE}"
TARGET_PROTO_DIR="$ROOT_DIR/sdk/services/s3minio/proto"
TARGET_PROTO="$TARGET_PROTO_DIR/s3minio.proto"

if [[ ! -f "$SOURCE_PROTO" ]]; then
  echo "Skipping proto copy: canonical source not found at $SOURCE_PROTO" >&2
  echo "Set S3MINIO_MANAGER_PROTO to paycloud-be-s3minio-manager/proto/s3minio.proto to refresh the snapshot." >&2
  exit 0
fi

mkdir -p "$TARGET_PROTO_DIR"
cp "$SOURCE_PROTO" "$TARGET_PROTO"

echo "Updated shared proto snapshot: $TARGET_PROTO"
