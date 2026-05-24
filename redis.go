package paycloudhelper

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"

	"github.com/PayCloud-ID/paycloudhelper/phjson"
)

const (
	defaultProcessTimeout  = 3000 // 3 seconds
	minTimeout             = 700  // 700ms minimum
	defaultRedisRetryDelay = 50
	defaultRedisRetryMax   = 3
	defaultRedisBackoff    = 10
)

var (
	DefaultRedisTimeout                          = 1000 * time.Millisecond
	redisPoolClient                              atomic.Pointer[redis.Client] // atomic load/store; serialized writes via redisReconnectMu
	redisReconnectMu                             sync.Mutex                   // serializes lazy reconnects in GetRedisPoolClient
	redisHostMem, redisPortMem, redisPasswordMem *string
	redisDbMem                                   *int
	redisOptions                                 *redis.Options
	redisSync                                    *redsync.Redsync
	redisSyncInitOnce                            sync.Once
	redisSyncInitErr                             error
	redisDefaultDuration                         = 300 * time.Second
	redisLockKey                                 = "redis_lock:" // Default Redis lock key prefix
)


// LockError represents a distributed lock operation error with context
type LockError struct {
	Key    string // The lock key
	Op     string // Operation: "acquire" or "release"
	Reason string // Human-readable reason for the error
	Err    error  // Underlying error (if any)
}

func (e *LockError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("lock %s failed for key '%s': %s (cause: %v)", e.Op, e.Key, e.Reason, e.Err)
	}
	return fmt.Sprintf("lock %s failed for key '%s': %s", e.Op, e.Key, e.Reason)
}

func (e *LockError) Unwrap() error {
	return e.Err
}

// RedisInitOptions provides advanced configuration for Redis initialization with retry logic
type RedisInitOptions struct {
	Options    redis.Options
	MaxRetries int           // Maximum number of retry attempts (default: 3)
	RetryDelay time.Duration // Base delay between retries (default: 1s, uses exponential backoff)
	FailFast   bool          // If true, return error on failure; if false, log but continue (default: false for backward compat)
}

// InitializeRedisWithRetry initializes Redis connection with configurable retry logic
// This provides better resilience against transient connection failures during startup
func InitializeRedisWithRetry(opts RedisInitOptions) error {
	// Set defaults for unspecified options
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = 1 * time.Second
	}

	LogI("%s starting retry logic max_retries=%d base_delay=%v", buildLogPrefix("InitializeRedisWithRetry"), opts.MaxRetries, opts.RetryDelay)

	redisLockKey = fmt.Sprintf("redis_lock:%s:", GetAppName())

	// Initialize Redis options with default values
	InitRedisOptions(opts.Options)

	var lastErr error
	for attempt := 1; attempt <= opts.MaxRetries; attempt++ {
		err := initRedisClient(GetRedisOptions())
		if err == nil {
			LogI("%s connected successfully attempt=%d/%d", buildLogPrefix("InitializeRedisWithRetry"), attempt, opts.MaxRetries)
			return nil
		}

		lastErr = err
		LogW("%s attempt failed attempt=%d/%d err=%v", buildLogPrefix("InitializeRedisWithRetry"), attempt, opts.MaxRetries, err)

		// Exponential backoff: delay increases with each attempt
		if attempt < opts.MaxRetries {
			backoffDelay := opts.RetryDelay * time.Duration(attempt)
			LogI("%s retrying delay=%v", buildLogPrefix("InitializeRedisWithRetry"), backoffDelay)
			time.Sleep(backoffDelay)
		}
	}

	// Handle final failure based on FailFast setting
	if opts.FailFast {
		return fmt.Errorf("failed to initialize Redis after %d attempts: %w", opts.MaxRetries, lastErr)
	}

	// Backward compatible behavior: log error but don't fail
	LogE("%s failed to initialize redis after attempts=%d err=%s", buildLogPrefix("InitializeRedisWithRetry"), opts.MaxRetries, lastErr.Error())
	return nil
}

// InitializeRedis initializes Redis with default retry behavior (backward compatible wrapper)
// For advanced retry configuration, use InitializeRedisWithRetry instead
func InitializeRedis(opt redis.Options) {
	_ = InitializeRedisWithRetry(RedisInitOptions{
		Options:    opt,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		FailFast:   false, // Backward compatible: don't fail on error
	})
}

