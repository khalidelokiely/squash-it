package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func GetLRU(size int) *LRUCache {
	return NewLRUCache(size)
}

type ErrCache interface {
	Cache
	InjectError()
}
type MockLNCache struct {
	mu    sync.RWMutex
	items map[string]string
	err   bool
}

func NewLNCache(size int) *MockLNCache {
	return &MockLNCache{
		items: make(map[string]string, size),
	}
}
func (c *MockLNCache) Get(ctx context.Context, key string) (string, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if val, ok := c.items[key]; ok {
		return val, true, nil
	}
	return "", false, nil
}

func (c *MockLNCache) Set(ctx context.Context, key string, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.err {
		return errors.New("cache error")
	}
	c.items[key] = value
	return nil
}

func (c *MockLNCache) InjectError() {
	c.mu.Lock()
	c.err = true
	c.mu.Unlock()
}

func GetPipeline() *Pipeline {
	l1 := NewLNCache(100)
	l2 := NewLNCache(100)

	return NewCachePipeline(l1, l2)
}

func TestLRU_Get(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		name          string
		input         string
		expectedValue string
		expectedFound bool
	}{
		{name: "Should Find", input: "k1", expectedValue: "v1", expectedFound: true},
		{name: "Should Not Find", input: "k3", expectedValue: "", expectedFound: false},
	}

	for _, tc := range testTable {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lru := GetLRU(1)

			lru.Set(context.Background(), "k1", "v1")

			val, found, _ := lru.Get(context.Background(), tc.input)
			if found != tc.expectedFound {
				t.Errorf("expected %v, got %v", tc.expectedFound, found)
			}
			if val != tc.expectedValue {
				t.Errorf("expected %v, got %v", tc.expectedValue, val)
			}
		})
	}

}

func TestLRU_Set(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		name          string
		inputKey      string
		inputValue    string
		seedKey       string
		seedValue     string
		expectedValue *node
	}{
		{name: "add k1 v1", inputKey: "k1", inputValue: "v1", expectedValue: &node{key: "k1", value: "v1"}},
		{name: "add k2 v2", inputKey: "k2", inputValue: "v2", expectedValue: &node{key: "k2", value: "v2"}},
		{name: "change k2 to v3", inputKey: "k2", inputValue: "v3", seedKey: "k2", seedValue: "v2", expectedValue: &node{key: "k2", value: "v3"}},
	}

	for _, tc := range testTable {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lru := GetLRU(2)

			if tc.seedKey != "" {
				lru.Set(context.Background(), tc.seedKey, tc.seedValue)
			}
			lru.Set(context.Background(), tc.inputKey, tc.inputValue)
			lru.mu.Lock()
			defer lru.mu.Unlock()

			val, ok := lru.items[tc.inputKey]
			if !ok {
				t.Fatalf("expected key %q to exist, but it was not found", tc.inputKey)
			}

			if val.key != tc.expectedValue.key || val.value != tc.expectedValue.value {
				t.Errorf("expected %v, got %v", tc.expectedValue, val)
			}
		})
	}
}

func TestLRU_Evict(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		name          string
		capacity      int
		seedKeys      []string
		checkKey      string
		expectedFound bool
		expectedValue string
	}{
		{
			name:          "Should Evict Oldest From Cache",
			capacity:      1,
			seedKeys:      []string{"k1", "k2"},
			checkKey:      "k1",
			expectedFound: false,
			expectedValue: "",
		},
		{
			name:          "Should Keep Newest Item And Evict Oldest",
			capacity:      1,
			seedKeys:      []string{"k1", "k2"},
			checkKey:      "k2",
			expectedFound: true,
			expectedValue: "value_of_k2",
		},
		{
			name:          "Should Keep Newest Item And Evict Oldest Extended Capacity",
			capacity:      2,
			seedKeys:      []string{"k1", "k2", "k3"},
			checkKey:      "k1",
			expectedFound: false,
			expectedValue: "",
		},
	}

	for _, tc := range testTable {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lru := GetLRU(tc.capacity)
			ctx := context.Background()

			for _, key := range tc.seedKeys {
				lru.Set(ctx, key, "value_of_"+key)
			}

			val, found, _ := lru.Get(ctx, tc.checkKey)

			if found != tc.expectedFound {
				t.Fatalf("key %q: expected found %v, got %v", tc.checkKey, tc.expectedFound, found)
			}

			if val != tc.expectedValue {
				t.Errorf("key %q: expected value %q, got %q", tc.checkKey, tc.expectedValue, val)
			}
		})
	}
}

func TestLRU_Race(t *testing.T) {}

func TestPipeline_GetFromNearest(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		name          string
		seedL1Key     string
		seedL1Value   string
		seedL2Key     string
		seedL2Value   string
		inputKey      string
		expectedValue string
		expectedFound bool
	}{
		{
			name:          "Exists in L1 Should Not Go to L2",
			seedL1Key:     "k1",
			seedL1Value:   "v1_L1",
			seedL2Key:     "k1",
			seedL2Value:   "v1_L2",
			inputKey:      "k1",
			expectedValue: "v1_L1",
			expectedFound: true,
		},
		{
			name:          "Doesnt Exist in L1 Should Go to L2",
			seedL2Key:     "k1",
			seedL2Value:   "v1_L2",
			inputKey:      "k1",
			expectedValue: "v1_L2",
			expectedFound: true,
		},
		{
			name:          "Should not Exist in L1 Or L2",
			inputKey:      "missing_key",
			expectedValue: "",
			expectedFound: false,
		},
	}

	for _, tc := range testTable {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pipeline := GetPipeline()
			ctx := context.Background()

			if tc.seedL1Key != "" && len(pipeline.caches) > 0 {
				pipeline.caches[0].Set(ctx, tc.seedL1Key, tc.seedL1Value)
			}
			if tc.seedL2Key != "" && len(pipeline.caches) > 1 {
				pipeline.caches[1].Set(ctx, tc.seedL2Key, tc.seedL2Value)
			}

			val, found, _ := pipeline.Get(ctx, tc.inputKey)

			if found != tc.expectedFound {
				t.Errorf("expected %v, got %v", tc.expectedFound, found)
			}

			if val != tc.expectedValue {
				t.Errorf("expected %v, got %v", tc.expectedValue, val)
			}
		})
	}
}

