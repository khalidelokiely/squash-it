package cache

import "context"

type Cache interface {
	// Get fetch the value from the cache
	Get(ctx context.Context, key string) (string, bool, error)

	// Set a k/v pair into the cache
	Set(ctx context.Context, key string, value string) error
}