func GetRedisPoolClient() (*redis.Client, error) {
	if redisOptions == nil {
		return nil, errors.New("nil redis options")
	}
	// Fast path: client already alive.
	if c := redisPoolClient.Load(); c != nil {
		return c, nil
	}
	// Slow path: serialize reconnects so only one goroutine rebuilds the client.
	redisReconnectMu.Lock()
	defer redisReconnectMu.Unlock()
	if c := redisPoolClient.Load(); c != nil { // double-check after acquiring lock
		return c, nil
	}
	if err := initRedisClient(redisOptions); err != nil {
		return nil, err
	}
	return redisPoolClient.Load(), nil
}

func InitRedisOptions(rawOpt redis.Options) *redis.Options {
	ro := &rawOpt

	if h, p, err := net.SplitHostPort(rawOpt.Addr); err == nil {
		redisHostMem = &h
		redisPortMem = &p
	}
	redisPasswordMem = &rawOpt.Password
	redisDbMem = &rawOpt.DB

	if ro.Addr == "" {
		ro.Addr = net.JoinHostPort(*redisHostMem, *redisPortMem)
	}
	if ro.Password == "" {
		ro.Password = *redisPasswordMem
	}
	if ro.DB == 0 {
		ro.DB = *redisDbMem
	}
	if ro.Username == "" {
		ro.Username = "default"
	}
	if ro.MaxRetries == 0 {
		ro.MaxRetries = 3
	}
	if ro.MinRetryBackoff == 0 {
		ro.MinRetryBackoff = 10 * time.Millisecond
	}
	if ro.MaxRetryBackoff == 0 {
		ro.MaxRetryBackoff = 500 * time.Millisecond
	}
	// go-redis/v8 used Options.IdleTimeout; in go-redis/v9 this became ConnMaxIdleTime.
	// Keep the prior default behavior by setting ConnMaxIdleTime when unset.
	if ro.ConnMaxIdleTime == 0 {
		ro.ConnMaxIdleTime = 5 * time.Minute
	}
	redisOptions = ro

	// Set custom timeout if provided
	if redisOptions.ReadTimeout > 0 {
		DefaultRedisTimeout = redisOptions.ReadTimeout + DefaultRedisTimeout
		LogI("%s custom redis timeout set=%v", buildLogPrefix("InitRedisOptions"), DefaultRedisTimeout)
	}

	return redisOptions
}

// InitRedSyncOnce initializes the redSync instance once.
// sync.Once handles the already-initialized fast path; the pre-check is omitted
// because reading redisSync outside the Once boundary is a data race.
func InitRedSyncOnce() error {
	redisSyncInitOnce.Do(func() {
		redisSyncInitErr = func() error {
			client, err := GetRedisPoolClient()
			if err != nil {
				LogE("%s failed to initialize redsync err=%s", buildLogPrefix("InitRedSyncOnce"), err.Error())
				return err
			}

			// Create a pool with go-redis client
			pool := goredis.NewPool(client)

			// Create an instance of redSync to be used
			redisSync = redsync.New(pool)
			LogI("%s redsync initialized successfully", buildLogPrefix("InitRedSyncOnce"))
			return nil
		}()
	})
	return redisSyncInitErr
}

func GetRedisOptions() *redis.Options {
	return redisOptions
}

// InitRedisFromEnv initializes Redis from standard environment variables.
// Returns nil immediately when REDIS_HOST is not set — Redis is optional.
// When REDIS_HOST is set but the connection fails, an error is returned so
// the caller (main.go) can decide whether to abort startup.
//
// Supported environment variables:
//
//	REDIS_HOST      Redis server hostname (required to enable Redis)
//	REDIS_PORT      Redis server port          (default: 6379)
//	REDIS_PASSWORD  Redis password             (default: empty)
//	REDIS_DB        Redis database index       (default: 0)
func InitRedisFromEnv() error {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		LogI("%s REDIS_HOST not set, skipping Redis init", buildLogPrefix("InitRedisFromEnv"))
		return nil
	}

	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}

	db := 0
	if s := os.Getenv("REDIS_DB"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			db = v
		} else {
			LogW("%s invalid REDIS_DB=%q using db=0", buildLogPrefix("InitRedisFromEnv"), s)
		}
	}

	return InitializeRedisWithRetry(RedisInitOptions{
		Options: redis.Options{
			Addr:     net.JoinHostPort(host, port),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       db,
		},
		MaxRetries: defaultRedisRetryMax,
		RetryDelay: 1 * time.Second,
		FailFast:   true,
	})
}

// RedisEnabled reports whether Redis has been successfully initialized.
// Use this to guard optional Redis-dependent features at runtime.
func RedisEnabled() bool {
	return redisPoolClient.Load() != nil
}

