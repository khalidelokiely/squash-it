package main

import (
	"context"
	"fmt"
	"log"
)

type URLService struct {
	repo     *URLRepository
	pipeline *CachePipeline
	filter   Filter
	hasher   Hasher
}

func NewURLService(repo *URLRepository, pipeline *CachePipeline, filter Filter, hasher Hasher) *URLService {
	return &URLService{
		repo:     repo,
		pipeline: pipeline,
		filter:   filter,
		hasher:   hasher,
	}
}

// Scribble:
// Flow: /encode -> takes a url -> hash it -> bloom.Exists ->
// 												-> Doesnt Exist -> Pipeline
//																		-> Doesnt Exist -> DB FindByPathHash
//																							----> Doesnt exist -> Create
//																							----> Exists
//																	Set(KV) <-------------------------------
//						Return To User <---------- Add Bloom <--------------------
//												-> Maybe Exists -> Pipeline
//																		-> Exists
//						Return To User <---------- Add Bloom <--------------------
//																		-> Doesn't Exist

func (s *URLService) ShortenURL(ctx context.Context, url string) (string, error) {
	// Hash
	hashToken, _ := s.hasher.GenerateNCharHash(8, url, 0)

	// Check pipeline
	if _, exists, err := s.pipeline.Get(ctx, hashToken); exists {
		fmt.Printf("URL %s cache hit\n", url)
		return hashToken, err
	}

	// Doesn't exist in cache
	var err error

	fetchedModel, err := s.repo.FindByPathHash(ctx, hashToken)
	model := &URL{PathHash: hashToken, LongURL: url}

	if err != nil {
		fmt.Printf("URL %s doesn't exist creating\n", url)
		err := s.repo.Create(ctx, model)
		if err != nil {
			return "", err
		}
	}

	if fetchedModel != nil {
		model = fetchedModel
	}

	fmt.Println(model)

	fmt.Printf("URL %s propagating into caches\n", url)

	err = s.pipeline.Set(ctx, hashToken, model.LongURL)
	if err != nil {
		log.Println(err)
	}
	s.filter.Add(hashToken)

	fmt.Printf("URL %s cache miss\n", url)

	return hashToken, nil
}

func (s *URLService) GetURLFromPathHash(ctx context.Context, hashToken string) (string, error) {
	if !s.filter.Exists(hashToken) {
		return "", fmt.Errorf("%s doesn't exist", hashToken)
	}

	result, found, err := s.pipeline.Get(ctx, hashToken)

	if err != nil {
		return "", err
	}

	if found {
		return result, nil
	}

	model, err := s.repo.FindByPathHash(ctx, hashToken)

	if err != nil {
		return "", err
	}

	return model.LongURL, nil
}
