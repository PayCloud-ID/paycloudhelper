# S3MinIO Phase 2 Scale-Out Status

This document captures Phase 2 completion for S3MinIO as the first service-scoped SDK rollout.

## Scope

Phase 2 goal in this cycle: complete consumer adoption for S3MinIO only, using shared imports from:

- `bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/helper`
- `bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/grpc`

## First Adopter

- Primary first adopter path: `paycloud-be-adminpg-manager`.
- High-risk adopter validated in the same wave: `paycloud-be-reporting-manager`.

## Consumer Migration Outcome

The following consumers are on the service-scoped SDK surface and no longer maintain local S3MinIO stubs:

1. `paycloud-be-adminpg-manager`
2. `paycloud-be-adminft-manager`
3. `paycloud-be-clientpg-manager`
4. `paycloud-be-dashboard-manager`
5. `paycloud-be-merchantft-manager`
6. `paycloud-be-merchantpg-manager`
7. `paycloud-be-reporting-manager`

## Phase 2 Deliverables Completed

1. Shared import path convergence to `sdk/services/s3minio/*`.
2. Removal of consumer-local `s3minio.pb.go` and `s3minio_grpc.pb.go` duplicates.
3. Removal of reporting direct `Http_Minio` fallback path.
4. Preservation of provider canonical proto ownership in `paycloud-be-s3minio-manager`.

## Exit Criteria Check

- Consumer-local proto ownership removed for S3MinIO: complete.
- Shared SDK import path used across all target consumers: complete.
- Reporting fallback path removed: complete.
- Ready to proceed to broader multi-service SDK scaling model: complete.