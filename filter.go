package main

import (
	"log"
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
)

type Filter interface {
	Exists(key string) bool
	Add(key string)
	Serialize() ([]byte, error)
}

type BloomFilter struct {
	bf      *bloom.BloomFilter
	ready   bool
	backlog []string
	// mutex here is needed because bits-and-blooms/bloom is not thread-safe by design
	// read more: https://pkg.go.dev/github.com/bits-and-blooms/bloom/v3#readme-goroutine-safety
	mu sync.RWMutex
}

func NewBloomFilter(items uint) *BloomFilter {
	return &BloomFilter{
		bf:      bloom.NewWithEstimates(items, 0.01),
		ready:   true,
		backlog: nil,
	}
}

// NewFromBinary loads a serialized Bloom filter from a byte slice with a non-blocking,
// graceful fallback mechanism.
// During instantiation, it immediately returns a usable BloomFilter backed by an empty
// fallback, while a background goroutine unmarshals the historical data.
// Concurrency behavior during background loading:
//   - Reads: Exists() safely defaults to true (triggering a pessimistic service-level database lookup)
//     to prevent false negatives and data loss.
//   - Writes: Add() queues new keys into an in-memory backlog slice.
//
// When the background reload completes (or fails), the backlog is flushed into the
// active filter to ensure zero data loss, and standard Bloom filtering resumes.
//
// TODO: The main nuance to address while scaling this bloom.BloofFilter is that to enable resiliency across
//
//	multiple application instances, the remote storage location for the serialized filter must be locked.
//	A distributed lock ensures an instance safely loads and reserializes the state before another instance
//	pulls the updated copy.
func NewFromBinary(items uint, data []byte) *BloomFilter {
	fallbackBf := bloom.NewWithEstimates(items, 0.01)

	filter := &BloomFilter{
		bf:      fallbackBf,
		ready:   false,
		backlog: make([]string, 0, 1024),
	}

	go func() {
		targetBf := &bloom.BloomFilter{}
		err := targetBf.UnmarshalBinary(data)
		if err != nil {
			log.Printf("Bloom filter failed to unmarshal: %v", err)
			filter.mu.Lock()

			for _, item := range filter.backlog {
				filter.bf.AddString(item)
			}

			filter.ready = true
			filter.backlog = nil
			filter.mu.Unlock()
			return
		}

		filter.mu.Lock()
		for _, item := range filter.backlog {
			targetBf.AddString(item)
		}
		filter.bf = targetBf
		filter.ready = true
		filter.backlog = nil
		filter.mu.Unlock()
	}()

	return filter
}

func (f *BloomFilter) Exists(key string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.ready {
		return f.bf.TestString(key)
	}

	return true
}

func (f *BloomFilter) Add(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.ready {
		f.backlog = append(f.backlog, key)
		return
	}

	f.bf.AddString(key)
}

func (f *BloomFilter) Serialize() ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.bf.MarshalBinary()
}
