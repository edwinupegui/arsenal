package store

import (
	"database/sql"
	"embed"

	"github.com/edwinupegui/arsenal/internal/sqliteutil"
)

// Open opens a connection to the arsenal SQLite database. Thin wrapper around
// sqliteutil.Open kept for backward compatibility — new code should depend on
// the sqliteutil package directly.
func Open(path string) (*sql.DB, error) {
	return sqliteutil.Open(path)
}

// Migrate applies pending migrations from the embedded FS. Thin wrapper
// around sqliteutil.Migrate kept for backward compatibility.
func Migrate(db *sql.DB, fsys embed.FS, dir string) error {
	return sqliteutil.Migrate(db, fsys, dir)
}
