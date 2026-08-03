package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"squash-it/internal/cache"
	"squash-it/internal/filter"
	"squash-it/internal/hash"
)

var ErrUnknownHash = errors.New("unknown hash token")

type URLService struct {
	repo     *URLRepository
	pipeline *cache.Pipeline
	filter   filter.Filter
	hasher   hash.Hasher
}

func NewURLService(repo *URLRepository, pipeline *cache.Pipeline, filter filter.Filter, hasher hash.Hasher) *URLService {
	return &URLService{
		repo:     repo,
		pipeline: pipeline,
		filter:   filter,
		hasher:   hasher,
	}
}

// CreateURL takes in a bare url
func (s *URLService) CreateURL(ctx context.Context, longURL string) (string, error) {
	maxAttempts := uint32(5)

	for attempt := uint32(0); attempt < maxAttempts; attempt++ {
		hashToken, err := s.hasher.Generate8CharHash(longURL, attempt)

		if err != nil {
			return "", err
		}

		if !s.filter.Exists(hashToken) {
			err := s.executeInsert(ctx, hashToken, longURL)
			return hashToken, err
		}

		existingURL, err := s.lookupURLFromHash(ctx, hashToken)

		if err != nil {
			if errors.Is(err, ErrNotFound) {
				err := s.executeInsert(ctx, hashToken, longURL)
				return hashToken, err
			}

			fmt.Println(err.Error())

			return "", fmt.Errorf("failed to verify url hashToken: %w", err)
		}

		if existingURL == longURL {
			return hashToken, nil
		}

		// Collision
		log.Printf("Hash Collision detected for token %s on attempt %d", hashToken, attempt)
	}

	return "", fmt.Errorf("failed to generate hash for url after %d attempts", maxAttempts)
}

func (s *URLService) GetURLFromHash(ctx context.Context, hashToken string) (string, error) {
	longURL, err := s.lookupURLFromHash(ctx, hashToken)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrUnknownHash
		}
		return "", err
	}
	return longURL, nil
}

func (s *URLService) UpdateClickCount(ctx context.Context, hashToken string) error {
	return s.repo.UpdateClickCount(ctx, hashToken)
}

// executeInsert
func (s *URLService) executeInsert(ctx context.Context, hashToken, longURL string) error {
	model := &URL{
		PathHash: hashToken,
		LongURL:  longURL,
	}

	// Persist in Repository
	err := s.repo.Create(ctx, model)

	if err != nil {
		return err
	}

	// Propagate to caches
	if err := s.pipeline.Set(ctx, hashToken, longURL); err != nil {
		log.Printf("Setting %s to %s failed: %v", hashToken, longURL, err)
	}

	// Add to filter
	s.filter.Add(hashToken)

	return nil
}

// TODO: Use this method in a retry loop

// lookupURLFromHash
func (s *URLService) lookupURLFromHash(ctx context.Context, hashToken string) (string, error) {
	// Doesn't Exist in bloom - fail fast
	if !s.filter.Exists(hashToken) {
		log.Printf("Token not found in Bloom Filter")
		return "", ErrNotFound
	}

	// Bloom couldn't assert it doesn't exist - Check cache
	result, found, err := s.pipeline.Get(ctx, hashToken)

	if err != nil {
		log.Printf("Error getting %s from cache: %v", hashToken, err)
	}

	if found {
		log.Printf("Cache HIT for hash token %s", hashToken)
		return result, nil
	}

	log.Printf("Cache MISS for hash token %s", hashToken)
	model, err := s.repo.FindByPathHash(ctx, hashToken)

	if err != nil {
		return "", err
	}

	if err := s.pipeline.Set(ctx, hashToken, model.LongURL); err != nil {
		log.Printf("Setting %s to %s failed: %v", hashToken, model.LongURL, err)
	}

	return model.LongURL, nil
}

// verifyURL
func (s *URLService) isValidURL(longURL string) bool {
	u, err := url.ParseRequestURI(longURL)
	if err != nil {
		return false
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	return true
}
