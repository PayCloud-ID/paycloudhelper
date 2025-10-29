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

func InitializeRedis(opt redis.Options) {
	LogI("InitRedis: Start ...")

	redisLockKey = fmt.Sprintf("redis_lock:%s:", GetAppName())

	// Initialize Redis options with default values
	InitRedisOptions(opt)

	// Initialize Redis client
	err := initRedisClient(GetRedisOptions())
	if err != nil {
		LogE("InitRedis: Failed to initialize Redis client: %s", err.Error())
	}
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
		errMsg := fmt.Sprintf("failed to initialize redsync: %v", err)
		return false, fmt.Errorf(errMsg)
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
			// Lock not acquired but no error occurred
			return false, nil
		}
		return false, fmt.Errorf("failed to acquire lock for key %s: %w", key, err)
	}

	// Store the mutex in a map for later release
	StoreMutex(key, mutex)

	return true, nil
}

func ReleaseLock(key string) error {
	mutex := GetMutex(key)
	if mutex == nil {
		errMsg := fmt.Sprintf("no mutex found for key %s", key)
		return fmt.Errorf(errMsg)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRedisTimeout)
	defer cancel()

	ok, err := mutex.UnlockContext(ctx)
	if err != nil {
		errMsg := fmt.Sprintf("failed to release lock for key %s: %v", key, err)
		return fmt.Errorf(errMsg)
	}

	if !ok {
		errMsg := fmt.Sprintf("failed to release lock for key %s: not owner", key)
		return fmt.Errorf(errMsg)
	}

	// Remove the mutex from the map
	RemoveMutex(key)

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
