package paycloudhelper

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"

	"github.com/go-redis/redis/v8"
)

var redisPoolClient *redis.Client
var redisHostMem, redisPortMem, redisPasswordMem *string
var redisDbMem *int
var redisOptions *redis.Options

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

func GetRedisOptions() *redis.Options {
	return redisOptions
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
	return redisOptions
}

func GetRedisClient(redisHost, redisPort, redisPassword string, redisDb int) error {
	ctx := context.Background()

	LogI("InitRedis: Starting... %s:%s/%v", redisHost, redisPort, redisDb)
	InitRedisOptions(redis.Options{
		Addr:     redisHost + ":" + redisPort,
		Password: redisPassword,
		DB:       redisDb,
	})

	redisPoolClient = redis.NewClient(GetRedisOptions())

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

	return nil
}

func StoreRedis(id string, data interface{}, duration time.Duration) (err error) {
	// get redis client
	rClient, errCl := GetRedisPoolClient()
	if errCl != nil {
		return errCl
	}

	_, err = rClient.Ping(rClient.Context()).Result()
	if err != nil {
		LoggerErrorHub(err)
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	ctx := context.Background()
	err = rClient.Set(ctx, id, string(jsonData), duration).Err()
	if err != nil {
		return err
	}

	return nil
}

func GetRedis(id string) (result string, err error) {
	// get redis client
	rClient, errCl := GetRedisPoolClient()
	if errCl != nil {
		return "", errCl
	}

	_, err = rClient.Ping(rClient.Context()).Result()
	if err != nil {
		LoggerErrorHub(err)
		return
	}

	ctx := context.Background()
	getRedis := rClient.Get(ctx, id)
	if getRedis == nil {
		return
	}

	if err = getRedis.Err(); err != nil {
		return
	}

	return getRedis.Result()
}

func DeleteRedis(id string) (err error) {
	// get redis client
	rClient, errCl := GetRedisPoolClient()
	if errCl != nil {
		return errCl
	}

	_, err = rClient.Ping(rClient.Context()).Result()
	if err != nil {
		LoggerErrorHub(err)
		return
	}

	ctx := context.Background()
	res := rClient.Del(ctx, id)
	if res == nil {
		return
	}

	if err = res.Err(); err != nil {
		return
	}

	return
}