func GetRedisClient(redisHost, redisPort, redisPassword string, redisDb int) error {
	LogI("%s host=%s port=%s db=%v", buildLogPrefix("GetRedisClient"), redisHost, redisPort, redisDb)
	if GetRedisOptions() == nil {
		InitRedisOptions(redis.Options{
			Addr:     redisHost + ":" + redisPort,
			Password: redisPassword,
			DB:       redisDb,
		})
	}

	err := initRedisClient(GetRedisOptions())

	return err
}

func initRedisClient(opt *redis.Options) error {
	if opt == nil {
		return errors.New("nil redis options")
	}
	LogI("%s starting redis client initialization", buildLogPrefix("initRedisClient"))

	c := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
	defer cancel()

	res, err := c.Ping(ctx).Result()
	if err != nil {
		_ = c.Close()
		redisPoolClient.Store(nil) // clear so RedisEnabled() accurately reflects connectivity
		LoggerErrorHub(err)
		LogE("%s open redis pool connection failed", buildLogPrefix("initRedisClient"))
		return err
	}

	redisPoolClient.Store(c) // store only after successful Ping

	if GetAppName() != "" {
		c.Do(context.Background(), "CLIENT", "SETNAME", GetAppName())
		LogI("%s client name=%v", buildLogPrefix("initRedisClient"), c.ClientGetName(ctx))
	}

	LogI("%s open redis pool connection successful status=%s", buildLogPrefix("initRedisClient"), res)

	if err := InitRedSyncOnce(); err != nil {
		LogW("%s failed to initialize redsync err=%s", buildLogPrefix("initRedisClient"), err.Error())
	}

	return nil
}

// Redis TTL guardrail (added 2026-05-23).
//
// MaxTTL returns the upper-bound clamp for Redis writes via StoreRedis and
// StoreRedisWithContext. Reads env REDIS_MAX_TTL_MINUTES; default 43200 (30d).
// A value <= 0 in the env falls back to the default.
//
// Soft-clamp behavior (not reject): any caller-provided TTL above MaxTTL is
// silently capped at MaxTTL and a warning is logged. This catches future
// overflow-style bugs (like the 127-year clientpg:user-permissions TTL fixed
// in Task 1.1) at write time without breaking the call.
//
// Negative TTLs collapse to 0 (= no expiry) with a warning. Callers that
// genuinely need no-expiry should use StoreRedisNoExpiry explicitly.
func MaxTTL() time.Duration {
	const defaultMinutes = 30 * 24 * 60
	minutes := defaultMinutes
	if s := os.Getenv("REDIS_MAX_TTL_MINUTES"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			minutes = n
		}
	}
	return time.Duration(minutes) * time.Minute
}

// clampStoreTTL applies the soft-clamp policy used by StoreRedis and
// StoreRedisWithContext. Returns the TTL the caller should pass to Redis.
func clampStoreTTL(key string, ttl time.Duration) time.Duration {
	if ttl < 0 {
		LogW("[StoreRedis] negative TTL %v for key=%q, clamping to 0 (no-expiry)", ttl, key)
		return 0
	}
	if max := MaxTTL(); ttl > max {
		LogW("[StoreRedis] TTL %v exceeds max %v for key=%q, clamping (possible overflow bug)", ttl, max, key)
		return max
	}
	return ttl
}

// StoreRedisWithContext stores data to Redis with a custom context
// Allows caller to control cancellation and timeout behavior.
// Applies the soft-clamp (see MaxTTL) so out-of-range TTLs are bounded.
func StoreRedisWithContext(ctx context.Context, id string, data interface{}, duration time.Duration) error {
	rClient, errCl := GetRedisPoolClient()
	if errCl != nil {
		return errCl
	}

	jsonData, err := phjson.Marshal(data)
	if err != nil {
		return err
	}

	// Use provided context with additional timeout as safety net
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultRedisTimeout)
	defer cancel()

	return rClient.Set(timeoutCtx, id, jsonData, clampStoreTTL(id, duration)).Err()
}

// StoreRedis stores data to Redis (backward compatible wrapper)
func StoreRedis(id string, data interface{}, duration time.Duration) error {
	return StoreRedisWithContext(context.Background(), id, data, duration)
}

// StoreRedisNoExpiry stores data to Redis with no expiry (TTL=0).
// Use only for keys that genuinely must never expire. Bypasses MaxTTL.
func StoreRedisNoExpiry(id string, data interface{}) error {
	rClient, errCl := GetRedisPoolClient()
	if errCl != nil {
		return errCl
	}

	jsonData, err := phjson.Marshal(data)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
	defer cancel()

	return rClient.Set(ctx, id, jsonData, 0).Err()
}

