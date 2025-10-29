package paycloudhelper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
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
	redisPoolClient                              *redis.Client
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

	LogI("InitRedis: Starting with retry logic (max_retries=%d, base_delay=%v)...", opts.MaxRetries, opts.RetryDelay)

	redisLockKey = fmt.Sprintf("redis_lock:%s:", GetAppName())

	// Initialize Redis options with default values
	InitRedisOptions(opts.Options)

	var lastErr error
	for attempt := 1; attempt <= opts.MaxRetries; attempt++ {
		err := initRedisClient(GetRedisOptions())
		if err == nil {
			LogI("InitRedis: Connected successfully on attempt %d/%d", attempt, opts.MaxRetries)
			return nil
		}

		lastErr = err
		LogW("InitRedis: Attempt %d/%d failed: %v", attempt, opts.MaxRetries, err)

		// Exponential backoff: delay increases with each attempt
		if attempt < opts.MaxRetries {
			backoffDelay := opts.RetryDelay * time.Duration(attempt)
			LogI("InitRedis: Retrying in %v...", backoffDelay)
			time.Sleep(backoffDelay)
		}
	}

	// Handle final failure based on FailFast setting
	if opts.FailFast {
		return fmt.Errorf("failed to initialize Redis after %d attempts: %w", opts.MaxRetries, lastErr)
	}

	// Backward compatible behavior: log error but don't fail
	LogE("InitRedis: Failed to initialize Redis client after %d attempts: %s", opts.MaxRetries, lastErr.Error())
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

	// open new pool connection if previously memory address pool connection is nil
	if redisPoolClient == nil {
		if err := GetRedisClient(*redisHostMem, *redisPortMem, *redisPasswordMem, *redisDbMem); err != nil {
			return nil, err
		}
	}

	return redisPoolClient, nil
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
	if ro.IdleTimeout == 0 {
		ro.IdleTimeout = 5 * time.Minute
	}
	redisOptions = ro

	// Set custom timeout if provided
	if redisOptions.ReadTimeout > 0 {
		DefaultRedisTimeout = redisOptions.ReadTimeout + DefaultRedisTimeout
		LogI("InitRedis: Custom redis timeout set to %v", DefaultRedisTimeout)
	}

	return redisOptions
}

// InitRedSyncOnce initializes the redSync instance once
func InitRedSyncOnce() error {
	if redisSync != nil {
		return nil
	}

	redisSyncInitOnce.Do(func() {
		redisSyncInitErr = func() error {
			client, err := GetRedisPoolClient()
			if err != nil {
				LogE(fmt.Sprintf("Failed to initialize redisSync: %s", err.Error()))
				return err
			}

			// Create a pool with go-redis client
			pool := goredis.NewPool(client)

			// Create an instance of redSync to be used
			redisSync = redsync.New(pool)
			LogI("InitRedSync: redisSync initialized successfully")
			return nil
		}()
	})
	return redisSyncInitErr
}

func GetRedisOptions() *redis.Options {
	return redisOptions
}

func GetRedisClient(redisHost, redisPort, redisPassword string, redisDb int) error {
	LogI("InitRedis: GetRedisClient... %s:%s/%v", redisHost, redisPort, redisDb)
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
	LogI("InitRedis: Starting...")

	redisPoolClient = redis.NewClient(GetRedisOptions())

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
	defer cancel()

	res, err := redisPoolClient.Ping(ctx).Result()
	if err != nil {
		LoggerErrorHub(err)
		LogE("InitRedis: open redis pool connection failed")
		return err
	}

	if GetAppName() != "" {
		redisPoolClient.Do(context.Background(), "CLIENT", "SETNAME", GetAppName())
		LogI("InitRedis: %v", redisPoolClient.ClientGetName(ctx))
	}

	LogI("InitRedis: open redis pool connection successfully. %s", res)

	// Initialize RedSync after Redis is initialized
	if err := InitRedSyncOnce(); err != nil {
		LogW("Warning: Failed to initialize redisSync: %s", err.Error())
	}

	return nil
}

