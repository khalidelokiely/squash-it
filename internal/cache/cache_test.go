package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

// ErrCache extends Cache to allow test error injection.
type ErrCache interface {
	Cache
	InjectError()
}

// MockLNCache provides an in-memory mock implementation of the Cache interface.
type MockLNCache struct {
	mu    sync.RWMutex
	items map[string]string
	err   bool
}

// NewLNCache constructs a new MockLNCache instance.
func NewLNCache(size int) *MockLNCache {
	return &MockLNCache{
		items: make(map[string]string, size),
	}
}

// Get retrieves a key from the mock cache.
func (c *MockLNCache) Get(ctx context.Context, key string) (string, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.items[key]; ok {
		return val, true, nil
	}
	return "", false, nil
}

// Set stores a key-value pair or returns an error if error injection is active.
func (c *MockLNCache) Set(ctx context.Context, key string, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.err {
		return errors.New("cache error")
	}
	c.items[key] = value
	return nil
}

// InjectError forces the mock cache to return errors on Set operations.
func (c *MockLNCache) InjectError() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = true
}

// newTestLRU creates an LRU instance for testing purposes.
func newTestLRU(t testing.TB, capacity int) *LRUCache {
	t.Helper()
	return NewLRUCache(capacity)
}

// newTestPipeline creates a two-tier mock pipeline for testing purposes.
func newTestPipeline(t testing.TB) *Pipeline {
	t.Helper()
	l1 := NewLNCache(100)
	l2 := NewLNCache(100)
	return NewCachePipeline(l1, l2)
}

// TestLRU_Get verifies key retrieval and missing key handling in the LRU cache.
func TestLRU_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantValue string
		wantFound bool
	}{
		{name: "Found Key", input: "k1", wantValue: "v1", wantFound: true},
		{name: "Missing Key", input: "k3", wantValue: "", wantFound: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lru := newTestLRU(t, 1)

			_ = lru.Set(context.Background(), "k1", "v1")

			val, found, _ := lru.Get(context.Background(), tt.input)
			if found != tt.wantFound {
				t.Errorf("Get(%q) found = %v, want %v", tt.input, found, tt.wantFound)
			}
			if val != tt.wantValue {
				t.Errorf("Get(%q) value = %q, want %q", tt.input, val, tt.wantValue)
			}
		})
	}
}

// TestLRU_Set verifies inserting new keys and updating existing keys in the LRU cache.
func TestLRU_Set(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputKey   string
		inputValue string
		seedKey    string
		seedValue  string
		wantValue  string
	}{
		{name: "Insert First Key", inputKey: "k1", inputValue: "v1", wantValue: "v1"},
		{name: "Insert Second Key", inputKey: "k2", inputValue: "v2", wantValue: "v2"},
		{name: "Update Existing Key", inputKey: "k2", inputValue: "v3", seedKey: "k2", seedValue: "v2", wantValue: "v3"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lru := newTestLRU(t, 2)
			ctx := context.Background()

			if tt.seedKey != "" {
				_ = lru.Set(ctx, tt.seedKey, tt.seedValue)
			}
			_ = lru.Set(ctx, tt.inputKey, tt.inputValue)

			val, found, _ := lru.Get(ctx, tt.inputKey)
			if !found {
				t.Fatalf("Get(%q) key missing post-Set", tt.inputKey)
			}
			if val != tt.wantValue {
				t.Errorf("Get(%q) value = %q, want %q", tt.inputKey, val, tt.wantValue)
			}
		})
	}
}

// TestLRU_Evict verifies that capacity limits trigger least-recently-used eviction.
func TestLRU_Evict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		capacity  int
		seedKeys  []string
		checkKey  string
		wantFound bool
		wantValue string
	}{
		{
			name:      "Evict Oldest At Capacity 1",
			capacity:  1,
			seedKeys:  []string{"k1", "k2"},
			checkKey:  "k1",
			wantFound: false,
			wantValue: "",
		},
		{
			name:      "Retain Newest At Capacity 1",
			capacity:  1,
			seedKeys:  []string{"k1", "k2"},
			checkKey:  "k2",
			wantFound: true,
			wantValue: "value_of_k2",
		},
		{
			name:      "Evict Oldest At Capacity 2",
			capacity:  2,
			seedKeys:  []string{"k1", "k2", "k3"},
			checkKey:  "k1",
			wantFound: false,
			wantValue: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lru := newTestLRU(t, tt.capacity)
			ctx := context.Background()

			for _, key := range tt.seedKeys {
				_ = lru.Set(ctx, key, "value_of_"+key)
			}

			val, found, _ := lru.Get(ctx, tt.checkKey)
			if found != tt.wantFound {
				t.Fatalf("Get(%q) found = %v, want %v", tt.checkKey, found, tt.wantFound)
			}
			if val != tt.wantValue {
				t.Errorf("Get(%q) value = %q, want %q", tt.checkKey, val, tt.wantValue)
			}
		})
	}
}

// TestLRU_Race verifies thread safety under heavy concurrent read and write operations.
func TestLRU_Race(t *testing.T) {
	t.Parallel()
	lru := newTestLRU(t, 10)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key_%d", id%10)
			val := fmt.Sprintf("val_%d", id)

			_ = lru.Set(ctx, key, val)
			_, _, _ = lru.Get(ctx, key)
		}(i)
	}
	wg.Wait()
}