func StoreRedisWithLock(id string, data interface{}, duration time.Duration) (err error) {
	// Redis Lock
	lockKey := redisLockKey + id
	lockTTL := GetTrxRedisLockTimeout()
	LogI("%s lock_ttl=%s lock_key=%s", buildLogPrefix("StoreRedisWithLock"), lockTTL, lockKey)

	locked, acquireErr := AcquireLock(lockKey, lockTTL)
	if acquireErr != nil {
		// error acquiring lock
		return acquireErr
	}

	if !locked {
		// already being updated by another process
		return errors.New("already being updated by another process")
	}

	LogI("%s lock acquired key=%v ttl=%v", buildLogPrefix("StoreRedisWithLock"), lockKey, lockTTL)
	defer func() {
		releaseErr := ReleaseLock(lockKey)
		if releaseErr != nil {
			// error releasing lock
			LogD("%s release lock failed err=%s", buildLogPrefix("StoreRedisWithLock"), releaseErr.Error())
		}
	}()

	err = StoreRedis(id, data, duration)

	return
}

// GetRedisWithContext retrieves data from Redis with a custom context
func GetRedisWithContext(ctx context.Context, id string) (string, error) {
	rClient, errCl := GetRedisPoolClient()
	if errCl != nil {
		return "", errCl
	}

	// Use provided context with additional timeout as safety net
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultRedisTimeout)
	defer cancel()

	getRedis := rClient.Get(timeoutCtx, id)
	if getRedis == nil {
		return "", nil
	}

	err := getRedis.Err()
	if err != nil {
		return "", err
	}

	return getRedis.Result()
}

// GetRedis retrieves data from Redis (backward compatible wrapper)
func GetRedis(id string) (string, error) {
	return GetRedisWithContext(context.Background(), id)
}

// DeleteRedisWithContext deletes data from Redis with a custom context
func DeleteRedisWithContext(ctx context.Context, id string) error {
	rClient, errCl := GetRedisPoolClient()
	if errCl != nil {
		return errCl
	}

	// Use provided context with additional timeout as safety net
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultRedisTimeout)
	defer cancel()

	res := rClient.Del(timeoutCtx, id)
	if res == nil {
		return nil
	}

	return res.Err()
}

// DeleteRedis deletes data from Redis (backward compatible wrapper)
func DeleteRedis(id string) error {
	return DeleteRedisWithContext(context.Background(), id)
}

// AcquireLock acquires a distributed lock using RedSync
func AcquireLock(key string, ttl time.Duration) (bool, error) {
	// Ensure redisSync is initialized thread-safely
	if err := InitRedSyncOnce(); err != nil {
		return false, &LockError{
			Key:    key,
			Op:     "acquire",
			Reason: "redsync initialization failed",
			Err:    err,
		}
	}

	// Create a mutex with options
	mutex := redisSync.NewMutex(
		key,
		redsync.WithExpiry(ttl),
		// Add drift factor to account for clock skew
		redsync.WithDriftFactor(0.01),
	)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
	defer cancel()

	// Try to obtain the lock
	err := mutex.LockContext(ctx)
	if err != nil {
		// ErrFailed: retries exhausted without quorum. ErrTaken: quorum agrees lock is held.
		// Both represent normal contention — not an infrastructure error.
		var errTaken *redsync.ErrTaken
		if err == redsync.ErrFailed || errors.As(err, &errTaken) {
			LogD("%s lock already held key=%s", buildLogPrefix("AcquireLock"), key)
			return false, nil
		}
		return false, &LockError{
			Key:    key,
			Op:     "acquire",
			Reason: "mutex lock operation failed",
			Err:    err,
		}
	}

	// Store the mutex in a map for later release
	StoreMutex(key, mutex)
	LogD("%s lock acquired key=%s ttl=%s", buildLogPrefix("AcquireLock"), key, ttl)

	return true, nil
}

