#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <service-name>" >&2
  exit 1
fi

SERVICE_NAME="$1"
SERVICE_DIR="$ROOT_DIR/sdk/services/$SERVICE_NAME"
DOC_FILE="$ROOT_DIR/docs/sdk/${SERVICE_NAME}-scaffold.md"

mkdir -p "$SERVICE_DIR/helper" "$SERVICE_DIR/grpc" "$SERVICE_DIR/http" "$SERVICE_DIR/pb" "$SERVICE_DIR/proto" "$SERVICE_DIR/facade"

cat > "$SERVICE_DIR/helper/doc.go" <<EOF
// Package helper exposes the service-scoped ${SERVICE_NAME} SDK contracts.
package helper
EOF

cat > "$SERVICE_DIR/grpc/doc.go" <<EOF
// Package grpc exposes the service-scoped ${SERVICE_NAME} gRPC transport adapter.
package grpc
EOF

cat > "$SERVICE_DIR/http/doc.go" <<EOF
// Package http exposes the service-scoped ${SERVICE_NAME} HTTP transition adapter.
package http
EOF

cat > "$SERVICE_DIR/pb/doc.go" <<EOF
// Package pb exposes the service-scoped ${SERVICE_NAME} protobuf compatibility surface.
package pb
EOF

cat > "$SERVICE_DIR/proto/README.md" <<EOF
# ${SERVICE_NAME} proto snapshot

Store the canonical proto snapshot for ${SERVICE_NAME} here.
Update this directory only through scripts/proto workflows.
EOF

cat > "$SERVICE_DIR/facade/doc.go" <<EOF
// Package facade provides stable constructors for service-scoped ${SERVICE_NAME} SDK usage.
package facade
EOF

cat > "$DOC_FILE" <<EOF
# ${SERVICE_NAME} SDK Scaffold

Generated scaffold for the ${SERVICE_NAME} service SDK.

## Directories
- sdk/services/${SERVICE_NAME}/helper
- sdk/services/${SERVICE_NAME}/grpc
- sdk/services/${SERVICE_NAME}/http
- sdk/services/${SERVICE_NAME}/pb
- sdk/services/${SERVICE_NAME}/proto
- sdk/services/${SERVICE_NAME}/facade

## Next Steps
1. Define transport-neutral contracts in helper.
2. Add gRPC adapters in grpc.
3. Keep HTTP package only for approved transition gaps.
4. Copy canonical proto snapshot into proto.
5. Document rollout and compatibility notes before release.
EOF

cat <<EOF
Created scaffold for ${SERVICE_NAME}:
- ${SERVICE_DIR}/helper
- ${SERVICE_DIR}/grpc
- ${SERVICE_DIR}/http
- ${SERVICE_DIR}/pb
- ${SERVICE_DIR}/proto
- ${SERVICE_DIR}/facade
- ${DOC_FILE}

Foundation checklist:
1. Add transport-neutral contracts in helper.
2. Prefer sdk/shared/* for cross-service runtime pieces.
3. Do not introduce direct internal HTTP calls outside shared adapters.
4. Add service-specific validation tests before rollout.
EOF
