# gRPC Stub Maintenance

## Required Commands
- `./scripts/proto/update-s3minio-proto.sh`
- `./scripts/proto/gen-s3minio-client.sh`
- `./scripts/proto/check-stub-drift.sh`

## Maintenance Rules
- Never hand-edit generated `*.pb.go` files.
- Keep generated package names stable across releases.
- Validate helper package tests after generation.
- If drift is detected in CI, regenerate and commit before merge.
- During Phase 1, service-scoped SDK packages may wrap compatibility packages while generation still lands in legacy paths.
