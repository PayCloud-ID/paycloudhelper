# S3MinIO Shared SDK Architecture

This document defines the shared S3MinIO client architecture in paycloudhelper.

## Goal
- Provide one reusable client surface for all PayCloud services.
- Keep provider-specific behavior isolated from consumer services.
- Prevent service-local HTTP wrappers and duplicated transport logic.

## Layers
- `sdk/services/s3minio/helper`: shared interfaces, DTOs, and operation contracts.
- `sdk/services/s3minio/grpc`: default runtime adapter for gRPC access.
- `sdk/services/s3minio/http`: temporary bridge adapter for provider parity gaps.
- `sdk/services/s3minio/pb`: generated-style protobuf types from provider canonical proto.
- `sdk/services/s3minio/proto`: copied canonical proto snapshot used by generation/drift workflows.

## Runtime Policy
- Default path: `sdk/services/s3minio/grpc`.
- Temporary fallback path: `sdk/services/s3minio/http` (only when parity gap is documented and approved).
- Direct internal HTTP calls from service code are disallowed.
- All new and migrated code must use `sdk/services/s3minio/*`.