func ReleaseLock(key string) error {
	mutex := GetMutex(key)
	if mutex == nil {
		return &LockError{
			Key:    key,
			Op:     "release",
			Reason: "no mutex found for this key (was lock acquired?)",
			Err:    nil,
		}
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
	defer cancel()

	ok, err := mutex.UnlockContext(ctx)
	if err != nil {
		return &LockError{
			Key:    key,
			Op:     "release",
			Reason: "mutex unlock operation failed",
			Err:    err,
		}
	}

	if !ok {
		return &LockError{
			Key:    key,
			Op:     "release",
			Reason: "not the lock owner (lock may have expired or was released by another process)",
			Err:    nil,
		}
	}

	// Remove the mutex from the map
	RemoveMutex(key)
	LogD("%s lock released key=%s", buildLogPrefix("ReleaseLock"), key)

	return nil
}

// AcquireLockWithRetry attempts to acquire a distributed lock with retries
// key: the lock key
// ttl: lock time-to-live
// maxRetries: maximum number of retry attempts
// retryDelay: delay between retries
// Returns:
// - mutex: the lock mutex (nil if not acquired)
// - acquired: whether the lock was acquired
// - err: any error that occurred
func AcquireLockWithRetry(key string, ttl time.Duration, maxRetries int, retryDelay time.Duration) (*redsync.Mutex, bool, error) {
	if err := InitRedSyncOnce(); err != nil {
		return nil, false, fmt.Errorf("failed to initialize redsync: %w", err)
	}

	// Create a mutex with options
	mutex := redisSync.NewMutex(
		key,
		redsync.WithExpiry(ttl),
		redsync.WithTries(maxRetries),
		redsync.WithRetryDelay(retryDelay),
		// Optional: Add drift factor to account for clock skew
		redsync.WithDriftFactor(0.01),
	)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
	defer cancel()

	// Try to obtain the lock
	err := mutex.LockContext(ctx)
	if err != nil {
		// ErrFailed: retries exhausted without quorum. ErrTaken: quorum agrees lock is held.
		// Both represent normal contention — not an infrastructure error.
		var errTaken *redsync.ErrTaken
		if err == redsync.ErrFailed || errors.As(err, &errTaken) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to acquire lock for key %s: %w", key, err)
	}

	return mutex, true, nil
}

// ReleaseLockWithRetry releases a previously acquired lock with retry mechanism.
// Each attempt gets its own fresh context so a timeout on one attempt does not
// poison all subsequent attempts.
func ReleaseLockWithRetry(mutex *redsync.Mutex, maxRetries int) error {
	if mutex == nil {
		return fmt.Errorf("mutex is nil")
	}

	backoff := GetTrxRedisBackoff()
	var err error
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
		ok, unlockErr := mutex.UnlockContext(ctx)
		cancel()

		if unlockErr == nil {
			if !ok {
				err = fmt.Errorf("failed to release lock: not owner")
				time.Sleep(time.Duration(backoff*(i+1)) * time.Millisecond)
				continue
			}
			return nil
		}
		err = unlockErr
		time.Sleep(time.Duration(backoff*(i+1)) * time.Millisecond)
	}

	return fmt.Errorf("failed to release lock after %d attempts: %w", maxRetries, err)
}

// cachedBackoff and cachedLockTimeout are loaded once from env on first use.
// Reading os.Getenv on every lock acquire/release is a per-call syscall; caching
// eliminates that cost without restricting when the .env file must be loaded.
var (
	backoffOnce        sync.Once
	cachedBackoff      int
	lockTimeoutOnce    sync.Once
	cachedLockTimeout  time.Duration
)

// GetTrxRedisBackoff returns the retry backoff in milliseconds for distributed lock
// operations. Value is read from TRANSACTION_REDIS_BACKOFF once and cached.
func GetTrxRedisBackoff() int {
	backoffOnce.Do(func() {
		v := defaultRedisBackoff
		if val, err := strconv.Atoi(os.Getenv("TRANSACTION_REDIS_BACKOFF")); err == nil && val >= 10 {
			v = val
		}
		cachedBackoff = v
	})
	return cachedBackoff
}

// GetTrxRedisLockTimeout returns the distributed lock TTL.
// Value is read from TRANSACTION_REDIS_LOCK_TIMEOUT once and cached.
func GetTrxRedisLockTimeout() time.Duration {
	lockTimeoutOnce.Do(func() {
		ms := 2000
		if val, err := strconv.Atoi(os.Getenv("TRANSACTION_REDIS_LOCK_TIMEOUT")); err == nil && val >= minTimeout {
			ms = val
		}
		cachedLockTimeout = time.Duration(ms) * time.Millisecond
	})
	return cachedLockTimeout
}

// resetRedisClientStateForTesting tears down package-level Redis state so tests can
// attach a fresh client (e.g. miniredis). Only for paycloudhelper package tests.
func resetRedisClientStateForTesting() {
	if c := redisPoolClient.Load(); c != nil {
		_ = c.Close()
	}
	redisPoolClient.Store(nil)
	redisSync = nil
	redisSyncInitOnce = sync.Once{}
	redisSyncInitErr = nil
	redisOptions = nil
	// Reset cached env values so tests that manipulate env vars see fresh reads.
	backoffOnce = sync.Once{}
	cachedBackoff = 0
	lockTimeoutOnce = sync.Once{}
	cachedLockTimeout = 0
}
