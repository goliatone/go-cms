package testsupport

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func NewSQLiteMemoryDB() (*sql.DB, error) {
	return sql.Open("sqlite3", "file::memory:?cache=shared")
}
