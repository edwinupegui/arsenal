package resources_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/resources"
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

func validCreate() resources.CreateInput {
	return resources.CreateInput{
		Title:    "Hexagonal Architecture",
		URL:      "https://example.com/hex",
		Type:     domain.TypeArticle,
		Language: domain.LangEN,
		Tags:     []string{"architecture", "Clean code", "  Architecture  "}, // dups + whitespace
	}
}

func TestCreate_HappyPath(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	got, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if got.Row.Title != "Hexagonal Architecture" {
		t.Errorf("title = %q", got.Row.Title)
	}
	// "architecture" is a duplicate of "  Architecture  " after normalize, so
	// we expect 2 tags total, sorted.
	wantTags := []string{"architecture", "clean code"}
	if !equalStrings(got.Tags, wantTags) {
		t.Errorf("tags = %v, want %v", got.Tags, wantTags)
	}

	// Round-trip via Get.
	round, err := svc.Get(ctx, got.Row.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !equalStrings(round.Tags, wantTags) {
		t.Errorf("Get tags = %v, want %v", round.Tags, wantTags)
	}
}

func TestCreate_Validation(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	cases := []struct {
		name   string
		mutate func(*resources.CreateInput)
	}{
		{"empty title", func(in *resources.CreateInput) { in.Title = "   " }},
		{"empty url", func(in *resources.CreateInput) { in.URL = "" }},
		{"non-http scheme", func(in *resources.CreateInput) { in.URL = "ftp://x.com" }},
		{"bad type", func(in *resources.CreateInput) { in.Type = "podcasts" }}, // typo
		{"bad lang", func(in *resources.CreateInput) { in.Language = "JA" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validCreate()
			tc.mutate(&in)
			if _, err := svc.Create(ctx, in); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestCreate_DuplicateURL(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	if _, err := svc.Create(ctx, validCreate()); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := svc.Create(ctx, validCreate())
	if err == nil {
		t.Fatal("expected duplicate URL error, got nil")
	}
}

func TestCreate_RollbackOnTagFailure(t *testing.T) {
	// Force a tag-attach failure by feeding a tag that survives normalize but
	// the service should reject earlier — actually NormalizeTag rejects
	// over-length tags, so we use that to validate the Create path errors out
	// before any insert lands.
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	in := validCreate()
	in.Tags = []string{string(make([]byte, domain.MaxTagLength+5))}
	if _, err := svc.Create(ctx, in); err == nil {
		t.Fatal("expected validation error")
	}

	// Verify nothing landed.
	q := store.New(db)
	count, err := q.CountResources(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 resources, got %d", count)
	}
}

func TestUpdate_ReplacesTagsAndPrunesOrphans(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Replace tag set entirely.
	_, err = svc.Update(ctx, created.Row.ID, resources.UpdateInput{
		Title: "Hex Arch v2", URL: "https://example.com/hex",
		Type: domain.TypeArticle, Language: domain.LangEN,
		Tags: []string{"ddd", "patterns"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	round, err := svc.Get(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !equalStrings(round.Tags, []string{"ddd", "patterns"}) {
		t.Errorf("after Update tags = %v", round.Tags)
	}

	// Original "architecture" / "clean code" tags should now be orphaned and
	// therefore deleted.
	q := store.New(db)
	tags, err := q.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	names := make(map[string]bool, len(tags))
	for _, t := range tags {
		names[t.Name] = true
	}
	for _, want := range []string{"ddd", "patterns"} {
		if !names[want] {
			t.Errorf("missing tag %q", want)
		}
	}
	for _, gone := range []string{"architecture", "clean code"} {
		if names[gone] {
			t.Errorf("orphan tag %q should have been pruned", gone)
		}
	}
}

func TestSoftDelete_RestoreCycle(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.Row.ID

	if err := svc.SoftDelete(ctx, id); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	q := store.New(db)
	row, err := q.GetResource(ctx, id)
	if err != nil {
		t.Fatalf("GetResource after delete: %v", err)
	}
	if !row.DeletedAt.Valid {
		t.Error("expected deleted_at to be set")
	}

	if err := svc.Restore(ctx, id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	row, err = q.GetResource(ctx, id)
	if err != nil {
		t.Fatalf("GetResource after restore: %v", err)
	}
	if row.DeletedAt.Valid {
		t.Error("expected deleted_at to be NULL after restore")
	}
}

func TestPurge_RemovesRowAndOrphanTags(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Purge(ctx, created.Row.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	q := store.New(db)
	if _, err := q.GetResource(ctx, created.Row.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected ErrNoRows, got %v", err)
	}
	tags, err := q.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected 0 tags after purge, got %d", len(tags))
	}
}

func TestSetFavorite(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.SetFavorite(ctx, created.Row.ID, true); err != nil {
		t.Fatalf("SetFavorite: %v", err)
	}
	got, err := svc.Get(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Row.Favorite != 1 {
		t.Errorf("favorite = %d, want 1", got.Row.Favorite)
	}
}

func TestImport_PreservesTimestamps(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	const ts = "2025-01-15T10:30:00.000Z"
	in := resources.ImportInput{
		CreateInput: validCreate(),
		CreatedAt:   ts,
		UpdatedAt:   ts,
	}
	got, err := svc.Import(ctx, in)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if got.Row.CreatedAt != ts {
		t.Errorf("created_at = %q, want %q", got.Row.CreatedAt, ts)
	}
	if got.Row.UpdatedAt != ts {
		t.Errorf("updated_at = %q, want %q", got.Row.UpdatedAt, ts)
	}
}

func TestImport_PreservesSoftDeletedState(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := resources.New(db)

	const ts = "2025-01-15T10:30:00.000Z"
	const del = "2025-02-20T08:00:00.000Z"
	delPtr := del
	in := resources.ImportInput{
		CreateInput: validCreate(),
		CreatedAt:   ts,
		UpdatedAt:   ts,
		DeletedAt:   &delPtr,
	}
	got, err := svc.Import(ctx, in)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if !got.Row.DeletedAt.Valid || got.Row.DeletedAt.String != del {
		t.Errorf("deleted_at = %+v, want %q", got.Row.DeletedAt, del)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
