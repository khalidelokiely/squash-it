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

type Repository interface {
	// FindByLongURL takes in the long representation of a URL and returns a pointer to a URL with the database
	// or error with nil pointer
	FindByLongURL(ctx context.Context, longURL string) (*URL, error)

	// FindByPathHash takes in the pathHash (hashToken) and returns a pointer to a URL with the database.
	// or error with nil pointer.
	// Note: the pathHash is NOT a complete URL. It's the first {pathHash} named parameter in the endpoint or
	//		 the pathHash ("path_hash" as JSON field) Field in URLDecodeDTO
	FindByPathHash(ctx context.Context, pathHash string) (*URL, error)

	// Create idempotent creation using pathHash and longURL.
	// On LongURL conflict: get the record and return it - no errors
	// On PathHash conflict: return ErrDuplicatedPathHash error
	Create(ctx context.Context, url *URL) error

	// UpdateClickCountByPathHash increments the click_count field in the table by 1
	UpdateClickCountByPathHash(ctx context.Context, pathHash string) error
}

var ErrNotFound = errors.New("url record not found")
var ErrDuplicatedPathHash = errors.New("duplicated path hash")

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

func (r *URLRepository) FindByLongURL(ctx context.Context, longURL string) (*URL, error) {
	return r.findBy(ctx, "long_url", longURL)
}

func (r *URLRepository) FindByPathHash(ctx context.Context, pathHash string) (*URL, error) {

	return r.findBy(ctx, "path_hash", pathHash)
}

func (r *URLRepository) findBy(ctx context.Context, col string, params ...any) (*URL, error) {
	var u URL

	columns := strings.Join(u.Columns(), ", ")

	query := fmt.Sprintf(`SELECT %s
								FROM %s
								WHERE %s = ?`, columns, r.tableName, col)

	err := r.db.QueryRowContext(ctx, query, params...).Scan(u.ScanDest()...)

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

	query := fmt.Sprintf(`INSERT INTO %s (path_hash, long_url)
								VALUES (?, ?)
								ON CONFLICT (long_url) DO UPDATE SET long_url = EXCLUDED.long_url
								RETURNING %s`,
		r.tableName, columns)

	err := r.db.QueryRowContext(ctx, query, url.PathHash, url.LongURL).
		Scan(url.ScanDest()...)

	if err != nil {
		if errors.Is(r.db.MapError(err), db.ErrDuplicateKey) {
			return ErrDuplicatedPathHash
		}

		return fmt.Errorf("repository query failed: %w", err)
	}

	return nil
}

func (r *URLRepository) UpdateClickCountByPathHash(ctx context.Context, pathHash string) error {
	query := fmt.Sprintf(`UPDATE %s SET click_count = click_count + 1 WHERE path_hash = ?`, r.tableName)
	result, err := r.db.ExecContext(ctx, query, pathHash)

	if err != nil {
		return fmt.Errorf("execute query failed: %w", err)
	}

	rows, err := result.RowsAffected()

	if err != nil {
		return fmt.Errorf("rows affected check: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no rows affected to click count update")
	}

	return nil
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
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}

		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("transaction rollback failed: %v", err)
		}
	}()

	if err = fn(tx); err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transaction commit failed: %w", err)
	}
	return nil

}

func (r *URLRepository) Migrate(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS urls
				(
					 id             INTEGER PRIMARY KEY AUTOINCREMENT,
					 path_hash      TEXT     NOT NULL UNIQUE,
					 long_url       TEXT     NOT NULL UNIQUE,
					 click_count    INTEGER  NOT NULL DEFAULT 0,
					 long_url_safe  INTEGER  NOT NULL DEFAULT 1,
					 created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
					 created_by     TEXT     NOT NULL DEFAULT 'go.api',
					 deleted_at     DATETIME DEFAULT NULL,
					 deleted_by     TEXT     DEFAULT NULL,
					 deleted_reason TEXT     DEFAULT NULL
				  );
	`)

	return err
}
