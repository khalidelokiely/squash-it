package db

import (
	"database/sql"
	"errors"
	"strings"

	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var ErrDuplicateKey = errors.New("error: duplicate key")

type Database struct {
	*sql.DB
}

func NewSQLite(database string) *Database {
	if database == "" {
		database = "data/squash-it.db"
	}

	if len(strings.Split(database, ".")) == 1 {
		database = database + ".db"
	}

	if len(strings.Split(database, "/")) == 1 {
		database = "data/" + database
	}

	db, err := sql.Open("sqlite", database)

	if err != nil {
		panic(err)
	}

	return &Database{db}
}

func (s *Database) MapError(err error) error {
	if err == nil {
		return nil
	}

	if liteErr, ok := errors.AsType[*sqlite.Error](err); ok {
		code := liteErr.Code()
		if code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY || code == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return ErrDuplicateKey
		}
	}
	return err
}
