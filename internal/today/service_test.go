package today

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
)

// mockProvider is a test double that returns configurable sections/errors.
type mockProvider struct {
	name     string
	sections []Section
	err      error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Sections(ctx context.Context) ([]Section, error) {
	return m.sections, m.err
}

func makeItems(n int) []Item {
	items := make([]Item, n)
	for i := range items {
		items[i] = Item{ID: int64(i + 1), Title: "item"}
	}
	return items
}

func TestRegistry_CollectsFromTwoProviders(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "a", sections: []Section{{Key: "overdue", Title: "Overdue", Items: makeItems(1)}}})
	r.Register(&mockProvider{name: "b", sections: []Section{{Key: "recent", Title: "Recent", Items: makeItems(1)}}})

	secs, errs := r.Collect(context.Background())
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(secs) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(secs))
	}
}

func TestService_SectionOrderingFixed(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "mixed", sections: []Section{
		{Key: "upcoming", Title: "Upcoming", Items: makeItems(1)},
		{Key: "overdue", Title: "Overdue", Items: makeItems(1)},
		{Key: "due-today", Title: "Due Today", Items: makeItems(1)},
		{Key: "recent", Title: "Recent", Items: makeItems(1)},
	}})

	svc := NewWithRegistry(reg)
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
}

func TestService_DensityTruncatesAt5(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "dense", sections: []Section{
		{Key: "overdue", Title: "Overdue", Items: makeItems(8)},
	}})

	svc := NewWithRegistry(reg)
	secs, _ := svc.Build(context.Background())
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if len(secs[0].Items) != 5 {
		t.Errorf("expected 5 items, got %d", len(secs[0].Items))
	}
	if secs[0].ShowAllURL == "" {
		t.Error("expected ShowAllURL set on overflow")
	}
}

func TestService_NoTruncationBelowDensity(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "sparse", sections: []Section{
		{Key: "due-today", Title: "Due Today", Items: makeItems(3)},
	}})

	svc := NewWithRegistry(reg)
	secs, _ := svc.Build(context.Background())
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if len(secs[0].Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(secs[0].Items))
	}
	if secs[0].ShowAllURL != "" {
		t.Error("expected no ShowAllURL when below density")
	}
}

func TestService_EmptySectionsOmitted(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "empty", sections: []Section{
		{Key: "overdue", Title: "Overdue", Items: []Item{}, IsEmpty: true},
		{Key: "upcoming", Title: "Upcoming", Items: makeItems(2)},
	}})

	svc := NewWithRegistry(reg)
	secs, _ := svc.Build(context.Background())
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if secs[0].Key != "upcoming" {
		t.Errorf("expected upcoming, got %q", secs[0].Key)
	}
}

func TestService_ProviderErrorDegradesGracefully(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "broken", err: errors.New("boom"), sections: []Section{
		{Key: "overdue", Title: "Overdue", Items: makeItems(2)},
	}})
	reg.Register(&mockProvider{name: "ok", sections: []Section{
		{Key: "recent", Title: "Recent", Items: makeItems(2)},
	}})

	svc := NewWithRegistry(reg)
	secs, perrs := svc.Build(context.Background())
	if len(perrs) != 1 {
		t.Fatalf("expected 1 provider error, got %d", len(perrs))
	}
	if perrs[0].Name != "broken" {
		t.Errorf("provider error name = %q, want broken", perrs[0].Name)
	}
	if len(secs) != 1 || secs[0].Key != "recent" {
		t.Fatalf("expected recent section only, got %+v", secs)
	}
}

func TestService_ShowAllURLSetOnOverflow(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "overflow", sections: []Section{
		{Key: "overdue", Title: "Overdue", Items: makeItems(6)},
	}})

	svc := NewWithRegistry(reg)
	secs, _ := svc.Build(context.Background())
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if secs[0].ShowAllURL == "" {
		t.Error("expected ShowAllURL set when items > 5")
	}
}

func TestService_ShowAllURLMapping(t *testing.T) {
	// Static-URL cases (no date dependency).
	cases := []struct {
		key  string
		want string
	}{
		{"overdue", "/todos?status=open&overdue=true"},
		{"due-today", "/todos?status=open&due=today"},
		{"upcoming", "/todos?status=open&due=upcoming"},
		{"recent", "/resources"},
		{"this-month-spending", "/finance?kind=expense"},
		{"recent-transactions", "/finance"},
		{"unknown", ""},
	}
	for _, tc := range cases {
		reg := NewRegistry()
		reg.Register(&mockProvider{name: "mapper", sections: []Section{
			{Key: tc.key, Title: tc.key, Items: makeItems(6)},
		}})
		svc := NewWithRegistry(reg)
		secs, _ := svc.Build(context.Background())
		if len(secs) != 1 {
			t.Fatalf("%s: expected 1 section, got %d", tc.key, len(secs))
		}
		if secs[0].ShowAllURL != tc.want {
			t.Errorf("%s: ShowAllURL = %q, want %q", tc.key, secs[0].ShowAllURL, tc.want)
		}
	}
}

