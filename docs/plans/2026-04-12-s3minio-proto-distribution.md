# S3MinIO Proto Distribution Workflow

## Goal

Keep one canonical S3MinIO protobuf contract in the provider repo and regenerate shared helper client stubs in one place.

## Canonical Source

- Provider canonical proto: /Users/natan/go/src/paycloud-be-s3minio-manager/proto/s3minio.proto
- Shared service-sdk snapshot: /Users/natan/Projects/_htdocs_pc/paycloudhelper/sdk/services/s3minio/proto/s3minio.proto

## Workflow

1. Edit canonical provider proto in paycloud-be-s3minio-manager.
2. In paycloudhelper, run scripts/proto/update-s3minio-proto.sh.
3. Run scripts/proto/gen-s3minio-client.sh.
4. Run helper tests for sdk/services/s3minio/helper, grpc, and http.
5. Release paycloudhelper version.
6. Consumer services upgrade one paycloudhelper dependency.

## Commands

```bash
cd /Users/natan/Projects/_htdocs_pc/paycloudhelper
./scripts/proto/update-s3minio-proto.sh
./scripts/proto/gen-s3minio-client.sh
```
