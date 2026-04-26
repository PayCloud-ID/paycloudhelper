---
name: redis-v9-consumer-migration-scheduler
description: Guides cron and scheduler services to migrate paycloudhelper Redis dependencies while preserving singleton job execution and timing guarantees.
applyTo: '**/*cron*.go, **/*scheduler*.go, **/*job*.go'
---

# Redis v9 Migration for Scheduler Services

## Focus Areas

- Singleton job execution via distributed lock.
- Safe TTL and lock timeout values for long-running jobs.
- Avoiding overlap during deploy/restart windows.

## Scheduler Checklist

1. Upgrade paycloudhelper and direct Redis imports to v9.
2. Confirm lock TTL > worst-case job duration (or refresh strategy).
3. Keep lock keys stable across service instances.
4. Validate no double-run across rolling restart.

## Testing

- Simulated overlap test with two scheduler instances.
- Lock timeout edge tests for long jobs.
- Race test for startup and shutdown hooks.
- Canary run in staging with observed scheduler metrics.
