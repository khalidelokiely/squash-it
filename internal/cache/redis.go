package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	rdb        *redis.Client
	defaultTTL time.Duration
}

func NewRedisCache(host, port, password string, defaultTTL time.Duration) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       0,
	})
	return &RedisCache{
		rdb:        rdb,
		defaultTTL: defaultTTL,
	}
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", false, nil
		}
		return "", false, err
	}
	return val, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value string) error {
	return c.rdb.Set(ctx, key, value, c.defaultTTL).Err()
}

func (c *RedisCache) Ping() error {
	return c.rdb.Ping(context.Background()).Err()
}
