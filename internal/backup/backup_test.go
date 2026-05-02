package backup_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/edwinupegui/arsenal/internal/backup"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
)

// helper: open a fresh migrated DB at a unique path inside t.TempDir().
func newDB(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Seed a row so we can assert count post-snapshot.
	if _, err := db.Exec(`
		INSERT INTO resources (title, url, type, language)
		VALUES ('Test', 'https://example.com/x', 'article', 'EN')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = db.Close()
	return path
}

func TestSnapshot_RoundTripPreservesRows(t *testing.T) {
	srcPath := newDB(t, "src.db")
	dest := filepath.Join(t.TempDir(), "snap.db")

	src, err := store.Open(srcPath)
	if err != nil {
		t.Fatalf("open src: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	if err := backup.Snapshot(context.Background(), src, dest); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	snap, err := store.Open(dest)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	t.Cleanup(func() { _ = snap.Close() })

	var n int
	if err := snap.QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("snapshot has %d rows, want 1", n)
	}
}

func TestSnapshot_RefusesOverwrite(t *testing.T) {
	srcPath := newDB(t, "src.db")
	src, err := store.Open(srcPath)
	if err != nil {
		t.Fatalf("open src: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	dest := filepath.Join(t.TempDir(), "snap.db")
	if err := backup.Snapshot(context.Background(), src, dest); err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if err := backup.Snapshot(context.Background(), src, dest); err == nil {
		t.Fatal("expected error when overwriting existing file")
	}
}

func TestDefaultPath_Format(t *testing.T) {
	now := time.Date(2026, 5, 2, 14, 30, 45, 0, time.UTC)
	got := backup.DefaultPath("/tmp/backups", now)
	want := "/tmp/backups/arsenal-20260502-143045.db"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
