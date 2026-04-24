---

## name: redis-patterns
description: Guides Redis initialization, store/get operations, distributed lock patterns, and key conventions in paycloudhelper.
applyTo: '**/redis*.go, **/mutex.go'

# Redis Patterns — paycloudhelper

## Architecture

- **Singleton** `redisPoolClient *redis.Client` — one per process, lazy-initialized via `GetRedisPoolClient()`
- **RedSync** `*redsync.Redsync` — distributed lock coordinator, initialized via `InitRedSyncOnce()`
- **Lock key prefix** `redis_lock:{AppName}:` — set automatically by `InitializeRedisWithRetry()`

## Initialization

### Basic (Legacy)

```go
// Consumer service startup
InitializeRedis(redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
})
```

### With Retry (Recommended for Production)

```go
err := InitializeRedisWithRetry(RedisInitOptions{
    Options: redis.Options{
        Addr:     "redis:6379",
        Password: os.Getenv("REDIS_PASSWORD"),
        DB:       0,
    },
    MaxRetries: 3,          // default: 3
    RetryDelay: time.Second, // exponential backoff
    FailFast:   true,        // return error on failure
})
if err != nil {
    log.Fatal("redis init failed:", err)
}
```

## Key Conventions


| Key Pattern                       | Used By           | Purpose                       |
| --------------------------------- | ----------------- | ----------------------------- |
| `redis_lock:{AppName}:{resource}` | Distributed locks | Prevent concurrent processing |
| `csrf-{token}`                    | `VerifCsrf`       | CSRF token validation         |
| `revoke_token_{merchantId}`       | `RevokeToken`     | JWT revocation list           |
| `{idempotency-key}`               | `VerifIdemKey`    | Duplicate request detection   |


## Store & Retrieve

```go
// Store with duration
err := StoreRedis("my-key", myValue, 5*time.Minute)

// Retrieve
val, err := GetRedis("my-key")
if err != nil {
    if strings.Contains(err.Error(), "redis: nil") {
        // Key not found — expected case
    } else {
        // Real Redis error
    }
}

// Store with distributed lock (atomic)
err := StoreRedisWithLock("my-key", myValue, 5*time.Minute)
```

## Distributed Locks

### Simple Lock (< 2s operations)

```go
locked, err := AcquireLock("redis_lock:myapp:resource-id", 2*time.Second)
if err != nil {
    return err
}
if !locked {
    return errors.New("resource busy")
}
defer ReleaseLock("redis_lock:myapp:resource-id")

// ... critical section ...
```

### Lock with Retry (Recommended for Contention)

```go
mutex, acquired, err := AcquireLockWithRetry(
    "redis_lock:myapp:resource-id",
    2*time.Second,        // TTL
    3,                    // max retries
    50*time.Millisecond,  // retry delay (from TRANSACTION_REDIS_BACKOFF env)
)
if err != nil || !acquired {
    return fmt.Errorf("failed to acquire lock: %w", err)
}
defer ReleaseLockWithRetry(mutex, 3)

// ... critical section ...
```

### Lock Configuration via Env


| Env Var                          | Default   | Min      | Purpose     |
| -------------------------------- | --------- | -------- | ----------- |
| `TRANSACTION_REDIS_LOCK_TIMEOUT` | `2000` ms | `700` ms | Lock TTL    |
| `TRANSACTION_REDIS_BACKOFF`      | `10` ms   | —        | Retry delay |


## Timeout Handling

```go
// DefaultRedisTimeout = 1000ms — used in GetRedis/StoreRedis
// Always set context with timeout for Redis operations
ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
defer cancel()
result, err := redisPoolClient.Get(ctx, key).Result()
```

## Mutex Map (In-Process Coordination)

```go
// Store a mutex instance (for cross-goroutine lock sharing)
StoreMutex("my-resource", mutex)

// Retrieve for release in a different context
m := GetMutex("my-resource")
if m != nil {
    m.Unlock()
    RemoveMutex("my-resource")
}
```

## Conditional Authentication

```go
// Redis options — only set credentials if non-empty
// ✅ GOOD: Won't cause "ERR AUTH called without any password" on no-auth Redis
opt := redis.Options{Addr: "redis:6379"}
if password != "" {
    opt.Password = password
    opt.Username = username
}
InitializeRedis(opt)
```

## Error Catalog


| Error                             | Meaning                      | Action                       |
| --------------------------------- | ---------------------------- | ---------------------------- |
| `redis: nil`                      | Key not found                | Expected — not a fatal error |
| `context deadline exceeded`       | Timeout                      | Retry or surface as 500      |
| `dial tcp: connection refused`    | Redis down                   | Alert + retry                |
| `WRONGTYPE Operation`             | Wrong data type at key       | Bug — check key namespace    |
| Lock acquire `false` with nil err | Lock held by another process | Retry or return busy         |


