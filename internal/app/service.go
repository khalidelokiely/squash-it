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

var (
	ErrUnknownHash = errors.New("unknown hash token")
	ErrInvalidURL  = errors.New("invalid URL scheme or format. URL must start with http: or https: ")
)

var collisionRetryAttempts uint32 = 5

type URLService struct {
	repo     Repository
	pipeline cache.Cache
	filter   filter.Filter
	hasher   hash.Hasher
}

func NewURLService(repo Repository, pipeline cache.Cache, filter filter.Filter, hasher hash.Hasher) *URLService {
	return &URLService{
		repo:     repo,
		pipeline: pipeline,
		filter:   filter,
		hasher:   hasher,
	}
}

// CreateURL takes in a bare url
func (s *URLService) CreateURL(ctx context.Context, longURL string) (string, error) {
	if !s.isValidURL(longURL) {
		return "", ErrInvalidURL
	}

	for attempt := uint32(0); attempt < collisionRetryAttempts; attempt++ {
		hashToken, err := s.hasher.Generate8CharHash(longURL, attempt)

		if err != nil {
			return "", err
		}

		// False Positive
		if s.filter.Exists(hashToken) {
			result, err := s.lookupURLFromHash(ctx, hashToken)

			if err == nil {
				if result == longURL {
					return hashToken, nil
				}

				log.Printf("Hash collision on attempt %d: token %s belongs to %s", attempt, hashToken, result)
				continue
			}

			if !errors.Is(err, ErrNotFound) {
				return "", fmt.Errorf("unexpected error during hash lookup %w", err)
			}
		}

		pathHash, err := s.executeInsert(ctx, hashToken, longURL)

		if err == nil {
			return pathHash, nil
		}

		if errors.Is(err, ErrDuplicatedPathHash) {
			log.Printf("Hash Collision detected for token %s on attempt %d", hashToken, attempt)
			continue
		}

		return "", fmt.Errorf("failed to execute insert %w", err)
	}

	return "", fmt.Errorf("failed to generate hash for url after %d attempts", collisionRetryAttempts)
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
	return s.repo.UpdateClickCountByPathHash(ctx, hashToken)
}

// executeInsert
func (s *URLService) executeInsert(ctx context.Context, hashToken, longURL string) (string, error) {
	model := &URL{
		PathHash: hashToken,
		LongURL:  longURL,
	}

	// Persist in database
	err := s.repo.Create(ctx, model)

	if err != nil {
		return "", err
	}

	// Propagate to caches
	if err := s.pipeline.Set(ctx, model.PathHash, longURL); err != nil {
		log.Printf("Setting %s to %s failed: %v", hashToken, longURL, err)
	}

	s.filter.Add(model.PathHash)

	return model.PathHash, nil
}

// TODO: Use this method in a retry loop

func (s *URLService) lookupLongURL(ctx context.Context, longURL string) (string, error) {
	model, err := s.repo.FindByLongURL(ctx, longURL)

	if err != nil {
		return "", err
	}

	err = s.pipeline.Set(ctx, model.PathHash, longURL)

	if err != nil {
		log.Printf("Setting %s to %s failed: %v", model.PathHash, model.LongURL, err)
	}

	s.filter.Add(model.PathHash)

	return model.PathHash, nil
}

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
