package app

import (
	"context"
	"log"
	"squash-it/internal/cache"
	"squash-it/internal/db"
	"squash-it/internal/filter"
	"squash-it/internal/hash"
	"squash-it/internal/rate"
	"squash-it/internal/router"
)

func New(
	ctx context.Context,
	database *db.Database,
	cache cache.Cache,
	filter filter.Filter,
	hasher hash.Hasher,
	router *router.Router,
	limiter rate.Limiter,
) {

	repo := NewURLRepository(database)
	if err := repo.Migrate(ctx); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	svc := NewURLService(repo, cache, filter, hasher)
	handler := NewURLShortenHandler(svc)

	NewRoutes(router, handler, limiter)
}
