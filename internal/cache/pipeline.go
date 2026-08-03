package cache

import "context"

type Pipeline struct {
	caches []Cache
}

func NewCachePipeline(caches ...Cache) *Pipeline {
	return &Pipeline{caches: caches}
}

// Get iterates through the different layers of caches we've initialized it with
// and tries to find the key in any of them.
// there are 2 cases:
//  1. A cache hit happens from the first layer L1 -> Return the result instantly
//  2. A cache hit happens deeper in the pipeline (i.e. L2+) -> the value is returned and backfilled
//     into all higher layers so future requests resolve optimally without leaving the
//     local network (LRU cache for squash-it)
//  3. A cache miss. boolean returns false. How we get the data later is not its responsibility
func (c *Pipeline) Get(ctx context.Context, key string) (string, bool, error) {
	var result string
	var found bool
	hitIndex := -1

	for i, cache := range c.caches {
		val, ok, err := cache.Get(ctx, key)
		if err != nil {
			continue
		}

		if ok {
			result = val
			found = true
			hitIndex = i
			break
		}
	}

	if !found {
		return "", false, nil
	}

	// Backfill only the higher layers that missed it
	for i := 0; i < hitIndex; i++ {
		err := c.caches[i].Set(ctx, key, result)
		if err != nil {
			return "", false, err
		}
	}

	return result, true, nil
}

// Set takes a key and value and sets them into all cache layers in the pipeline in reverse
// order (LN -> L1)
// if there is an error in any layer, the last Err is returned.
func (c *Pipeline) Set(ctx context.Context, key string, value string) error {
	var lastErr error

	for i := len(c.caches) - 1; i >= 0; i-- {
		// Stop executing immediately if the caller cancelled the request
		if err := ctx.Err(); err != nil {
			return err
		}

		err := c.caches[i].Set(ctx, key, value)
		if err != nil {
			lastErr = err
		}
	}
	return lastErr
}
