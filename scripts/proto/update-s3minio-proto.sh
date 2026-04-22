#!/usr/bin/env bash
# update-s3minio-proto.sh (scripts/proto)
#
# Purpose:
# - Refresh local shared proto snapshot for S3MinIO SDK from canonical manager proto source.
# - Keep sdk/services/s3minio/proto/s3minio.proto aligned with source-of-truth proto.
#
# Usage:
# - ./scripts/proto/update-s3minio-proto.sh
#
# Options / Environment:
# - S3MINIO_MANAGER_PROTO (optional)
#   - Overrides canonical source proto file path.
#   - Default: ./paycloud-be-s3minio-manager/proto/s3minio.proto
#
# What It Reads:
# - Source proto path from S3MINIO_MANAGER_PROTO or default path.
# - Existing repository tree for target directory creation.
#
# What It Affects / Does:
# - Ensures target directory exists: sdk/services/s3minio/proto
# - Copies source proto to target snapshot path:
#   sdk/services/s3minio/proto/s3minio.proto
# - If source proto is missing, prints guidance and exits successfully (no-op).
#
# Exit Behavior:
# - 0 when snapshot is updated or source is absent (intentional skip).
# - Non-zero on filesystem errors (mkdir/cp failure, etc.).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_SOURCE="./paycloud-be-s3minio-manager/proto/s3minio.proto"
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
