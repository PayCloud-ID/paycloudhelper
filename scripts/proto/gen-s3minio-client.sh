#!/usr/bin/env bash
# gen-s3minio-client.sh (scripts/proto)
#
# Purpose:
# - Validate that current S3MinIO SDK packages compile against the checked-in proto snapshot.
# - This is a safety/consistency gate for hand-maintained SDK pb surface.
#
# Usage:
# - ./scripts/proto/gen-s3minio-client.sh
#
# Options:
# - No CLI options.
#
# What It Reads:
# - Go package sources under:
#   - sdk/services/s3minio/helper
#   - sdk/services/s3minio/grpc
#   - sdk/services/s3minio/http
#   - sdk/services/s3minio/facade
# - Current module and dependency graph via go test.
#
# What It Affects / Does:
# - Runs go test for selected SDK packages with -count=1 (no cached test results).
# - Does not write generated code in this script; it validates compilation compatibility.
#
# Exit Behavior:
# - Non-zero when package compilation/tests fail.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

# The SDK uses a hand-maintained gRPC surface in sdk/services/s3minio/pb/client.go
# (see docs/sdk/grpc-stub-maintenance.md). protoc with go_package would emit into
# unrelated tree paths; buf generate is optional for teams regenerating stubs.
# This script validates that the proto snapshot and pb surface compile together.
go test ./sdk/services/s3minio/helper ./sdk/services/s3minio/grpc ./sdk/services/s3minio/http ./sdk/services/s3minio/facade -count=1

echo "S3MinIO SDK packages validated against current proto snapshot"
