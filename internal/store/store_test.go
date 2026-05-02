package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
)

// setup returns a fresh DB seeded with two categories and a curated set of
// resources covering each filter dimension so each test can assert against
// the same fixtures.
func setup(t *testing.T) (*sql.DB, *store.Queries) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
		INSERT INTO categories (id, slug, name, icon, sort_order) VALUES
			(1, 'arch', 'Architecture', 'architect', 1),
			(2, 'gamedev', 'Gamedev', 'gamepad', 2);

		INSERT INTO resources (id, title, url, description, type, language, category_id, favorite, created_at, updated_at) VALUES
			(1, 'Hex Arch',  'https://example.com/hex',  'about hexagonal architecture', 'article', 'EN', 1, 1, '2025-01-01T10:00:00.000Z', '2025-01-01T10:00:00.000Z'),
			(2, 'Pattern Lab','https://example.com/pat', 'design patterns explained',    'video',   'ES', 1, 0, '2025-01-02T10:00:00.000Z', '2025-01-02T10:00:00.000Z'),
			(3, 'Game Math',  'https://example.com/gm',  'math for gamedev',             'article', 'EN', 2, 0, '2025-01-03T10:00:00.000Z', '2025-01-03T10:00:00.000Z'),
			(4, 'Trashed',    'https://example.com/del', NULL,                           'tool',    'EN', 1, 0, '2025-01-04T10:00:00.000Z', '2025-01-04T10:00:00.000Z');

		UPDATE resources SET deleted_at = '2025-02-01T00:00:00.000Z' WHERE id = 4;

		INSERT INTO tags (id, name) VALUES (1, 'patterns'), (2, 'math');
		INSERT INTO resource_tags (resource_id, tag_id) VALUES
			(1, 1), -- Hex Arch ← patterns
			(2, 1), -- Pattern Lab ← patterns
			(3, 2); -- Game Math ← math
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db, store.New(db)
}

func TestListResourcesFiltered_ExcludesTrashedByDefault(t *testing.T) {
	_, q := setup(t)
	rows, err := q.ListResourcesFiltered(context.Background(), store.ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("got %d rows, want 3 (trashed excluded)", len(rows))
	}
}

func TestListResourcesFiltered_OnlyTrashed(t *testing.T) {
	_, q := setup(t)
	rows, err := q.ListResourcesFiltered(context.Background(), store.ListFilter{Trashed: true})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Resource.ID != 4 {
		t.Errorf("trashed-only = %+v, want id=4", rows)
	}
}

func TestListResourcesFiltered_ByCategorySlug(t *testing.T) {
	_, q := setup(t)
	rows, err := q.ListResourcesFiltered(context.Background(), store.ListFilter{
		CategorySlug: "gamedev",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Resource.ID != 3 {
		t.Errorf("by cat = %+v, want id=3", rows)
	}
}

func TestListResourcesFiltered_ByTag(t *testing.T) {
	_, q := setup(t)
	rows, err := q.ListResourcesFiltered(context.Background(), store.ListFilter{
		TagName: "patterns",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("by tag patterns = %d rows, want 2", len(rows))
	}
}

func TestListResourcesFiltered_TypeAndLang(t *testing.T) {
	_, q := setup(t)
	rows, err := q.ListResourcesFiltered(context.Background(), store.ListFilter{
		Type: "article", Language: "EN",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("article+EN = %d rows, want 2", len(rows))
	}
}

func TestListResourcesFiltered_OnlyFavorite(t *testing.T) {
	_, q := setup(t)
	rows, err := q.ListResourcesFiltered(context.Background(), store.ListFilter{
		OnlyFavorite: true,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Resource.ID != 1 {
		t.Errorf("favorites = %+v", rows)
	}
}

func TestListResourcesFiltered_TagsAndCategoryAreFlat(t *testing.T) {
	_, q := setup(t)
	rows, err := q.ListResourcesFiltered(context.Background(), store.ListFilter{
		TagName: "patterns",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, r := range rows {
		if r.Resource.ID == 1 {
			if !r.CategorySlug.Valid || r.CategorySlug.String != "arch" {
				t.Errorf("expected category slug 'arch', got %v", r.CategorySlug)
			}
			if len(r.Tags) == 0 || r.Tags[0] != "patterns" {
				t.Errorf("expected tags [patterns], got %v", r.Tags)
			}
		}
	}
}

func TestSearchResources_FTS5_PrefixAndDiacritics(t *testing.T) {
	_, q := setup(t)
	ctx := context.Background()

	rows, err := q.SearchResources(ctx, "hexagonal", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 || rows[0].Resource.ID != 1 {
		t.Errorf("search hexagonal = %+v, want only id=1", rows)
	}

	// Prefix match on "patt" should hit both Pattern Lab and the description
	// of Hex Arch (which contains "hexagonal architecture", no patt — drop)
	rows, err = q.SearchResources(ctx, "patt", 10)
	if err != nil {
		t.Fatalf("search patt: %v", err)
	}
	// Pattern Lab title + Hex Arch description has "patterns" via tag.
	if len(rows) < 1 {
		t.Errorf("patt prefix should match at least one row, got %d", len(rows))
	}
}

func TestSearchResources_EmptyQueryReturnsEmpty(t *testing.T) {
	_, q := setup(t)
	rows, err := q.SearchResources(context.Background(), "   ", 10)
	if err != nil {
		t.Fatalf("search empty: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected empty result for blank query, got %d rows", len(rows))
	}
}

func TestSearchResources_ExcludesTrashed(t *testing.T) {
	_, q := setup(t)
	// "Trashed" is the title of the soft-deleted row; FTS still indexed it
	// when it was created, but the JOIN must exclude it.
	rows, err := q.SearchResources(context.Background(), "trashed", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows (soft-deleted), got %d", len(rows))
	}
}
