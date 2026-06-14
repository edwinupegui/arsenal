package providers_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/edwinupegui/arsenal/internal/calendar"
	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/today/providers"
)

func newCalendarTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// pinClock returns a clock func that always returns t.
func pinClock(ts time.Time) func() time.Time { return func() time.Time { return ts } }

// todayDate returns the local date for the given time as YYYY-MM-DD.
func todayDate(t time.Time) string { return t.Format("2006-01-02") }

func seedCalendarEvent(t *testing.T, db *sql.DB, title, startAt, endAt string, allDay bool, location string) *calendar.Event {
	t.Helper()
	svc := calendar.New(db)
	ev, err := svc.Create(context.Background(), calendar.CreateInput{
		Title:    title,
		StartAt:  startAt,
		EndAt:    endAt,
		AllDay:   allDay,
		Location: location,
	})
	if err != nil {
		t.Fatalf("seedCalendarEvent %q: %v", title, err)
	}
	return ev
}

// --- Name ---

func TestCalendarProvider_Name(t *testing.T) {
	db := newCalendarTestDB(t)
	p := providers.NewCalendarProvider(db)
	if got := p.Name(); got != "calendar" {
		t.Errorf("Name() = %q, want calendar", got)
	}
}

func TestCalendarProvider_ImplementsProvider(t *testing.T) {
	var _ today.Provider = providers.NewCalendarProvider(nil)
}

// --- events-today section ---

func TestCalendarProvider_EventsTodayIncludesTimedEvent(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	seedCalendarEvent(t, db, "Morning standup", "2026-06-15T09:00:00", "2026-06-15T09:30:00", false, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	sec := findSection(secs, "events-today")
	if sec == nil {
		t.Fatalf("expected events-today section, got %v", sectionKeys(secs))
	}
	if len(sec.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(sec.Items))
	}
	if sec.Items[0].Title != "Morning standup" {
		t.Errorf("item title = %q, want Morning standup", sec.Items[0].Title)
	}
}

func TestCalendarProvider_EventsTodayIncludesAllDayEvent(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	seedCalendarEvent(t, db, "Team holiday", "2026-06-15", "", true, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	sec := findSection(secs, "events-today")
	if sec == nil {
		t.Fatalf("expected events-today section, got %v", sectionKeys(secs))
	}
	if len(sec.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(sec.Items))
	}
}

