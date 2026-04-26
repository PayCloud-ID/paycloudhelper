# Error Catalog

## lock never released

- Symptom: throughput drops; keys remain locked.
- Cause: missing deferred unlock or panic path bypassing release.
- Fix: ensure release in `defer`, include panic-safe recovery.

## worker flood on contention

- Symptom: high retry loop and CPU usage.
- Cause: contention treated as error without backoff.
- Fix: handle `acquired=false` as controlled retry path.

## reprocessing duplicates

- Symptom: same message processed multiple times.
- Cause: lock scope too narrow or changed key format.
- Fix: restore stable keying and enforce idempotency checks.
