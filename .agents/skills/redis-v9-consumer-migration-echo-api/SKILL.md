---
name: redis-v9-consumer-migration-echo-api
description: Covers paycloudhelper v2 Redis migration for Echo API services using CSRF, idempotency, revoke-token, and cache-backed request paths.
applyTo: '**/*middleware*.go, **/cmd/**/*.go, **/internal/handler/**/*.go'
---

# Redis v9 Migration for Echo API Services

## Focus Areas

- Middleware dependencies: `VerifCsrf`, `VerifIdemKey`, `RevokeToken`.
- Request-path Redis calls that must stay low-latency.
- Correct startup ordering before HTTP server starts accepting traffic.

## Startup Checklist

1. Call `InitializeRedisWithRetry` before registering middleware or routes.
2. Confirm request timeout settings map to existing SLOs.
3. Keep idempotency/session TTL semantics unchanged.

## API Risk Controls

- Do not change response shape/status for duplicate idempotency requests.
- Preserve token revoke semantics and key names.
- Keep compatibility with existing Redis key format used by running clients.

## Test Matrix

- Middleware unit tests with `miniredis`.
- Cancelled-context test on Redis reads/writes in request helpers.
- Lock contention tests for endpoints using distributed lock wrappers.
- Race tests for startup and lock map behavior.