func TestCalendarProvider_EventsTodayExcludesOtherDays(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	// yesterday
	seedCalendarEvent(t, db, "Yesterday meeting", "2026-06-14T09:00:00", "", false, "")
	// tomorrow
	seedCalendarEvent(t, db, "Tomorrow meeting", "2026-06-16T09:00:00", "", false, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if sec := findSection(secs, "events-today"); sec != nil {
		t.Errorf("expected no events-today section, got %d items", len(sec.Items))
	}
}

func TestCalendarProvider_EventsTodayExcludesSoftDeleted(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	ev := seedCalendarEvent(t, db, "Trashed meeting", "2026-06-15T09:00:00", "", false, "")
	svc := calendar.New(db)
	if err := svc.SoftDelete(context.Background(), ev.Row.ID); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if sec := findSection(secs, "events-today"); sec != nil {
		t.Errorf("expected no events-today section for deleted event, got %d items", len(sec.Items))
	}
}

func TestCalendarProvider_EventsTodayOmittedWhenEmpty(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if sec := findSection(secs, "events-today"); sec != nil {
		t.Errorf("expected no events-today section when DB empty, got it")
	}
}

// --- events-upcoming section ---

func TestCalendarProvider_EventsUpcomingIncludesNextSevenDays(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	// tomorrow = today+1
	seedCalendarEvent(t, db, "Tomorrow event", "2026-06-16T09:00:00", "", false, "")
	// in 7 days (boundary, inclusive)
	seedCalendarEvent(t, db, "Seven days event", "2026-06-22T09:00:00", "", false, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	sec := findSection(secs, "events-upcoming")
	if sec == nil {
		t.Fatalf("expected events-upcoming section, got %v", sectionKeys(secs))
	}
	if len(sec.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(sec.Items))
	}
}

func TestCalendarProvider_EventsUpcomingExcludesBeyondSevenDays(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	// today+8 — must be excluded
	seedCalendarEvent(t, db, "Eight days event", "2026-06-23T09:00:00", "", false, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if sec := findSection(secs, "events-upcoming"); sec != nil {
		t.Errorf("expected no events-upcoming section (event beyond 7 days), got %d items", len(sec.Items))
	}
}

func TestCalendarProvider_EventsUpcomingOmittedWhenEmpty(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if sec := findSection(secs, "events-upcoming"); sec != nil {
		t.Errorf("expected no events-upcoming section when DB empty")
	}
}

// --- Item mapping ---

func TestCalendarProvider_ItemMappingTimedRange(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	seedCalendarEvent(t, db, "Sprint review", "2026-06-15T09:00:00", "2026-06-15T10:30:00", false, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	sec := findSection(secs, "events-today")
	if sec == nil || len(sec.Items) == 0 {
		t.Fatalf("expected events-today with items")
	}
	it := sec.Items[0]
	if it.Domain != "calendar" {
		t.Errorf("domain = %q, want calendar", it.Domain)
	}
	if it.Title != "Sprint review" {
		t.Errorf("title = %q, want Sprint review", it.Title)
	}
	// Subtitle should contain the time range
	if !strings.Contains(it.Subtitle, "09:00") {
		t.Errorf("subtitle %q missing 09:00", it.Subtitle)
	}
	if !strings.Contains(it.Subtitle, "10:30") {
		t.Errorf("subtitle %q missing 10:30", it.Subtitle)
	}
	wantURL := fmt.Sprintf("/calendar/%d", sec.Items[0].ID)
	if it.URL != wantURL {
		t.Errorf("url = %q, want %q", it.URL, wantURL)
	}
}

func TestCalendarProvider_ItemMappingAllDay(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	seedCalendarEvent(t, db, "Public holiday", "2026-06-15", "", true, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	sec := findSection(secs, "events-today")
	if sec == nil || len(sec.Items) == 0 {
		t.Fatalf("expected events-today with items")
	}
	it := sec.Items[0]
	if it.Subtitle != "All day" {
		t.Errorf("subtitle = %q, want All day", it.Subtitle)
	}
}

func TestCalendarProvider_ItemMappingNoEnd(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	seedCalendarEvent(t, db, "Open event", "2026-06-15T14:00:00", "", false, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	sec := findSection(secs, "events-today")
	if sec == nil || len(sec.Items) == 0 {
		t.Fatalf("expected events-today with items")
	}
	it := sec.Items[0]
	// Subtitle should contain start time but no "–" separator (no end)
	if !strings.Contains(it.Subtitle, "14:00") {
		t.Errorf("subtitle %q missing 14:00", it.Subtitle)
	}
}

func TestCalendarProvider_ItemMappingWithLocation(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	seedCalendarEvent(t, db, "On-site meeting", "2026-06-15T09:00:00", "", false, "Conference Room A")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	sec := findSection(secs, "events-today")
	if sec == nil || len(sec.Items) == 0 {
		t.Fatalf("expected events-today with items")
	}
	it := sec.Items[0]
	if !strings.Contains(it.Subtitle, "Conference Room A") {
		t.Errorf("subtitle %q missing location", it.Subtitle)
	}
	if !strings.Contains(it.Subtitle, " · ") {
		t.Errorf("subtitle %q missing separator before location", it.Subtitle)
	}
}

// --- Timezone ---

func TestCalendarProvider_TimezoneRespected(t *testing.T) {
	db := newCalendarTestDB(t)
	ctx := context.Background()

	// Set timezone to UTC-3 (Buenos Aires)
	cs := configstore.New(db)
	if err := cs.Set(ctx, config.KeyUserTimezone, "America/Argentina/Buenos_Aires"); err != nil {
		t.Fatalf("set timezone: %v", err)
	}

	// Wall clock: 2026-06-16 01:00 UTC = 2026-06-15 22:00 local (-03:00)
	// So "today" in local time is 2026-06-15.
	now := time.Date(2026, 6, 16, 1, 0, 0, 0, time.UTC)
	seedCalendarEvent(t, db, "Local-today event", "2026-06-15T09:00:00", "", false, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(ctx)
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	sec := findSection(secs, "events-today")
	if sec == nil {
		t.Fatalf("expected events-today section (event is today in local tz), got %v", sectionKeys(secs))
	}
}

func TestCalendarProvider_UTCFallbackOnInvalidTimezone(t *testing.T) {
	db := newCalendarTestDB(t)
	ctx := context.Background()

	cs := configstore.New(db)
	if err := cs.Set(ctx, config.KeyUserTimezone, "Not/ATimezone"); err != nil {
		t.Fatalf("set timezone: %v", err)
	}

	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	seedCalendarEvent(t, db, "UTC event", "2026-06-15T09:00:00", "", false, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	// Must not error — should fall back to UTC silently
	secs, err := p.Sections(ctx)
	if err != nil {
		t.Fatalf("Sections with invalid tz: %v", err)
	}
	sec := findSection(secs, "events-today")
	if sec == nil {
		t.Fatalf("expected events-today section with UTC fallback, got %v", sectionKeys(secs))
	}
}

// --- Section titles ---

// TestCalendarProvider_SectionTitles asserts that section Title strings match
// the spec exactly: "Today's Events" (events-today) and "Upcoming Events"
// (events-upcoming). REQ-TP-08 / calendar-provider spec.
func TestCalendarProvider_SectionTitles(t *testing.T) {
	db := newCalendarTestDB(t)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	// Seed one event today and one tomorrow so both sections appear.
	seedCalendarEvent(t, db, "Today event", "2026-06-15T09:00:00", "", false, "")
	seedCalendarEvent(t, db, "Tomorrow event", "2026-06-16T09:00:00", "", false, "")

	p := providers.NewCalendarProvider(db, providers.WithCalendarClock(pinClock(now)))
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}

	todaySec := findSection(secs, "events-today")
	if todaySec == nil {
		t.Fatalf("expected events-today section")
	}
	if todaySec.Title != "Today's Events" {
		t.Errorf("events-today Title = %q, want \"Today's Events\"", todaySec.Title)
	}

	upSec := findSection(secs, "events-upcoming")
	if upSec == nil {
		t.Fatalf("expected events-upcoming section")
	}
	if upSec.Title != "Upcoming Events" {
		t.Errorf("events-upcoming Title = %q, want \"Upcoming Events\"", upSec.Title)
	}
}

// --- helpers ---

func findSection(secs []today.Section, key string) *today.Section {
	for i := range secs {
		if secs[i].Key == key {
			return &secs[i]
		}
	}
	return nil
}

func sectionKeys(secs []today.Section) []string {
	keys := make([]string, len(secs))
	for i, s := range secs {
		keys[i] = s.Key
	}
	return keys
}
