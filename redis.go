package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	rdb        *redis.Client
	defaultTTL time.Duration
}

func NewRedisCache(defaultTTL time.Duration) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "my_secret_password",
		DB:       0,
	})
	rdb.FlushDB(context.Background())

	return &RedisCache{
		rdb:        rdb,
		defaultTTL: defaultTTL,
	}
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := c.rdb.Get(ctx, key).Result()

	if err != nil {
		return "", false, err
	}

	return val, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value string) error {
	return c.rdb.Set(ctx, key, value, c.defaultTTL).Err()
}
