package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
}

type redisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) Cache {
	return &redisCache{client: client}
}

func (r *redisCache) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return err // Includes redis.Nil if not found
	}
	return json.Unmarshal([]byte(val), dest)
}

func (r *redisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, expiration).Err()
}

func (r *redisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
