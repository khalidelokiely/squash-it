package main

import (
	"log"
	"squash-it/pkg/db"
	"squash-it/router"
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

		DELETE FROM urls;
	`)

	if err != nil {
		log.Fatal(err)
	}

	repository := NewURLRepository(database)
	//
	//url, err := repository.FindByPathHash(context.Background(), "abdfi")
	//
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//fmt.Println(url)
	//
	//url = &URL{
	//	PathHash:    "ufodhjfl",
	//	LongURL:     "www.google.com",
	//	ClickCount:  0,
	//	LongURLSafe: true,
	//	CreatedAt:   time.Now(),
	//	CreatedBy:   "sys",
	//}
	//
	//err = repository.Create(context.Background(), url)
	//
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//fmt.Println(url)

	r := router.NewRouter()
	filter := NewBloomFilter(1000)
	lru := NewLRUCache(100)

	redis := NewRedisCache(24 * 7 * time.Hour)

	pipeline := NewCachePipeline(lru, redis)

	hasher := NewMurmurHash(6)

	svc := NewURLService(repository, pipeline, filter, hasher)

	h := NewURLShortenHandler(svc)

	r.POST("/encode", h.EncodeURL)
	r.POST("/decode", h.DecodeURL)
	r.GET("/{hashToken}", h.VisitURL)
	r.Spin()

}
