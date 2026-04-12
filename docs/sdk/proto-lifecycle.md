# S3MinIO Proto Lifecycle

This workflow keeps provider proto and shared helper stubs synchronized.

## Source of Truth
- Canonical proto: `paycloud-be-s3minio-manager/proto/s3minio.proto`
- Shared service-sdk snapshot: `paycloudhelper/sdk/services/s3minio/proto/s3minio.proto`
- Generated client surface: `paycloudhelper/sdk/services/s3minio/pb/client.go`

## Update Flow
1. Update canonical proto in provider repository.
2. Run `./scripts/proto/update-s3minio-proto.sh` in paycloudhelper.
3. Run `./scripts/proto/gen-s3minio-client.sh` in paycloudhelper.
4. Commit generated artifacts and release helper version.