// TestPipeline_GetFromNearest verifies short-circuit reads from upper cache tiers.
func TestPipeline_GetFromNearest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		seedL1Key string
		seedL1Val string
		seedL2Key string
		seedL2Val string
		inputKey  string
		wantVal   string
		wantFound bool
	}{
		{
			name:      "Hit L1 Directly",
			seedL1Key: "k1",
			seedL1Val: "v1_L1",
			seedL2Key: "k1",
			seedL2Val: "v1_L2",
			inputKey:  "k1",
			wantVal:   "v1_L1",
			wantFound: true,
		},
		{
			name:      "Fallthrough To L2",
			seedL2Key: "k1",
			seedL2Val: "v1_L2",
			inputKey:  "k1",
			wantVal:   "v1_L2",
			wantFound: true,
		},
		{
			name:      "Miss All Tiers",
			inputKey:  "missing_key",
			wantVal:   "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pipeline := newTestPipeline(t)
			ctx := context.Background()

			if tt.seedL1Key != "" {
				_ = pipeline.caches[0].Set(ctx, tt.seedL1Key, tt.seedL1Val)
			}
			if tt.seedL2Key != "" {
				_ = pipeline.caches[1].Set(ctx, tt.seedL2Key, tt.seedL2Val)
			}

			val, found, _ := pipeline.Get(ctx, tt.inputKey)
			if found != tt.wantFound {
				t.Errorf("Get(%q) found = %v, want %v", tt.inputKey, found, tt.wantFound)
			}
			if val != tt.wantVal {
				t.Errorf("Get(%q) value = %q, want %q", tt.inputKey, val, tt.wantVal)
			}
		})
	}
}

// TestPipeline_GetFromFurthestBackfillNearest verifies that lower-tier cache hits backfill upper tiers.
func TestPipeline_GetFromFurthestBackfillNearest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		seedL1Key   string
		seedL1Val   string
		seedL2Key   string
		seedL2Val   string
		inputKey    string
		wantPipeVal string
		wantL1Val   string
		wantFound   bool
	}{
		{
			name:        "L2 Hit Backfills L1",
			seedL2Key:   "k1",
			seedL2Val:   "v1_L2",
			inputKey:    "k1",
			wantPipeVal: "v1_L2",
			wantL1Val:   "v1_L2",
			wantFound:   true,
		},
		{
			name:        "L1 Hit Does Not Modify L1 From L2",
			seedL1Key:   "k1",
			seedL1Val:   "v1_L1",
			seedL2Key:   "k1",
			seedL2Val:   "v1_L2",
			inputKey:    "k1",
			wantPipeVal: "v1_L1",
			wantL1Val:   "v1_L1",
			wantFound:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pipeline := newTestPipeline(t)
			ctx := context.Background()

			if tt.seedL1Key != "" {
				_ = pipeline.caches[0].Set(ctx, tt.seedL1Key, tt.seedL1Val)
			}
			if tt.seedL2Key != "" {
				_ = pipeline.caches[1].Set(ctx, tt.seedL2Key, tt.seedL2Val)
			}

			pipeVal, pipeFound, _ := pipeline.Get(ctx, tt.inputKey)
			if pipeFound != tt.wantFound {
				t.Fatalf("Pipeline Get(%q) found = %v, want %v", tt.inputKey, pipeFound, tt.wantFound)
			}
			if pipeVal != tt.wantPipeVal {
				t.Errorf("Pipeline Get(%q) value = %q, want %q", tt.inputKey, pipeVal, tt.wantPipeVal)
			}

			l1Val, l1Found, _ := pipeline.caches[0].Get(ctx, tt.inputKey)
			if l1Found != tt.wantFound {
				t.Fatalf("L1 Get(%q) found = %v, want %v", tt.inputKey, l1Found, tt.wantFound)
			}
			if l1Val != tt.wantL1Val {
				t.Errorf("L1 side-effect Get(%q) value = %q, want %q", tt.inputKey, l1Val, tt.wantL1Val)
			}
		})
	}
}

// TestPipeline_SetToAllCachesOrError verifies write operations propagate across all cache tiers.
func TestPipeline_SetToAllCachesOrError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputKey    string
		inputValue  string
		injectError bool
		onAll       bool
		wantVal     string
		wantFound   bool
		wantErr     bool
	}{
		{
			name:        "Set Success Across All Caches",
			inputKey:    "k1",
			inputValue:  "v1_L2",
			injectError: false,
			wantVal:     "v1_L2",
			wantFound:   true,
			wantErr:     false,
		},
		{
			name:        "Set Fails On All Caches",
			inputKey:    "k1",
			inputValue:  "v1_L2",
			injectError: true,
			onAll:       true,
			wantVal:     "",
			wantFound:   false,
			wantErr:     true,
		},
		{
			name:        "Set Fails On Partial Cache Tier",
			inputKey:    "k1",
			inputValue:  "v1_L2",
			injectError: true,
			onAll:       false,
			wantVal:     "v1_L2",
			wantFound:   true,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pipeline := newTestPipeline(t)
			ctx := context.Background()

			if tt.injectError {
				if tt.onAll {
					for _, c := range pipeline.caches {
						if errCache, ok := c.(ErrCache); ok {
							errCache.InjectError()
						}
					}
				} else {
					pipeline.caches[1].(ErrCache).InjectError()
				}
			}

			err := pipeline.Set(ctx, tt.inputKey, tt.inputValue)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Set(%q) error = %v, wantErr %v", tt.inputKey, err, tt.wantErr)
			}

			val, found, _ := pipeline.Get(ctx, tt.inputKey)
			if found != tt.wantFound {
				t.Fatalf("Post-Set Get(%q) found = %v, want %v", tt.inputKey, found, tt.wantFound)
			}
			if val != tt.wantVal {
				t.Errorf("Post-Set Get(%q) value = %q, want %q", tt.inputKey, val, tt.wantVal)
			}
		})
	}
}
