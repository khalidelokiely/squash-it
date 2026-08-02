package main

import (
	"context"
	"fmt"
	"log"
	"squash-it/pkg/db"
	"strings"
)

type UpdateMap map[string]interface{}

type URLRepository struct {
	db        *db.Database
	tableName string
}

func NewURLRepository(db *db.Database) *URLRepository {
	return &URLRepository{
		db:        db,
		tableName: "urls",
	}
}

func (r *URLRepository) FindByPathHash(ctx context.Context, pathHash string) (*URL, error) {
	var u URL

	columns := strings.Join(u.Columns(), ", ")

	query := fmt.Sprintf(`SELECT %s 
								FROM %s 
								WHERE path_hash = ?`,
		columns, r.tableName)

	err := r.db.QueryRowContext(ctx, query, pathHash).
		Scan(u.ScanDest()...)

	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *URLRepository) Create(ctx context.Context, url *URL) error {
	columns := strings.Join(url.Columns(), ", ")
	placeholders := r.getPlaceholders(2)

	query := fmt.Sprintf(`INSERT INTO %s (path_hash, long_url)
								VALUES (%s) 
								RETURNING %s`,
		r.tableName, placeholders, columns)

	err := r.db.QueryRowContext(ctx, query, url.PathHash, url.LongURL).
		Scan(url.ScanDest()...)

	if err != nil {

		log.Printf(err.Error())
		return err
	}
	return nil
}

func (r *URLRepository) Update(ctx context.Context, id uint64, updateMap UpdateMap) (*URL, error) {
	return nil, nil
}

func (r *URLRepository) SDelete(ctx context.Context, id uint64) error {
	return nil
}

func (r *URLRepository) HDelete(ctx context.Context, id uint64) error {
	return nil
}

func (r *URLRepository) getPlaceholders(count int) string {
	placeholders := make([]string, count)

	for i := 0; i < count; i++ {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}
