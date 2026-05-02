package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // register pure-Go sqlite driver
)

// Open returns a *sql.DB connected to a SQLite file at path with sane pragmas
// for a single-process local app: WAL, foreign keys on, synchronous=NORMAL.
func Open(path string) (*sql.DB, error) {
	dsn := path +
		"?_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

// Migrate applies all pending migrations from the embedded FS using goose.
// Goose's default logger is silenced — the CLI is responsible for surfacing
// progress when --verbose is set.
func Migrate(db *sql.DB, fsys embed.FS, dir string) error {
	goose.SetBaseFS(fsys)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
