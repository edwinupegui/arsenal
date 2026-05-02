package exportmd_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/exportmd"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
)

// newDB returns a freshly migrated DB for testing.
func newDB(t *testing.T) (string, *store.Queries, *resources.Service, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return path, store.New(db), resources.New(db), func() { _ = db.Close() }
}

func TestRoundTrip_ResourcesAndCategories(t *testing.T) {
	ctx := context.Background()

	// --- source DB ---
	_, srcQ, srcSvc, srcClose := newDB(t)
	defer srcClose()

	// Seed a category and a couple of resources with various features.
	cat, err := srcQ.CreateCategory(ctx, store.CreateCategoryParams{
		Slug: "demo", Name: "Demo Category", Icon: "star", SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("seed cat: %v", err)
	}
	catID := cat.ID

	if _, err := srcSvc.Create(ctx, resources.CreateInput{
		Title: "First", URL: "https://example.com/one",
		Type: domain.TypeArticle, Language: domain.LangEN,
		CategoryID: &catID,
		Tags:       []string{"alpha", "beta"},
		Favorite:   true,
	}); err != nil {
		t.Fatalf("seed res 1: %v", err)
	}
	if _, err := srcSvc.Create(ctx, resources.CreateInput{
		Title: "Second", URL: "https://example.com/two",
		Type: domain.TypeRepo, Language: domain.LangES,
		Tags: []string{"gamma"},
	}); err != nil {
		t.Fatalf("seed res 2: %v", err)
	}

	// --- export ---
	dir := filepath.Join(t.TempDir(), "export")
	report, err := exportmd.ExportAll(ctx, nil, srcQ, exportmd.Options{Dir: dir})
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if report.Resources != 2 {
		t.Errorf("exported %d resources, want 2", report.Resources)
	}

	// Sanity: _categories.json exists.
	if _, err := os.Stat(filepath.Join(dir, exportmd.CategoriesIndexFilename)); err != nil {
		t.Errorf("expected categories index file, got %v", err)
	}

	// --- destination DB (fresh) ---
	_, dstQ, dstSvc, dstClose := newDB(t)
	defer dstClose()

	imp, err := exportmd.Import(ctx, dir, dstSvc, dstQ)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if imp.Imported != 2 {
		t.Errorf("imported %d, want 2", imp.Imported)
	}
	if imp.Failed != 0 {
		t.Errorf("failed %d, want 0; warnings=%v", imp.Failed, imp.Warnings)
	}

	// Verify category was recreated.
	if _, err := dstQ.GetCategoryBySlug(ctx, "demo"); err != nil {
		t.Errorf("category not recreated: %v", err)
	}

	// Verify resource details survive the round-trip.
	rows, err := dstQ.ListResourcesFiltered(ctx, store.ListFilter{Limit: 100})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	byTitle := map[string]store.ListedResource{}
	for _, r := range rows {
		byTitle[r.Resource.Title] = r
	}

	first, ok := byTitle["First"]
	if !ok {
		t.Fatal("missing 'First'")
	}
	if first.Resource.Favorite != 1 {
		t.Error("'First' lost favorite flag")
	}
	if !first.CategorySlug.Valid || first.CategorySlug.String != "demo" {
		t.Errorf("'First' category = %v, want demo", first.CategorySlug)
	}
	if !containsAll(first.Tags, []string{"alpha", "beta"}) {
		t.Errorf("'First' tags = %v, want alpha+beta", first.Tags)
	}

	second, ok := byTitle["Second"]
	if !ok {
		t.Fatal("missing 'Second'")
	}
	if second.CategorySlug.Valid {
		t.Errorf("'Second' should be uncategorized, got %s", second.CategorySlug.String)
	}
}

func TestImport_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	_, srcQ, srcSvc, srcClose := newDB(t)
	defer srcClose()

	if _, err := srcSvc.Create(ctx, resources.CreateInput{
		Title: "Dup", URL: "https://example.com/dup",
		Type: domain.TypeArticle, Language: domain.LangEN,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "export")
	if _, err := exportmd.ExportAll(ctx, nil, srcQ, exportmd.Options{Dir: dir}); err != nil {
		t.Fatalf("export: %v", err)
	}

	_, dstQ, dstSvc, dstClose := newDB(t)
	defer dstClose()

	first, err := exportmd.Import(ctx, dir, dstSvc, dstQ)
	if err != nil || first.Imported != 1 {
		t.Fatalf("first import: %+v err=%v", first, err)
	}
	second, err := exportmd.Import(ctx, dir, dstSvc, dstQ)
	if err != nil {
		t.Fatalf("second import err: %v", err)
	}
	if second.Imported != 0 || second.SkippedDup != 1 {
		t.Errorf("second import = %+v, expected 0 imported / 1 skipped", second)
	}
}

func TestParseFile_HandlesQuotedValuesAndEscapes(t *testing.T) {
	dir := t.TempDir()
	body := `---
id: 7
title: "He said \"hi\" today"
url: "https://example.com/x"
type: article
language: EN
tags: ["one", "two with space"]
favorite: true
created_at: 2025-01-01T00:00:00.000Z
updated_at: 2025-01-02T00:00:00.000Z
---

Body line 1.
Body line 2.

## Notes

Notes paragraph.
`
	path := filepath.Join(dir, "1-x.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	pf, err := exportmd.ParseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pf.Title != `He said "hi" today` {
		t.Errorf("title = %q", pf.Title)
	}
	if !pf.Favorite {
		t.Error("favorite should be true")
	}
	if !containsAll(pf.Tags, []string{"one", "two with space"}) {
		t.Errorf("tags = %v", pf.Tags)
	}
	if pf.Description == "" || pf.Notes == "" {
		t.Errorf("body split failed: desc=%q notes=%q", pf.Description, pf.Notes)
	}
}

// containsAll returns true when haystack contains every element of needles
// (order-insensitive). Local helper to keep tests self-contained.
func containsAll(haystack, needles []string) bool {
	have := make(map[string]bool, len(haystack))
	for _, h := range haystack {
		have[h] = true
	}
	for _, n := range needles {
		if !have[n] {
			return false
		}
	}
	return true
}
