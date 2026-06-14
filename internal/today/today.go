package today

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

const maxItemsPerSection = 5

// Service orchestrates the Today view. It owns the registry and applies
// section ordering, density truncation, and empty-state decisions.
type Service struct {
	db       *sql.DB
	registry *Registry
	clock    func() time.Time
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithClock replaces the default time.Now source. Used by tests to pin the
// wall-clock for date-sensitive comparisons.
func WithClock(clock func() time.Time) ServiceOption {
	return func(s *Service) { s.clock = clock }
}

// New builds a Service with an empty registry. Callers should register
// providers via s.registry.Register(...). This avoids a circular import with
// the providers subpackage.
func New(db *sql.DB, opts ...ServiceOption) *Service {
	s := &Service{db: db, registry: NewRegistry(), clock: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// NewWithRegistry builds a Service with a pre-built registry. Useful for
// tests that inject mock providers.
func NewWithRegistry(registry *Registry, opts ...ServiceOption) *Service {
	s := &Service{registry: registry, clock: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Register adds a provider to the service registry.
func (s *Service) Register(p Provider) {
	s.registry.Register(p)
}

// Build collects sections from all providers, orders them, truncates to
// density limits, and sets ShowAllURL for overflow sections. Returns the
// final ordered slice plus any provider errors for graceful degradation.
func (s *Service) Build(ctx context.Context) ([]Section, []ProviderError) {
	sections, errs := s.registry.Collect(ctx)

	// Sort by fixed section order; unknown keys append at end.
	sort.SliceStable(sections, func(i, j int) bool {
		io, oki := sectionOrder[sections[i].Key]
		jo, okj := sectionOrder[sections[j].Key]
		if oki && okj {
			return io < jo
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return false
	})

	// Derive now in the user's local timezone so calendar show-all URL date
	// windows match the CalendarProvider's tz-aware "today"/"upcoming" windows.
	// Guard against a nil db (NewWithRegistry path) by falling back to UTC.
	var loc *time.Location
	if s.db != nil {
		loc, _ = UserLocation(ctx, s.db)
	}
	if loc == nil {
		loc = time.UTC
	}
	now := s.clock().In(loc)

	var out []Section
	for _, sec := range sections {
		if len(sec.Items) == 0 || sec.IsEmpty {
			continue
		}
		if len(sec.Items) > maxItemsPerSection {
			sec.Items = sec.Items[:maxItemsPerSection]
			sec.ShowAllURL = showAllURLFor(sec.Key, now)
		}
		out = append(out, sec)
	}
	return out, errs
}

// showAllURLFor returns the "show all" destination URL for a given section key.
// For calendar sections the URL uses from/to date params (REQ-TV-09) so the
// link actually filters the calendar list to the matching window. now must be
// expressed in the user's local timezone so date formatting matches the
// CalendarProvider's tz-aware day boundaries.
func showAllURLFor(key string, now time.Time) string {
	switch key {
	case "overdue":
		return "/todos?status=open&overdue=true"
	case "due-today":
		return "/todos?status=open&due=today"
	case "upcoming":
		return "/todos?status=open&due=upcoming"
	case "recent":
		return "/resources"
	case "this-month-spending":
		return "/finance?kind=expense"
	case "recent-transactions":
		return "/finance"
	case "events-today":
		today := now.Format("2006-01-02")
		return "/calendar?from=" + today + "&to=" + today
	case "events-upcoming":
		tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
		todayPlus7 := now.AddDate(0, 0, 7).Format("2006-01-02")
		return "/calendar?from=" + tomorrow + "&to=" + todayPlus7
	default:
		return ""
	}
}
