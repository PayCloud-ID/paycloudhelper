# S3MinIO Shared Client Observability Policy

## Scope

This policy standardizes liveness/readiness behavior for shared S3MinIO helper clients.

## Required Capabilities

- Liveness-equivalent health check via CheckHealth().
- Readiness-equivalent dependency check via CheckReady().
- Structured operation metadata: operation, transport, status code, elapsed time.

## Contracts

1. Health probe maps provider status to shared HealthResponse.
2. Readiness probe maps provider and dependency readiness to shared ReadyResponse.
3. All transports must return normalized status values (`ok` or `unavailable`).
4. Transport adapters should avoid leaking provider-specific payload formats.

## Transitional Rule

When a capability is HTTP-only in provider API, shared helper transport adapter in sdk/services/s3minio/http is allowed.
Consumer repositories must not call internal provider HTTP endpoints directly.
