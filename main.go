package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"squash-it/internal/app"
	"squash-it/internal/cache"
	"squash-it/internal/config"
	"squash-it/internal/db"
	"squash-it/internal/filter"
	"squash-it/internal/hash"
	"squash-it/internal/rate"
	"squash-it/internal/router"
)

func main() {
	// This context also helps in case of scaling - usually before a pod is killed, it gets sent a SIGTERM from k8s
	// then after a timeout, gets a hard SIGKILL.
	// In the local demo it works by listening to ctrl+c from terminal.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration. See .env.example
	cfg := config.Load()

	database := db.NewSQLite(cfg.DBName)
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("error closing database: %v", err)
		}
	}()

	// Create Pipeline Cache
	pipeline := cache.NewPipelineFromConfig(cfg)

	// Create Bloom
	bloom, err := filter.NewPersistentFromConfig(cfg)
	if err != nil {
		log.Fatalf("bloom filter initialization failed: %v", err)
	}

	// Create MumurHasher
	hasher := hash.NewMurmurHash(6)

	// Create Rate Limiter
	limiter := rate.NewUserTokenBucket(cfg.RatePerMinute, cfg.RateBurst, cfg.RateCleanupInterval)

	r := router.NewRouter(router.WithHostPorts(cfg.Port))

	// Boot up app and HTTP Routes
	app.New(ctx, database, pipeline, bloom, hasher, r, limiter)

	// Spin background worker to periodically flush bloom
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		bloom.Run(ctx)
	}()

	server := r.Spin()

	<-ctx.Done()
	log.Println("Shutdown signal received...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	wg.Wait()
	log.Println("Shutdown complete.")
}
