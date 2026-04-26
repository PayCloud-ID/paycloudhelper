#!/usr/bin/env bash
set -e

PROTO_DIR="sdk/services/s3minio/proto"
PROTO_FILE="${PROTO_DIR}/s3minio.proto"
OUT_DIR="sdk/services/s3minio/pb/wirepb"

mkdir -p "$OUT_DIR"

protoc \
  --proto_path="${PROTO_DIR}" \
  --go_out="${OUT_DIR}" \
  --go_opt=paths=source_relative \
  --go-grpc_out="${OUT_DIR}" \
  --go-grpc_opt=paths=source_relative \
  "${PROTO_FILE}"

echo "Proto generated successfully into ${OUT_DIR}"
