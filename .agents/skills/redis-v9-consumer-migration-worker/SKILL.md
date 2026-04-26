---
name: redis-v9-consumer-migration-worker
description: Provides migration guardrails for queue or async worker services upgrading to paycloudhelper v2 with Redis locks and retry-heavy execution paths.
applyTo: '**/*worker*.go, **/*consumer*.go, **/*queue*.go'
---

# Redis v9 Migration for Worker Services

## Focus Areas

- Distributed lock behavior (`AcquireLockWithRetry`, `ReleaseLockWithRetry`).
- Idempotent retry loops and poison-message handling.
- Startup safety for long-running consumers.

## Locking Contract

- `acquired=false` with nil error is expected contention.
- Infrastructure errors are non-nil and should increment failure metrics.
- Always release acquired lock with retry-aware release helper.

## Worker Checklist

1. Upgrade imports to v9.
2. Keep lock key naming stable for cross-service compatibility.
3. Preserve retry/backoff behavior and dead-letter routing.
4. Ensure worker exits on Redis init failure when configured fail-fast.

## Test Matrix

- Contention scenario test (second lock acquisition).
- Retry and unlock failure-path tests.
- `go test -race` with concurrent worker goroutines.
- Integration smoke run against staging queue + Redis.
