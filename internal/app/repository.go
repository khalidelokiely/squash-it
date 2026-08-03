package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"squash-it/internal/db"
	"strings"
)

var ErrNotFound = errors.New("url record not found")

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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("repository query timed out: %w", err)
		}
		return nil, fmt.Errorf("repository query failed: %w", err)
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
		return err
	}
	return nil
}

func (r *URLRepository) UpdateClickCount(ctx context.Context, pathHash string) error {
	return r.TransactionSerializable(ctx, func(tx *sql.Tx) error {
		query := fmt.Sprintf(`UPDATE %s SET click_count = click_count + 1 WHERE path_hash = ?`, r.tableName)
		result, err := tx.ExecContext(ctx, query, pathHash)

		if err != nil {
			return fmt.Errorf("execute query failed: %w", err)
		}

		rows, err := result.RowsAffected()

		if err != nil {
			return fmt.Errorf("rows affected check: %w", err)
		}

		if rows == 0 {
			return fmt.Errorf("no rows updated for hash: %s", pathHash)
		}

		return nil
	})
}

func (r *URLRepository) TransactionSerializable(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return r.txWithIso(ctx, sql.LevelSerializable, fn)
}

func (r *URLRepository) TransactionReadCommited(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return r.txWithIso(ctx, sql.LevelReadCommitted, fn)
}

func (r *URLRepository) TransactionRepeatableRead(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return r.txWithIso(ctx, sql.LevelRepeatableRead, fn)
}

func (r *URLRepository) txWithIso(ctx context.Context, level sql.IsolationLevel, fn func(tx *sql.Tx) error) error {
	opts := &sql.TxOptions{Isolation: level, ReadOnly: false}
	tx, err := r.db.BeginTx(ctx, opts)

	if err != nil {
		// unable to begin transaction
		return fmt.Errorf("transaction begin failed: %w", err)
	}

	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("transaction rollback failed: %v", err)
		}
	}()

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	err = fn(tx)

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transaction commit failed: %w", err)
	}
	return nil

}

func (r *URLRepository) getPlaceholders(count int) string {
	placeholders := make([]string, count)

	for i := 0; i < count; i++ {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}
