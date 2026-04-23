#!/usr/bin/env bash
# update-s3minio-proto.sh
#
# Purpose:
# - Thin wrapper that delegates canonical S3MinIO proto snapshot refresh to scripts/proto.
#
# Usage:
# - ./scripts/update-s3minio-proto.sh
#
# Options:
# - No CLI options; environment behavior is handled by delegated script.
#
# What It Reads:
# - BASH_SOURCE for root path resolution.
# - Delegated script path: scripts/proto/update-s3minio-proto.sh
#
# What It Affects / Does:
# - Replaces current process with delegated script via exec.
# - No direct write logic in this wrapper.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "$ROOT_DIR/scripts/proto/update-s3minio-proto.sh"
