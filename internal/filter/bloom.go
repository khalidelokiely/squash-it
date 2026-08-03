package filter

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
)

var ErrFilterNotReady = errors.New("bloom filter is currently restoring from binary")

type Bloom struct {
	bf      *bloom.BloomFilter
	ready   bool
	backlog []string
	// mutex here is needed because bits-and-blooms/bloom is not thread-safe by design
	// read more: https://pkg.go.dev/github.com/bits-and-blooms/bloom/v3#readme-goroutine-safety
	mu sync.RWMutex
}

func NewBloom(items uint) *Bloom {
	return &Bloom{
		bf:      bloom.NewWithEstimates(items, 0.01),
		ready:   true,
		backlog: nil,
	}
}

// NewFromBinary loads a serialized Bloom filter from a byte slice with a non-blocking,
// graceful fallback mechanism.
// During instantiation, it immediately returns a usable Bloom backed by an empty
// fallback, while a background goroutine unmarshals the historical data.
// Concurrency behavior during background loading:
//   - Reads: Exists() safely defaults to true (triggering a pessimistic service-level database lookup)
//     to prevent false negatives and data loss.
//   - Writes: Add() queues new keys into an in-memory backlog slice.
//
// When the background reload completes (or fails), the backlog is flushed into the
// active filter to ensure zero data loss, and standard Bloom filtering resumes.
//
// TODO: The main nuance to address while scaling this bloom.BloofFilter is that
//
//	to enable resiliency across multiple application instances, the remote
//	storage location for the serialized filter must be locked. A distributed
//	lock ensures an instance safely loads and reserializes the state before
//	another instance pulls the updated copy.
func NewFromBinary(items uint, data []byte) *Bloom {
	fmt.Println("found bloom filter, begnnning restore")
	fallbackBf := bloom.NewWithEstimates(items, 0.01)

	filter := &Bloom{
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

func (b *Bloom) Exists(key string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.ready {
		return b.bf.TestString(key)
	}

	return true
}

func (b *Bloom) Add(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.ready {
		b.backlog = append(b.backlog, key)
		return
	}

	b.bf.AddString(key)
}

func (b *Bloom) Serialize() ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.ready {
		return nil, ErrFilterNotReady
	}

	return b.bf.MarshalBinary()
}

// WriteTo takes an io.Writer and writes the existing filter to it.
// TODO: If !b.ready. Save the backlog instead as a json in disk. This way if
//
//	the filter application dies before we could fully unmarshal, we have
//	the WAL ready to replay.
func (b *Bloom) WriteTo(w io.Writer) (int64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.ready {
		return 0, ErrFilterNotReady
	}

	return b.bf.WriteTo(w)
}
