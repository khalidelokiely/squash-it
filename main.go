package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"squash-it/internal/app"
	"squash-it/internal/cache"
	"squash-it/internal/db"
	filter2 "squash-it/internal/filter"
	"squash-it/internal/hash"
	"squash-it/internal/rate"
	"squash-it/internal/router"
	"sync"
	"syscall"
	"time"
)

func main() {
	database := db.NewSQLite("squash.db")
	defer func(database *db.Database) {
		err := database.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(database)
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS urls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id TEXT NOT NULL UNIQUE,
			path_hash TEXT NOT NULL,
			long_url TEXT NOT NULL,
			click_count INTEGER NOT NULL DEFAULT 0,
			long_url_safe INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_by TEXT NOT NULL,
			deleted_at DATETIME DEFAULT NULL,          -- Nullable by default
			deleted_by TEXT DEFAULT NULL,              -- Nullable by default
			deleted_reason TEXT DEFAULT NULL           -- Nullable by default
		);
	`)

	if err != nil {
		log.Fatal(err)
	}

	r := router.NewRouter()
	var filter *filter2.Bloom
	file, err := os.ReadFile("data/squash-it.bloom.bin")

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			filter = filter2.NewBloom(100000)
		}
	}

	if file != nil {
		filter = filter2.NewFromBinary(100000, file)
	}

	repository := app.NewURLRepository(database)
	lru := cache.NewLRUCache(100)
	redis := cache.NewRedisCache(24 * 7 * time.Hour)

	pipeline := cache.NewCachePipeline(lru, redis)

	hasher := hash.NewMurmurHash(6)

	svc := app.NewURLService(repository, pipeline, filter, hasher)

	h := app.NewURLShortenHandler(svc)

	limiter := rate.NewUserTokenBucket(60, 3)

	r.POST("/encode", h.EncodeURL, app.RateLimiterMiddleware(limiter))
	r.POST("/decode", h.DecodeURL, app.RateLimiterMiddleware(limiter))
	r.GET("/{hashToken}", h.VisitURL)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		saveToFile := func(filePath string) error {
			tmpPath := filePath + ".tmp"
			file, err := os.Create(tmpPath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			// Guarantees no dangling .tmp file if writing/renaming fails!
			var success bool
			defer func() {
				file.Close()
				if !success {
					_ = os.Remove(tmpPath) // Cleanup on error/panic
				}
			}()

			if _, err := filter.WriteTo(file); err != nil {
				return fmt.Errorf("failed to write to file: %w", err)
			}

			// Flush memory buffers to disk & close handle BEFORE renaming
			if err := file.Sync(); err != nil {
				return fmt.Errorf("failed to sync file: %w", err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("failed to close file: %w", err)
			}

			file = nil

			// Safe atomic swap
			if err := os.Rename(tmpPath, filePath); err != nil {
				return fmt.Errorf("failed to rename file: %w", err)
			}

			success = true // Prevents defer from deleting the file
			fmt.Printf("[%s] Filter successfully saved to %s\n", time.Now().Format("2006-01-02 15:04:05"), filePath)
			return nil
		}

		for {
			select {
			case <-ticker.C:
				fmt.Println("Calling serialize on bloom filter")
				err := saveToFile("data/squash-it.bloom.bin")
				if err != nil {
					log.Printf("failed to save bloom filter: %v", err)
				}
			case <-ctx.Done():
				fmt.Println("SHUTDOWN: Serializaing Bloom")
				err := saveToFile("data/squash-it.bloom.bin")
				if err != nil {
					log.Printf("failed to save bloom filter: %v", err)
				}
				return
			}
		}
	}()

	server := r.Spin()

	<-ctx.Done()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	fmt.Println("Shutting down. Waiting for Background goroutine to finish")
	wg.Wait()

}
