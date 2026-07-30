# Common Mistakes

1. Updating paycloudhelper to v1.10.0+ but leaving direct v8 redis imports.
   (Also: looking for a `v2.0.0` to bump to — it does not exist. The break shipped on the v1.x line.)
2. Using `InitializeRedis` in new code instead of `InitializeRedisWithRetry`.
3. Treating lock contention as a hard error instead of a normal concurrency outcome.
4. Skipping `go test -race` for services with background workers and lock usage.
5. Enabling Redis auth fields when Redis server has no password configured.
