package filter

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

// newTestFilter creates a Filter instance for testing purposes.
func newTestFilter(t testing.TB, items uint) Filter {
	t.Helper()
	return NewBloom(items)
}

func newTestFilterFromBinary(t testing.TB, items uint, data []byte) Filter {
	t.Helper()
	return NewFromBinary(items, data)
}

func TestFilter_NoFalseNegative(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		addKey    string
		searchKey string
		want      bool
	}{
		{
			name:      "Key Added Must Assert Positive (Maybe Exists) Not Negative (Does Not Exist)",
			addKey:    "k1",
			searchKey: "k1",
			want:      true,
		},
		{
			name:      "Empty Filter Must Assert Negative",
			searchKey: "k1",
			want:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			filter := newTestFilter(t, 100)
			if tt.addKey != "" {
				filter.Add(tt.addKey)
			}

			if tt.want != filter.Exists(tt.searchKey) {
				t.Fatalf("want %v, got %v", tt.want, filter.Exists(tt.searchKey))
			}
		})
	}
}

// TestFilter_NoFalseNegativeAfterRestore
func TestFilter_NoFalseNegativeAfterRestore(t *testing.T) {
	t.Parallel()
	capacity := uint(100)
	original := newTestFilter(t, capacity)
	original.Add("k1")

	t.Run("Restore From Serialize()", func(t *testing.T) {
		data, err := original.Serialize()
		if err != nil {
			t.Fatalf("serialization failed: %v", err)
		}

		restored := newTestFilterFromBinary(t, capacity, data)
		if !restored.Exists("k1") {
			t.Fatalf("restored filter lost key 'k1'")
		}
	})

	t.Run("Restore From WriteTo()", func(t *testing.T) {
		var buf bytes.Buffer
		if _, err := original.WriteTo(&buf); err != nil {
			t.Fatalf("write to failed: %v", err)
		}

		restored := newTestFilterFromBinary(t, capacity, buf.Bytes())
		if !restored.Exists("k1") {
			t.Fatalf("restored filter lost key 'k1'")
		}
	})
}

// TestFilter_Race verifies thread safety under heavy concurrent read and write operations.
func TestFilter_Race(t *testing.T) {
	t.Parallel()
	filter := newTestFilter(t, 100000)

	var wg sync.WaitGroup
	const totalGoroutines = 5000
	for i := 0; i < totalGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key_%d", id%10)
			filter.Add(key)
			filter.Exists(key)
		}(i)
	}
	wg.Wait()

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key_%d", i)
		if !filter.Exists(key) {
			t.Fatalf("expected key %s to exist after concurrent insertions", key)
		}
	}
}
