# Error Catalog

## compile-time import mismatch

- Symptom: `cannot use redis.Options from .../v8 as .../v9`.
- Cause: service still imports `github.com/go-redis/redis/v8`.
- Fix: replace all v8 imports with `github.com/redis/go-redis/v9`, then `go mod tidy`.

## nil redis client at runtime

- Symptom: `redis options are nil, please initialize redis first`.
- Cause: startup path did not call `InitializeRedisWithRetry`.
- Fix: initialize Redis during service boot before middleware/worker start.

## lock acquisition returns false

- Symptom: lock methods return `acquired=false` with nil error.
- Cause: expected contention (`ErrFailed`/`ErrTaken`).
- Fix: treat as business-level retry/backoff, not as infrastructure failure.