// TestShowAllURLFor_CalendarEventsToday asserts REQ-TV-09: the events-today
// show-all URL uses the from/to date-filter format so the link actually
// filters the calendar list to today's events.
func TestShowAllURLFor_CalendarEventsToday(t *testing.T) {
	// Pin clock: 2026-06-15 in UTC.
	pinned := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	got := showAllURLFor("events-today", pinned)
	want := "/calendar?from=2026-06-15&to=2026-06-15"
	if got != want {
		t.Errorf("showAllURLFor(events-today) = %q, want %q", got, want)
	}
}

// TestShowAllURLFor_CalendarEventsUpcoming asserts REQ-TV-09: the
// events-upcoming show-all URL spans tomorrow through today+7.
func TestShowAllURLFor_CalendarEventsUpcoming(t *testing.T) {
	// Pin clock: 2026-06-15 in UTC.
	pinned := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	got := showAllURLFor("events-upcoming", pinned)
	// tomorrow = 2026-06-16, today+7 = 2026-06-22
	want := "/calendar?from=2026-06-16&to=2026-06-22"
	if got != want {
		t.Errorf("showAllURLFor(events-upcoming) = %q, want %q", got, want)
	}
}

func TestService_FinanceSectionOrdering(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "finance", sections: []Section{
		{Key: "this-month-spending", Title: "This Month's Spending", Items: makeItems(1)},
		{Key: "recent-transactions", Title: "Recent Transactions", Items: makeItems(1)},
	}})
	reg.Register(&mockProvider{name: "resources", sections: []Section{
		{Key: "recent", Title: "Recent Resources", Items: makeItems(1)},
	}})
	reg.Register(&mockProvider{name: "todos", sections: []Section{
		{Key: "overdue", Title: "Overdue", Items: makeItems(1)},
		{Key: "due-today", Title: "Due Today", Items: makeItems(1)},
		{Key: "upcoming", Title: "Upcoming", Items: makeItems(1)},
	}})

	svc := NewWithRegistry(reg)
	secs, errs := svc.Build(context.Background())
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
}

func TestRegistry_ProviderErrorSkipped(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "broken", err: errors.New("boom")})
	r.Register(&mockProvider{name: "ok", sections: []Section{{Key: "recent", Title: "Recent", Items: makeItems(1)}}})

	secs, errs := r.Collect(context.Background())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Name != "broken" {
		t.Errorf("error name = %q, want broken", errs[0].Name)
	}
	if len(secs) != 1 || secs[0].Key != "recent" {
		t.Fatalf("expected recent section only, got %+v", secs)
	}
}

// TestService_Build_CalendarShowAllURLUsesUserTimezone proves that the
// calendar show-all URLs generated by Build use the user's configured timezone
// for date formatting, not UTC. It pins the wall clock to a moment where UTC
// and the user's local date differ (2026-06-16 02:00 UTC = 2026-06-15 22:00
// EDT), then asserts the events-today show-all URL contains the local date
// (2026-06-15) and not the UTC date (2026-06-16).
func TestService_Build_CalendarShowAllURLUsesUserTimezone(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	// Configure the user timezone to America/New_York (UTC-4 in June: EDT).
	cs := configstore.New(db)
	if err := cs.Set(ctx, config.KeyUserTimezone, "America/New_York"); err != nil {
		t.Fatalf("set timezone: %v", err)
	}

	// Pin clock to 2026-06-16 02:00:00 UTC.
	// In America/New_York (EDT = UTC-4) this is 2026-06-15 22:00:00.
	// UTC date = "2026-06-16"; local date = "2026-06-15".
	pinnedUTC := time.Date(2026, 6, 16, 2, 0, 0, 0, time.UTC)
	localDate := "2026-06-15"
	utcDate := "2026-06-16"

	// Register a mock provider that returns 6 events-today items (> maxItemsPerSection)
	// so Build assigns ShowAllURL.
	reg := NewRegistry()
	reg.Register(&mockProvider{
		name: "calendar-mock",
		sections: []Section{
			{Key: "events-today", Title: "Today's Events", Items: makeItems(6)},
		},
	})

	svc := New(db, WithClock(func() time.Time { return pinnedUTC }))
	svc.registry = reg

	secs, errs := svc.Build(ctx)
	if len(errs) != 0 {
		t.Fatalf("unexpected provider errors: %v", errs)
	}
	if len(secs) == 0 {
		t.Fatal("expected at least one section (events-today)")
	}

	var eventsToday *Section
	for i := range secs {
		if secs[i].Key == "events-today" {
			eventsToday = &secs[i]
			break
		}
	}
	if eventsToday == nil {
		t.Fatalf("expected events-today section, got keys: %v", func() []string {
			keys := make([]string, len(secs))
			for i, s := range secs {
				keys[i] = s.Key
			}
			return keys
		}())
	}
	if eventsToday.ShowAllURL == "" {
		t.Fatal("expected ShowAllURL set on events-today overflow")
	}

	// The URL must use the local date, not the UTC date.
	if !strings.Contains(eventsToday.ShowAllURL, localDate) {
		t.Errorf("ShowAllURL = %q: want local date %q, not UTC date %q",
			eventsToday.ShowAllURL, localDate, utcDate)
	}
	if strings.Contains(eventsToday.ShowAllURL, utcDate) {
		t.Errorf("ShowAllURL = %q: contains UTC date %q; must use local date %q",
			eventsToday.ShowAllURL, utcDate, localDate)
	}
}
