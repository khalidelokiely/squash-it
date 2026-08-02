package db

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

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
