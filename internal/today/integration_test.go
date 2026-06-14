package today_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/edwinupegui/arsenal/internal/finance"
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

// TestService_Build_ShowAllURLOnOverflow seeds 7 overdue todos and asserts
// the overdue section is truncated to 5 with ShowAllURL set. This is the
// regression for the v3.0 follow-up: previously providers capped at 5
// before Service.Build saw the data, so ShowAllURL never triggered.
func TestService_Build_ShowAllURLOnOverflow(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := todos.New(db)
	yesterday := yesterdayStr()
	for i := 0; i < 7; i++ {
		_, err := svc.Create(ctx, todos.CreateInput{
			Title:    "overdue " + string(rune('a'+i)),
			Priority: todos.PriorityHigh,
			DueDate:  &yesterday,
		})
		if err != nil {
			t.Fatalf("seed overdue: %v", err)
		}
	}

	reg := today.NewRegistry()
	reg.Register(providers.NewTodosProvider(db))
	reg.Register(providers.NewResourcesProvider(db))
	todaySvc := today.NewWithRegistry(reg)

	secs, errs := todaySvc.Build(ctx)
	if len(errs) != 0 {
		t.Fatalf("unexpected provider errors: %v", errs)
	}
	if len(secs) == 0 {
		t.Fatal("expected at least one section (overdue)")
	}
	overdue := secs[0]
	if overdue.Key != "overdue" {
		t.Fatalf("first section key = %q, want overdue", overdue.Key)
	}
	if len(overdue.Items) > 5 {
		t.Errorf("overdue section has %d items, want ≤ 5 (density cap)", len(overdue.Items))
	}
	if len(overdue.Items) != 5 {
		t.Errorf("overdue section has %d items, want exactly 5 (truncated from 7)", len(overdue.Items))
	}
	if overdue.ShowAllURL == "" {
		t.Error("ShowAllURL empty on overflow section; expected '/todos?status=open&overdue=true'")
	}
	if overdue.ShowAllURL != "/todos?status=open&overdue=true" {
		t.Errorf("ShowAllURL = %q, want /todos?status=open&overdue=true", overdue.ShowAllURL)
	}
}

// TestService_Build_ShowAllURLOnRecentOverflow seeds 8 resources and asserts
// the recent section is truncated to 5 with ShowAllURL set to /resources.
func TestService_Build_ShowAllURLOnRecentOverflow(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	svc := resources.New(db)
	for i := 0; i < 8; i++ {
		_, err := svc.Create(ctx, resources.CreateInput{
			Title:    "Resource " + string(rune('a'+i)),
			URL:      "https://example.com/" + string(rune('a'+i)),
			Type:     "article",
			Language: "EN",
		})
		if err != nil {
			t.Fatalf("seed resource: %v", err)
		}
	}

	reg := today.NewRegistry()
	reg.Register(providers.NewResourcesProvider(db))
	todaySvc := today.NewWithRegistry(reg)

	secs, errs := todaySvc.Build(ctx)
	if len(errs) != 0 {
		t.Fatalf("unexpected provider errors: %v", errs)
	}
	if len(secs) != 1 {
		t.Fatalf("expected 1 section (recent), got %d", len(secs))
	}
	recent := secs[0]
	if recent.Key != "recent" {
		t.Errorf("section key = %q, want recent", recent.Key)
	}
	if len(recent.Items) != 5 {
		t.Errorf("recent section has %d items, want exactly 5 (truncated from 8)", len(recent.Items))
	}
	if recent.ShowAllURL == "" {
		t.Error("ShowAllURL empty on recent overflow; expected '/resources'")
	}
	if recent.ShowAllURL != "/resources" {
		t.Errorf("ShowAllURL = %q, want /resources", recent.ShowAllURL)
	}
}

func TestService_Build_FinanceSectionsIntegration(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	seedTodos(t, todos.New(db))
	seedResources(t, resources.New(db))
	finSvc := finance.New(db)
	for i := 0; i < 3; i++ {
		_, err := finSvc.Create(ctx, finance.CreateInput{
			Date:    todayStr(),
			Amount:  float64(10 * (i + 1)),
			Kind:    finance.KindExpense,
			Account: "checking",
		})
		if err != nil {
			t.Fatalf("seed finance: %v", err)
		}
	}

	reg := today.NewRegistry()
	reg.Register(providers.NewTodosProvider(db))
	reg.Register(providers.NewResourcesProvider(db))
	reg.Register(providers.NewFinanceProvider(db))
	todaySvc := today.NewWithRegistry(reg)

	secs, errs := todaySvc.Build(ctx)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	want := []string{"overdue", "due-today", "upcoming", "recent", "this-month-spending", "recent-transactions"}
	if len(secs) != len(want) {
		t.Fatalf("expected %d sections, got %d", len(want), len(secs))
	}
	for i, w := range want {
		if secs[i].Key != w {
			t.Errorf("section[%d].Key = %q, want %q", i, secs[i].Key, w)
		}
	}

	// Density cap applies to recent-transactions too.
	for _, s := range secs {
		if len(s.Items) > 5 {
			t.Errorf("section %q has %d items, want ≤ 5", s.Key, len(s.Items))
		}
	}
}
