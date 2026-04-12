#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

# The SDK uses a hand-maintained gRPC surface in sdk/services/s3minio/pb/client.go
# (see docs/sdk/grpc-stub-maintenance.md). protoc with go_package would emit into
# unrelated tree paths; buf generate is optional for teams regenerating stubs.
# This script validates that the proto snapshot and pb surface compile together.
go test ./sdk/services/s3minio/helper ./sdk/services/s3minio/grpc ./sdk/services/s3minio/http ./sdk/services/s3minio/facade -count=1

echo "S3MinIO SDK packages validated against current proto snapshot"
