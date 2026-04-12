# S3MinIO Service Onboarding

Use this checklist when integrating a service with the shared S3MinIO SDK.

## Checklist
1. Import `sdk/services/s3minio/helper` contracts and `sdk/services/s3minio/grpc` adapter for new code.
2. Wire service config for endpoint, timeout, and auth once at startup.
3. Replace local MinIO HTTP wrappers with shared client calls.
4. Run governance check: `./scripts/check-no-direct-s3minio-http.sh`.
5. Add service tests for upload, download, view URL, health, and readiness.
6. Document rollback procedure before deployment.

## Migration Note
- Existing and new consumers must use the `sdk/services/<service>` layout.
- Legacy `phs3*` paths are removed and blocked by governance checks.
