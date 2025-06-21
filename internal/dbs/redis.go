package dbs

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

func InitRedis(ctx context.Context) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
		return err
	}

	return nil
}

func CloseRedis() {
	if RedisClient != nil {
		_ = RedisClient.Close()
	}
}
