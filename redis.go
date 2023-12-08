package qoinhubhelper

import (
	"context"
	"encoding/json"
	"log"

	"time"

	redis "github.com/go-redis/redis/v8"
)

var redisPoolClient *redis.Client
var redisHostMem, redisPortMem, redisPasswordMem *string
var redisDbMem *int

func GetRedisClient(redisHost, redisPort, redisPassword string, redisDb int) error {

	ctx := context.Background()
	redisPoolClient = redis.NewClient(&redis.Options{
		Addr:       redisHost + ":" + redisPort,
		Password:   redisPassword,
		DB:         redisDb,
		MaxRetries: 3,
		PoolSize:   50,
		MaxConnAge: 5 * time.Minute,
	})

	res, err := redisPoolClient.Ping(ctx).Result()
	if err != nil {
		LoggerErrorHub(err)
		log.Println("open redis pool connection failed")
		return err
	}

	log.Println("open redis pool connection successfully")
	log.Println(res)

	redisHostMem = &redisHost
	redisPortMem = &redisPort
	redisPasswordMem = &redisPassword
	redisDbMem = &redisDb

	return nil
}

func StoreRedis(id string, data interface{}, duration time.Duration) (err error) {

	// open new pool connection if previously memory address pool connection is nil
	if redisPoolClient == nil {
		GetRedisClient(*redisHostMem, *redisPortMem, *redisPasswordMem, *redisDbMem)
	}

	_, err = redisPoolClient.Ping(redisPoolClient.Context()).Result()
	if err != nil {
		LoggerErrorHub(err)
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	ctx := context.Background()
	err = redisPoolClient.Set(ctx, id, string(jsonData), duration).Err()
	if err != nil {
		return err
	}

	return nil
}

func GetRedis(id string) (result string, err error) {

	// open new pool connection if previously memory address pool connection is nil
	if redisPoolClient == nil {
		GetRedisClient(*redisHostMem, *redisPortMem, *redisPasswordMem, *redisDbMem)
	}

	_, err = redisPoolClient.Ping(redisPoolClient.Context()).Result()
	if err != nil {
		LoggerErrorHub(err)
		return
	}

	ctx := context.Background()
	getRedis := redisPoolClient.Get(ctx, id)
	if getRedis == nil {
		return
	}

	if err = getRedis.Err(); err != nil {
		return
	}

	return getRedis.Result()
}

func DeleteRedis(id string) (err error) {

	// open new pool connection if previously memory address pool connection is nil
	if redisPoolClient == nil {
		GetRedisClient(*redisHostMem, *redisPortMem, *redisPasswordMem, *redisDbMem)
	}

	_, err = redisPoolClient.Ping(redisPoolClient.Context()).Result()
	if err != nil {
		LoggerErrorHub(err)
		return
	}

	delete := redisPoolClient.Del(context.Background(), id)
	if delete == nil {
		return
	}

	if err = delete.Err(); err != nil {
		return
	}

	return
}
