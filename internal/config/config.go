package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DBName              string
	BloomFilePath       string
	BloomSaveInterval   time.Duration
	BloomCapacity       uint
	RedisHost           string
	RedisPort           string
	RedisPassword       string
	RedisTTL            time.Duration
	LRUCapacity         int
	RatePerMinute       int
	RateBurst           int
	RateCleanupInterval time.Duration
	ShutdownTimeout     time.Duration
}

func Load() Config {
	return Config{
		DBName:              getEnv("DATABASE_NAME", "squash.db"),
		BloomFilePath:       getEnv("BLOOM_BINARY_FILE_PATH", "data/squash-it.bloom.bin"),
		BloomSaveInterval:   time.Duration(getEnvInt("BLOOM_BINARY_SAVE_INTERVAL_SECONDS", 5)) * time.Second,
		BloomCapacity:       uint(getEnvInt("BLOOM_EXPECTED_CAPACITY", 1000000)),
		RedisHost:           getEnv("REDIS_HOST", "localhost"),
		RedisPort:           getEnv("REDIS_PORT", "6379"),
		RedisPassword:       getEnv("REDIS_PASSWORD", "my_secret_password"),
		RedisTTL:            time.Duration(getEnvInt("REDIS_TTL_HOURS", 168)) * time.Hour,
		LRUCapacity:         getEnvInt("CAPACITY", 100),
		RatePerMinute:       getEnvInt("RATE_PER_MINUTE", 60),
		RateBurst:           getEnvInt("BURST", 5),
		RateCleanupInterval: time.Duration(getEnvInt("CLEANUP_DURATION_SECONDS", 300)) * time.Second,
		ShutdownTimeout:     time.Duration(getEnvInt("SHUTDOWN_TIMEOUT_SECONDS", 30)) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if valStr := getEnv(key, ""); valStr != "" {
		if val, err := strconv.Atoi(valStr); err == nil {
			return val
		}
	}
	return fallback
}
