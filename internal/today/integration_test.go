package today_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/today/providers"
	"github.com/edwinupegui/arsenal/internal/todos"
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

func todayStr() string   { return time.Now().UTC().Format("2006-01-02") }
func yesterdayStr() string { return time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02") }
func tomorrowStr() string { return time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02") }
func weekLaterStr() string {
	return time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
}

func seedTodos(t *testing.T, svc *todos.Service) {
	t.Helper()
	ctx := context.Background()
	yesterday := yesterdayStr()
	for i := 0; i < 3; i++ {
		_, err := svc.Create(ctx, todos.CreateInput{Title: "overdue", Priority: todos.PriorityHigh, DueDate: &yesterday})
		if err != nil {
			t.Fatalf("seed overdue: %v", err)
		}
	}
	today := todayStr()
	for i := 0; i < 2; i++ {
		_, err := svc.Create(ctx, todos.CreateInput{Title: "due today", Priority: todos.PriorityHigh, DueDate: &today})
		if err != nil {
			t.Fatalf("seed due today: %v", err)
		}
	}
	tomorrow := tomorrowStr()
	for i := 0; i < 4; i++ {
		_, err := svc.Create(ctx, todos.CreateInput{Title: "upcoming", Priority: todos.PriorityHigh, DueDate: &tomorrow})
		if err != nil {
			t.Fatalf("seed upcoming: %v", err)
		}
	}
}

func seedResources(t *testing.T, svc *resources.Service) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 8; i++ {
		_, err := svc.Create(ctx, resources.CreateInput{
			Title:    "Resource",
			URL:      "https://example.com/" + string(rune('a'+i)),
			Type:     "article",
			Language: "EN",
			Tags:     []string{},
		})
		if err != nil {
			t.Fatalf("seed resource: %v", err)
		}
	}
}

func TestService_Build_Integration(t *testing.T) {
	db := newTestDB(t)
	seedTodos(t, todos.New(db))
	seedResources(t, resources.New(db))

	reg := today.NewRegistry()
	reg.Register(providers.NewTodosProvider(db))
	reg.Register(providers.NewResourcesProvider(db))
	svc := today.NewWithRegistry(reg)

	secs, errs := svc.Build(context.Background())
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	want := []string{"overdue", "due-today", "upcoming", "recent"}
	if len(secs) != len(want) {
		t.Fatalf("expected %d sections, got %d", len(want), len(secs))
	}
	for i, w := range want {
		if secs[i].Key != w {
			t.Errorf("section[%d].Key = %q, want %q", i, secs[i].Key, w)
		}
	}

	// Density cap
	for _, s := range secs {
		if len(s.Items) > 5 {
			t.Errorf("section %q has %d items, want ≤ 5", s.Key, len(s.Items))
		}
	}

	// ShowAllURL on overflow sections
	if secs[0].ShowAllURL == "" { // overdue has 3 items, no overflow
		// no overflow expected
	}
	if secs[2].ShowAllURL == "" { // upcoming has 4 items, no overflow
		// no overflow expected
	}
}
