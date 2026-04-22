#!/usr/bin/env bash
# gen-s3minio-client.sh
#
# Purpose:
# - Thin wrapper that delegates S3MinIO SDK/proto client generation flow to the canonical
#   script in scripts/proto.
#
# Usage:
# - ./scripts/gen-s3minio-client.sh
#
# Options:
# - No CLI options; any behavior is defined by the delegated script.
#
# What It Reads:
# - BASH_SOURCE to resolve repository root.
# - Target delegated script:
#   scripts/proto/gen-s3minio-client.sh
#
# What It Affects / Does:
# - Replaces current process with delegated script via exec.
# - No direct file writes in this wrapper itself.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "$ROOT_DIR/scripts/proto/gen-s3minio-client.sh"
