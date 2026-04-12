# Multi-Service Integration Layout

## Roles
- Provider (`paycloud-be-s3minio-manager`): canonical API and proto owner.
- Shared helper (`paycloudhelper`): SDK and transport adapters.
- Consumers (manager services): business calls through shared SDK only.

## Directory Expectations
- Provider proto remains in provider repo.
- Shared generated stubs and adapters remain in paycloudhelper.
- Consumer repos avoid copied proto and direct HTTP bridge logic.

## Reference Layout
- `sdk/services/s3minio/*` is the Phase 1 reference implementation.
- New services should start from `make proto.service.scaffold SERVICE=<service>`.

## Shared Runtime Areas
- `sdk/shared/transport`
- `sdk/shared/observability`
- `sdk/shared/errors`