func TestPipeline_GetFromFurthestBackfillNearest(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		name                  string
		seedL1Key             string
		seedL1Value           string
		seedL2Key             string
		seedL2Value           string
		inputKey              string
		expectedPipelineValue string
		expectedL1Value       string
		expectedFound         bool
	}{
		{
			name:                  "Exists in L2 Only Should Backfill L1",
			seedL2Key:             "k1",
			seedL2Value:           "v1_L2",
			inputKey:              "k1",
			expectedPipelineValue: "v1_L2",
			expectedL1Value:       "v1_L2",
			expectedFound:         true,
		},
		{
			name:                  "Exists in L1 And L2 Should NOT Backfill L1",
			seedL1Key:             "k1",
			seedL1Value:           "v1_L1",
			seedL2Key:             "k1",
			seedL2Value:           "v1_L2",
			inputKey:              "k1",
			expectedPipelineValue: "v1_L1",
			expectedL1Value:       "v1_L1",
			expectedFound:         true,
		},
	}

	for _, tc := range testTable {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pipeline := GetPipeline()
			ctx := context.Background()

			if tc.seedL1Key != "" && len(pipeline.caches) > 0 {
				pipeline.caches[0].Set(ctx, tc.seedL1Key, tc.seedL1Value)
			}
			if tc.seedL2Key != "" && len(pipeline.caches) > 1 {
				pipeline.caches[1].Set(ctx, tc.seedL2Key, tc.seedL2Value)
			}

			// 1. Assert the pipeline output itself
			pipeVal, pipeFound, _ := pipeline.Get(ctx, tc.inputKey)
			if pipeFound != tc.expectedFound {
				t.Fatalf("pipeline Get: expected found %v, got %v", tc.expectedFound, pipeFound)
			}
			if pipeVal != tc.expectedPipelineValue {
				t.Errorf("pipeline Get: expected value %q, got %q", tc.expectedPipelineValue, pipeVal)
			}

			if len(pipeline.caches) > 0 {
				l1Val, l1Found, _ := pipeline.caches[0].Get(ctx, tc.inputKey)
				if l1Found != tc.expectedFound {
					t.Fatalf("L1 cache: expected found %v, got %v", tc.expectedFound, l1Found)
				}
				if l1Val != tc.expectedL1Value {
					t.Errorf("L1 cache side-effect: expected %q, got %q", tc.expectedL1Value, l1Val)
				}
			}
		})
	}
}

func TestPipeline_SetToAllCachesOrError(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		name                     string
		inputKey                 string
		inputValue               string
		injectError              bool
		onAll                    bool
		expectedValue            string
		expectedFound            bool
		expectedErrorAtLeastOnce bool
	}{
		{
			name:                     "Set Value To All Caches No Error",
			inputKey:                 "k1",
			inputValue:               "v1_L2",
			injectError:              false,
			expectedValue:            "v1_L2",
			expectedFound:            true,
			expectedErrorAtLeastOnce: false,
		},
		{
			name:                     "Set Value To All Caches With Error On All",
			inputKey:                 "k1",
			inputValue:               "v1_L2",
			injectError:              true,
			onAll:                    true,
			expectedValue:            "",
			expectedFound:            false,
			expectedErrorAtLeastOnce: true,
		},
		{
			name:                     "Set Value To All Caches With Error On One",
			inputKey:                 "k1",
			inputValue:               "v1_L2",
			injectError:              true,
			onAll:                    false,
			expectedValue:            "v1_L2",
			expectedFound:            true,
			expectedErrorAtLeastOnce: true,
		},
	}

	for _, tc := range testTable {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pipeline := GetPipeline()
			ctx := context.Background()
			if tc.injectError {
				if tc.onAll {
					for _, cache := range pipeline.caches {
						if cache, ok := cache.(ErrCache); ok {
							cache.InjectError()
						}
					}
				} else {
					if len(pipeline.caches) > 1 {
						pipeline.caches[1].(ErrCache).InjectError()
					}
				}
			}

			err := pipeline.Set(ctx, tc.inputKey, tc.inputValue)

			if tc.expectedErrorAtLeastOnce {
				if err == nil {
					t.Fatalf("Set: expected error but got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Set: unexpected error: %v", err)
				}
			}

			value, found, _ := pipeline.Get(ctx, tc.inputKey)

			if found != tc.expectedFound {
				t.Fatalf("Post-Set validation: expected found %v, got %v", tc.expectedFound, found)
			}
			if value != tc.expectedValue {
				t.Errorf("Post-Set validation: expected value %q, got %q", tc.expectedValue, value)
			}
		})
	}
}
