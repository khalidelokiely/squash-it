package filter

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

type PersistentBloomFilter struct {
	Filter
	filePath string
	interval time.Duration
}

// TODO: Allow passing bloom instance and manupulating it directly from interface

// NewPersistentBloomFilter allows creating a bloom filter that supports discovery of an existing backup binary file
// or gracefully degrades to a fresh filter
// Either case -> allows running Run() to save the filter to disk.
func NewPersistentBloomFilter(filePath string, defaultCapacity uint, interval time.Duration) (*PersistentBloomFilter, error) {
	var inner Filter

	file, err := os.Open(filePath)

	if errors.Is(err, os.ErrNotExist) {
		inner = NewBloom(defaultCapacity)
	} else if err != nil {
		return nil, fmt.Errorf("error opening bloom filter file: %w", err)
	}

	if inner == nil {
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("error reading bloom filter file: %w", err)
		}

		inner = NewFromBinary(defaultCapacity, data)
	}

	return &PersistentBloomFilter{
		Filter:   inner,
		filePath: filePath,
		interval: interval,
	}, nil
}

func (p *PersistentBloomFilter) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.Save(); err != nil {
				if errors.Is(err, ErrFilterNotReady) {
					log.Printf("skipping save: bloom filter is still unmarshaling")
				} else {
					log.Printf("error saving bloom filter: %v", err)
				}
			}
		case <-ctx.Done():
			log.Printf("SHUTDOWN: Performing final bloom filter flush...")
			if err := p.Save(); err != nil {
				log.Printf("error saving bloom filter: %v", err)
			}
			return
		}
	}
}

func (p *PersistentBloomFilter) Save() error {
	tmpPath := p.filePath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	var succeeded bool
	defer func() {
		file.Close()
		if !succeeded {
			_ = os.Remove(tmpPath)
		}
	}()

	// wrap in buffered writer to reduce OS syscall overhead
	// read: https://github.com/bits-and-blooms/bloom#serialization
	bufWriter := bufio.NewWriter(file)
	if _, err := p.Filter.WriteTo(bufWriter); err != nil {
		return fmt.Errorf("failed to write filter stream: %w", err)
	}

	if err := bufWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffered stream: %w", err)
	}

	// commit pages to physical disk storage
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file to disk: %w", err)
	}

	// Close file handle before atomic swap
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, p.filePath); err != nil {
		return fmt.Errorf("failed atomic rename: %w", err)
	}

	succeeded = true
	return nil
}
