package cache

import (
	"log"
	"squash-it/internal/config"
)

// NewPipelineFromConfig gracefully degrades to LRU only if REDIS is unresponsive
func NewPipelineFromConfig(cfg config.Config) Cache {
	var caches []Cache

	caches = append(caches, NewLRUCache(cfg.LRUCapacity))

	redis := NewRedisCache(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, cfg.RedisTTL)
	if err := redis.Ping(); err != nil {
		log.Printf("warning: Redis connection failed (%s:%s). Running LRU-only.", cfg.RedisHost, cfg.RedisPort)
	} else {
		caches = append(caches, redis)
	}

	return NewCachePipeline(caches...)
}
