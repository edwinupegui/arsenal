package providers_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/today/providers"
)

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

func seedResources(t *testing.T, svc *resources.Service, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		_, err := svc.Create(ctx, resources.CreateInput{
			Title:    "Resource " + string(rune('A'+i)),
			URL:      "https://example.com/" + string(rune('a'+i)),
			Type:     "article",
			Language: "EN",
			Tags:     []string{},
		})
		if err != nil {
			t.Fatalf("create resource: %v", err)
		}
	}
}

func TestResourcesProvider_RecentSection(t *testing.T) {
	db := newTestDB(t)
	resSvc := resources.New(db)
	seedResources(t, resSvc, 8)

	p := providers.NewResourcesProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if secs[0].Key != "recent" {
		t.Errorf("key = %q, want recent", secs[0].Key)
	}
	// Provider returns all matching rows (up to its practical limit);
	// density truncation to 5 + ShowAllURL setting is the Service's job.
	// See internal/today/integration_test.go::TestService_Build_ShowAllURLOnRecentOverflow
	// for the end-to-end cap behavior.
	if len(secs[0].Items) != 8 {
		t.Errorf("items = %d, want 8 (provider should not cap; Service caps)", len(secs[0].Items))
	}
}

func TestResourcesProvider_OmitsSectionWhenNoResources(t *testing.T) {
	db := newTestDB(t)

	p := providers.NewResourcesProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if len(secs) != 0 {
		t.Errorf("expected 0 sections, got %d", len(secs))
	}
}

func TestResourcesProvider_Limit5(t *testing.T) {
	db := newTestDB(t)
	resSvc := resources.New(db)
	seedResources(t, resSvc, 3)

	p := providers.NewResourcesProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if len(secs[0].Items) != 3 {
		t.Errorf("items = %d, want 3", len(secs[0].Items))
	}
}

func TestResourcesProvider_ItemMapping(t *testing.T) {
	db := newTestDB(t)
	resSvc := resources.New(db)
	created, err := resSvc.Create(context.Background(), resources.CreateInput{
		Title:    "Go Concurrency",
		URL:      "https://example.com/go",
		Type:     "article",
		Language: "EN",
		Tags:     []string{"golang", "concurrency"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	p := providers.NewResourcesProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if len(secs) == 0 || len(secs[0].Items) == 0 {
		t.Fatal("expected at least one item")
	}
	it := secs[0].Items[0]
	if it.Domain != "resources" {
		t.Errorf("domain = %q, want resources", it.Domain)
	}
	if it.Title != "Go Concurrency" {
		t.Errorf("title = %q, want Go Concurrency", it.Title)
	}
	if it.Subtitle != "article" {
		t.Errorf("subtitle = %q, want article", it.Subtitle)
	}
	if it.Priority != "" {
		t.Errorf("priority = %q, want empty", it.Priority)
	}
	if len(it.Tags) != 2 {
		t.Errorf("tags = %v, want 2", it.Tags)
	}
	wantURL := fmt.Sprintf("/resources/%d", created.Row.ID)
	if it.URL != wantURL {
		t.Errorf("url = %q, want %s", it.URL, wantURL)
	}
}

func TestResourcesProvider_ImplementsProvider(t *testing.T) {
	var _ today.Provider = providers.NewResourcesProvider(nil)
}