// StoreRedisWithContext stores data to Redis with a custom context
// Allows caller to control cancellation and timeout behavior
func StoreRedisWithContext(ctx context.Context, id string, data interface{}, duration time.Duration) error {
	rClient, errCl := GetRedisPoolClient()
	if errCl != nil {
		return errCl
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Use provided context with additional timeout as safety net
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultRedisTimeout)
	defer cancel()

	return rClient.Set(timeoutCtx, id, string(jsonData), duration).Err()
}

// StoreRedis stores data to Redis (backward compatible wrapper)
func StoreRedis(id string, data interface{}, duration time.Duration) error {
	return StoreRedisWithContext(context.Background(), id, data, duration)
}

func StoreRedisWithLock(id string, data interface{}, duration time.Duration) (err error) {
	fmtLogPrefix := "StoreRedisWithLock"

	// Redis Lock
	lockKey := redisLockKey + id
	lockTTL := GetTrxRedisLockTimeout()
	LogI(fmt.Sprintf("%s lock_ttl=%s lock_key=%s", fmtLogPrefix, lockTTL, lockKey))

	locked, acquireErr := AcquireLock(lockKey, lockTTL)
	if acquireErr != nil {
		// error acquiring lock
		return acquireErr
	}

	if !locked {
		// already being updated by another process
		return errors.New("already being updated by another process")
	}

	LogI("%s acquired lock_key=%v, lock_ttl=%v", fmtLogPrefix, lockKey, lockTTL)
	defer func() {
		releaseErr := ReleaseLock(lockKey)
		if releaseErr != nil {
			// error releasing lock
			LogD(fmt.Sprintf("%s ERR releasing lock: %s", fmtLogPrefix, releaseErr.Error()))
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
		if err == redsync.ErrFailed {
			// Lock not acquired but no error occurred (already held by another process)
			LogD("AcquireLock: lock already held key=%s", key)
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
	LogD("AcquireLock: lock acquired key=%s ttl=%s", key, ttl)

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
	LogD("ReleaseLock: lock released key=%s", key)

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
	// Initialize redisSync if not already initialized
	if redisSync == nil {
		err := InitRedSyncOnce()
		if err != nil || redisSync == nil {
			return nil, false, fmt.Errorf("failed to initialize redsync")
		}
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
		if err == redsync.ErrFailed {
			// Lock not acquired but no error occurred
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to acquire lock for key %s: %w", key, err)
	}

	return mutex, true, nil
}

// ReleaseLockWithRetry releases a previously acquired lock with retry mechanism
func ReleaseLockWithRetry(mutex *redsync.Mutex, maxRetries int) error {
	if mutex == nil {
		return fmt.Errorf("mutex is nil")
	}

	var err error

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
	defer cancel()

	for i := 0; i < maxRetries; i++ {
		// Try to release the lock
		if ok, unlockErr := mutex.UnlockContext(ctx); unlockErr == nil {
			if !ok {
				// Lock was not released but no error occurred
				err = fmt.Errorf("failed to release lock: not owner")
				time.Sleep(time.Duration(GetTrxRedisBackoff()*(i+1)) * time.Millisecond) // Exponential backoff
				continue
			}
			// Lock was successfully released
			return nil
		} else {
			// Error occurred while releasing the lock
			err = unlockErr
			time.Sleep(time.Duration(GetTrxRedisBackoff()*(i+1)) * time.Millisecond) // Exponential backoff
		}
	}

	return fmt.Errorf("failed to release lock after %d attempts: %w", maxRetries, err)
}

func GetTrxRedisBackoff() int {
	rInt := defaultRedisBackoff
	val, err := strconv.Atoi(os.Getenv("TRANSACTION_REDIS_BACKOFF"))
	if err == nil && val >= 10 {
		rInt = val
	}
	return rInt
}

func GetTrxRedisLockTimeout() time.Duration {
	rInt := 2000 // millisecond
	val, err := strconv.Atoi(os.Getenv("TRANSACTION_REDIS_LOCK_TIMEOUT"))
	if err == nil && val >= minTimeout {
		rInt = val
	}
	return time.Duration(rInt) * time.Millisecond
}
