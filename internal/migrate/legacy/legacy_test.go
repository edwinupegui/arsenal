package legacy_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/edwinupegui/arsenal/internal/migrate/legacy"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
)

// buildLegacyDB writes a SQLite file at path with the v1 schema and seeds
// it with two categories and three resources covering the edge cases the
// real legacy DB exercised: tags as JSON, a tag that duplicates the
// category name (must be dropped on import), and a soft-deleted row.
func buildLegacyDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE categories (
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			icon TEXT NOT NULL
		)`,
		`CREATE TABLE resources (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			title       TEXT NOT NULL,
			url         TEXT NOT NULL,
			description TEXT,
			tags        TEXT,
			language    TEXT NOT NULL,
			type        TEXT NOT NULL,
			category_id INTEGER NOT NULL REFERENCES categories(id),
			created_at  TEXT NOT NULL,
			deleted_at  TEXT
		)`,
		`INSERT INTO categories (id, name, icon) VALUES
			(1, 'Architecture, Clean Code', 'architect'),
			(2, 'AI Agents', 'robot')`,
		`INSERT INTO resources (id, title, url, description, tags, language, type, category_id, created_at, deleted_at) VALUES
			(1, 'Hex Arch', 'https://example.com/hex', 'desc 1', '["patterns", "architecture, clean code"]', 'EN', 'article', 1, '2025-01-01T10:00:00.000Z', NULL),
			(2, 'Agent Tutorial', 'https://example.com/agent', NULL, '["agents", "llm"]', 'ES', 'video', 2, '2025-01-02T10:00:00.000Z', NULL),
			(3, 'Trashed Item', 'https://example.com/old', NULL, '[]', 'EN', 'tool', 1, '2025-01-03T10:00:00.000Z', '2025-02-01T00:00:00.000Z')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed legacy: %v\nSQL: %s", err, s)
		}
	}
}

// newDestDB returns a fresh v2 DB ready to receive imports.
func newDestDB(t *testing.T) (*sql.DB, *store.Queries, *resources.Service) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dest.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open dest: %v", err)
	}
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate dest: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, store.New(db), resources.New(db)
}

func TestImport_HappyPath(t *testing.T) {
	ctx := context.Background()
	srcPath := filepath.Join(t.TempDir(), "legacy.db")
	buildLegacyDB(t, srcPath)

	db, q, svc := newDestDB(t)

	rep, err := legacy.Import(ctx, legacy.Options{
		SourcePath: srcPath, Service: svc, Queries: q,
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if rep.CategoriesInserted != 2 {
		t.Errorf("categories inserted = %d, want 2", rep.CategoriesInserted)
	}
	if rep.ResourcesImported != 3 {
		t.Errorf("resources imported = %d, want 3", rep.ResourcesImported)
	}
	// "architecture, clean code" tag matches the category name normalized
	// → must be dropped. After dedupe, distinct tags are: patterns, agents, llm.
	if rep.TagsDroppedAsCategoryOf != 1 {
		t.Errorf("tags dropped = %d, want 1", rep.TagsDroppedAsCategoryOf)
	}

	// Verify destination rows.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&n); err != nil {
		t.Fatalf("count resources: %v", err)
	}
	if n != 3 {
		t.Errorf("dest resources = %d, want 3", n)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM resources WHERE deleted_at IS NOT NULL`).Scan(&n); err != nil {
		t.Fatalf("count trashed: %v", err)
	}
	if n != 1 {
		t.Errorf("trashed = %d, want 1", n)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&n); err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if n != 3 { // patterns, agents, llm (architecture-as-tag dropped)
		t.Errorf("tags = %d, want 3", n)
	}

	// Verify slug derivation produced sane keys.
	var slug string
	if err := db.QueryRow(`SELECT slug FROM categories WHERE id = 1`).Scan(&slug); err != nil {
		t.Fatalf("get cat slug: %v", err)
	}
	if slug != "architecture-clean-code" {
		t.Errorf("cat slug = %q, want architecture-clean-code", slug)
	}
}

func TestImport_RejectsMissingSchema(t *testing.T) {
	ctx := context.Background()

	// Create a "DB" that's just an empty SQLite file without our tables.
	srcPath := filepath.Join(t.TempDir(), "empty.db")
	db, err := sql.Open("sqlite", srcPath)
	if err != nil {
		t.Fatalf("open empty: %v", err)
	}
	_ = db.Close()

	_, q, svc := newDestDB(t)

	_, err = legacy.Import(ctx, legacy.Options{
		SourcePath: srcPath, Service: svc, Queries: q,
	})
	if err == nil {
		t.Fatal("expected schema validation error")
	}
}

func TestImport_PreservesTimestampsAndDeletedAt(t *testing.T) {
	ctx := context.Background()
	srcPath := filepath.Join(t.TempDir(), "legacy.db")
	buildLegacyDB(t, srcPath)

	db, q, svc := newDestDB(t)

	if _, err := legacy.Import(ctx, legacy.Options{
		SourcePath: srcPath, Service: svc, Queries: q,
	}); err != nil {
		t.Fatalf("Import: %v", err)
	}

	var (
		created string
		deleted sql.NullString
	)
	if err := db.QueryRow(`SELECT created_at, deleted_at FROM resources WHERE url = ?`,
		"https://example.com/old").Scan(&created, &deleted); err != nil {
		t.Fatalf("query: %v", err)
	}
	if created != "2025-01-03T10:00:00.000Z" {
		t.Errorf("created_at = %q", created)
	}
	if !deleted.Valid || deleted.String != "2025-02-01T00:00:00.000Z" {
		t.Errorf("deleted_at = %+v", deleted)
	}
}
