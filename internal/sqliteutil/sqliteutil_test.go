package sqliteutil

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesUsableDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arsenal.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// foreign_keys is a per-connection pragma; reading it back verifies
	// the DSN pragma was honored.
	var fk int
	if err := db.QueryRowContext(context.Background(), `PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}

	// journal_mode is set by DSN; once it returns "wal" once it sticks.
	var mode string
	if err := db.QueryRowContext(context.Background(), `PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
}

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arsenal.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(context.Background(),
		`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	err = WithTx(context.Background(), db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(),
			`INSERT INTO t (v) VALUES (?)`, "hello")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM t`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row, got %d", n)
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arsenal.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(context.Background(),
		`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	want := errors.New("boom")
	err = WithTx(context.Background(), db, func(tx *sql.Tx) error {
		_, _ = tx.ExecContext(context.Background(),
			`INSERT INTO t (v) VALUES (?)`, "should-rollback")
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("WithTx err = %v, want %v", err, want)
	}

	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM t`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", n)
	}
}
