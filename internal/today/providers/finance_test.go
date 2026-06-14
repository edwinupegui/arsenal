package providers_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
	"github.com/edwinupegui/arsenal/internal/finance"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/today/providers"
)

func newFinanceTestDB(t *testing.T) *sql.DB {
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

func seedFinanceTransactions(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	svc := finance.New(db)
	q := store.New(db)

	catIDs := make(map[string]int64)
	for _, slug := range []string{"food", "transport", "services"} {
		name := slug
		if len(name) > 0 {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
		created, err := q.CreateCategory(ctx, store.CreateCategoryParams{
			Slug:      slug,
			Name:      name,
			SortOrder: 1,
		})
		if err != nil {
			t.Fatalf("create category %s: %v", slug, err)
		}
		catIDs[slug] = created.ID
	}

	categories := []struct {
		amount   float64
		category string
	}{
		{200, "food"},
		{100, "transport"},
		{50, "services"},
		{150, "food"},
	}

	for _, c := range categories {
		cid := catIDs[c.category]
		in := finance.CreateInput{
			Date:       "2026-06-10",
			Amount:     c.amount,
			Kind:       finance.KindExpense,
			Account:    "checking",
			CategoryID: &cid,
		}
		if _, err := svc.Create(ctx, in); err != nil {
			t.Fatalf("create expense: %v", err)
		}
	}
}

func TestFinanceProvider_Name(t *testing.T) {
	db := newFinanceTestDB(t)
	p := providers.NewFinanceProvider(db)
	if got := p.Name(); got != "finance" {
		t.Errorf("Name() = %q, want finance", got)
	}
}

func TestFinanceProvider_ImplementsProvider(t *testing.T) {
	var _ today.Provider = providers.NewFinanceProvider(nil)
}

func TestFinanceProvider_ThisMonthSpendingSection(t *testing.T) {
	db := newFinanceTestDB(t)
	seedFinanceTransactions(t, db)

	p := providers.NewFinanceProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}

	var spending *today.Section
	for i := range secs {
		if secs[i].Key == "this-month-spending" {
			spending = &secs[i]
			break
		}
	}
	if spending == nil {
		t.Fatalf("expected this-month-spending section, got %+v", secs)
	}
	if spending.Title != "This Month's Spending" {
		t.Errorf("title = %q, want %q", spending.Title, "This Month's Spending")
	}
	if len(spending.Items) != 1 {
		t.Fatalf("expected 1 summary item, got %d", len(spending.Items))
	}

	summary := spending.Items[0]
	if summary.Domain != "finance" {
		t.Errorf("domain = %q, want finance", summary.Domain)
	}
	wantTitle := "Total: $500.00"
	if summary.Title != wantTitle {
		t.Errorf("title = %q, want %q", summary.Title, wantTitle)
	}
	if summary.Subtitle == "" {
		t.Error("expected non-empty subtitle with top categories")
	}
	for _, want := range []string{"Food", "Transport", "Services"} {
		if !contains(summary.Subtitle, want) {
			t.Errorf("subtitle %q missing %q", summary.Subtitle, want)
		}
	}
}

func TestFinanceProvider_ThisMonthSpendingOmitsWhenNoExpenses(t *testing.T) {
	db := newFinanceTestDB(t)
	ctx := context.Background()
	svc := finance.New(db)
	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-05-10",
		Amount:  100,
		Kind:    finance.KindExpense,
		Account: "checking",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	p := providers.NewFinanceProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	for _, s := range secs {
		if s.Key == "this-month-spending" {
			t.Fatalf("expected no this-month-spending section, got %+v", s)
		}
	}
}

func TestFinanceProvider_RecentTransactionsSection(t *testing.T) {
	db := newFinanceTestDB(t)
	ctx := context.Background()
	svc := finance.New(db)
	for i := 0; i < 8; i++ {
		if _, err := svc.Create(ctx, finance.CreateInput{
			Date:    fmt.Sprintf("2026-06-%02d", 10+i),
			Amount:  float64(10 + i),
			Kind:    finance.KindExpense,
			Account: fmt.Sprintf("account-%d", i),
			Tags:    []string{"tag"},
		}); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	p := providers.NewFinanceProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}

	var recent *today.Section
	for i := range secs {
		if secs[i].Key == "recent-transactions" {
			recent = &secs[i]
			break
		}
	}
	if recent == nil {
		t.Fatalf("expected recent-transactions section, got %+v", secs)
	}
	if recent.Title != "Recent Transactions" {
		t.Errorf("title = %q, want %q", recent.Title, "Recent Transactions")
	}
	if len(recent.Items) != 8 {
		t.Errorf("items = %d, want 8", len(recent.Items))
	}
	// Should be sorted by date DESC.
	if len(recent.Items) > 1 && recent.Items[0].Title != "account-7 (expense)" {
		t.Errorf("first item = %q, want account-7", recent.Items[0].Title)
	}

	first := recent.Items[0]
	if first.Domain != "finance" {
		t.Errorf("domain = %q, want finance", first.Domain)
	}
	if len(first.Tags) != 1 || first.Tags[0] != "tag" {
		t.Errorf("tags = %v, want [tag]", first.Tags)
	}
	if first.URL == "" {
		t.Error("expected URL")
	}
}

func TestFinanceProvider_RecentTransactionsOmitsWhenEmpty(t *testing.T) {
	db := newFinanceTestDB(t)
	p := providers.NewFinanceProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if len(secs) != 0 {
		t.Errorf("expected 0 sections, got %d", len(secs))
	}
}

func TestFinanceProvider_RespectsTimezone(t *testing.T) {
	db := newFinanceTestDB(t)
	ctx := context.Background()
	cs := configstore.New(db)
	if err := cs.Set(ctx, config.KeyUserTimezone, "America/Argentina/Buenos_Aires"); err != nil {
		t.Fatalf("set timezone: %v", err)
	}

	p := providers.NewFinanceProvider(db, providers.WithFinanceClock(func() time.Time {
		// 2026-07-01 02:00 UTC = 2026-06-30 23:00 local (-03:00)
		return time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)
	}))

	if _, err := finance.New(db, finance.WithClock(func() time.Time {
		return time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)
	})).Create(ctx, finance.CreateInput{
		Date:    "2026-06-30",
		Amount:  100,
		Kind:    finance.KindExpense,
		Account: "checking",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	secs, err := p.Sections(ctx)
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	var spending *today.Section
	for i := range secs {
		if secs[i].Key == "this-month-spending" {
			spending = &secs[i]
			break
		}
	}
	if spending == nil {
		t.Fatalf("expected this-month-spending in June local time, got %+v", secs)
	}
}

func TestFinanceProvider_ItemMapping(t *testing.T) {
	db := newFinanceTestDB(t)
	ctx := context.Background()
	svc := finance.New(db)
	created, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-13",
		Amount:  42.50,
		Kind:    finance.KindExpense,
		Account: "checking",
		Tags:    []string{"food"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	p := providers.NewFinanceProvider(db)
	secs, err := p.Sections(ctx)
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	var recent *today.Section
	for i := range secs {
		if secs[i].Key == "recent-transactions" {
			recent = &secs[i]
			break
		}
	}
	if recent == nil || len(recent.Items) == 0 {
		t.Fatalf("expected recent-transactions section with items, got %+v", secs)
	}
	it := recent.Items[0]
	if it.Domain != "finance" {
		t.Errorf("domain = %q, want finance", it.Domain)
	}
	wantTitle := "checking (expense)"
	if it.Title != wantTitle {
		t.Errorf("title = %q, want %q", it.Title, wantTitle)
	}
	if it.Subtitle != "$42.50" {
		t.Errorf("subtitle = %q, want $42.50", it.Subtitle)
	}
	if len(it.Tags) != 1 || it.Tags[0] != "food" {
		t.Errorf("tags = %v, want [food]", it.Tags)
	}
	wantURL := fmt.Sprintf("/finance/%d", created.Row.ID)
	if it.URL != wantURL {
		t.Errorf("url = %q, want %s", it.URL, wantURL)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
