package calendar_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
)

// newTestDB returns a fresh SQLite DB at a per-test path, fully migrated.
// Using a file (not :memory:) keeps it close to production behavior — WAL
// mode is a no-op on memory DBs, and FTS5 triggers behave the same.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "arsenal.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
