package providers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
)

// CalendarProvider contributes "Events Today" and "Events Upcoming" sections
// to the Today view. It queries calendar_events using timezone-aware bounds so
// all-day and timed events are captured by a single lexicographic range query.
type CalendarProvider struct {
	queries *store.Queries
	db      *sql.DB
	now     func() time.Time
}

// CalendarProviderOption configures a CalendarProvider.
type CalendarProviderOption func(*CalendarProvider)

// WithCalendarClock replaces the default time.Now source. Used by tests to pin
// the wall-clock for date-sensitive comparisons.
func WithCalendarClock(now func() time.Time) CalendarProviderOption {
	return func(p *CalendarProvider) { p.now = now }
}

// NewCalendarProvider builds a CalendarProvider backed by db.
func NewCalendarProvider(db *sql.DB, opts ...CalendarProviderOption) *CalendarProvider {
	p := &CalendarProvider{queries: store.New(db), db: db, now: time.Now}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the provider identifier.
func (p *CalendarProvider) Name() string { return "calendar" }

// Sections returns up to two sections: events-today and events-upcoming.
// It uses the user's configured timezone for day boundaries. Empty sections
// are omitted. Errors are returned so the registry degrades gracefully.
//
// The range query design relies on the lexicographic property of the storage
// format: all-day rows are stored as "YYYY-MM-DD" which sorts before any
// "YYYY-MM-DDTHH:MM:SS" on the same day, so a single range captures both
// without branching in SQL.
func (p *CalendarProvider) Sections(ctx context.Context) ([]today.Section, error) {
	loc, _ := today.UserLocation(ctx, p.db)
	if loc == nil {
		loc = time.UTC
	}

	now := p.now().In(loc)
	todayDate := now.Format("2006-01-02")

	// dayStart captures all-day rows (date-only strings sort before any T-prefixed datetime).
	dayStart := todayDate
	// dayEnd captures timed rows up to end of local day.
	dayEnd := todayDate + "T23:59:59"
	// weekEnd = today+7 inclusive (also end-of-day so timed events on day+7 are captured).
	weekEnd := now.AddDate(0, 0, 7).Format("2006-01-02") + "T23:59:59"

	var sections []today.Section

	// events-today: start_at in [dayStart, dayEnd]
	todayRows, err := p.queries.ListEventsToday(ctx, store.ListEventsTodayParams{
		StartAt:   dayStart,
		StartAt_2: dayEnd,
	})
	if err != nil {
		return nil, fmt.Errorf("list events today: %w", err)
	}
	if len(todayRows) > 0 {
		items := make([]today.Item, 0, len(todayRows))
		for _, row := range todayRows {
			items = append(items, mapCalendarItem(row))
		}
		sections = append(sections, today.Section{
			Key:     "events-today",
			Title:   "Events Today",
			Items:   items,
			IsEmpty: false,
		})
	}

	// events-upcoming: start_at in (dayEnd, weekEnd]
	upRows, err := p.queries.ListEventsUpcoming(ctx, store.ListEventsUpcomingParams{
		StartAt:   dayEnd,
		StartAt_2: weekEnd,
	})
	if err != nil {
		return nil, fmt.Errorf("list events upcoming: %w", err)
	}
	if len(upRows) > 0 {
		items := make([]today.Item, 0, len(upRows))
		for _, row := range upRows {
			items = append(items, mapCalendarItem(row))
		}
		sections = append(sections, today.Section{
			Key:     "events-upcoming",
			Title:   "Upcoming Events",
			Items:   items,
			IsEmpty: false,
		})
	}

	return sections, nil
}

// mapCalendarItem converts a store.CalendarEvent into a today.Item.
// For timed events the subtitle shows "HH:MM–HH:MM" (or just "HH:MM" when
// end_at is NULL). For all-day events the subtitle is "All day". In both
// cases the location, when non-empty, is appended after " · ".
func mapCalendarItem(row store.CalendarEvent) today.Item {
	var subtitle string
	if row.AllDay == 1 {
		subtitle = "All day"
	} else {
		subtitle = formatTimeRange(row.StartAt, row.EndAt)
	}
	if row.Location != "" {
		subtitle += " · " + row.Location
	}
	return today.Item{
		Domain:   "calendar",
		ID:       row.ID,
		Title:    row.Title,
		Subtitle: subtitle,
		URL:      fmt.Sprintf("/calendar/%d", row.ID),
	}
}

// formatTimeRange builds a "HH:MM–HH:MM" or "HH:MM" string from stored
// start_at / end_at values. start_at is always "YYYY-MM-DDTHH:MM:SS" for
// timed events. end_at is a sql.NullString; when invalid (NULL) only the
// start time is returned.
func formatTimeRange(startAt string, endAt sql.NullString) string {
	start := parseTimeHHMM(startAt)
	if !endAt.Valid || endAt.String == "" {
		return start
	}
	end := parseTimeHHMM(endAt.String)
	return start + "–" + end
}

// parseTimeHHMM extracts "HH:MM" from a "YYYY-MM-DDTHH:MM:SS" string.
// Returns the raw string on parse failure.
func parseTimeHHMM(s string) string {
	// Expect at least "YYYY-MM-DDTHH:MM" (16 chars).
	if len(s) >= 16 && s[10] == 'T' {
		return s[11:16]
	}
	return s
}
