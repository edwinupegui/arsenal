package today

import (
	"context"
	"database/sql"
	"sort"
)

const maxItemsPerSection = 5

// Service orchestrates the Today view. It owns the registry and applies
// section ordering, density truncation, and empty-state decisions.
type Service struct {
	db       *sql.DB
	registry *Registry
}

// New builds a Service with an empty registry. Callers should register
// providers via s.registry.Register(...). This avoids a circular import with
// the providers subpackage.
func New(db *sql.DB) *Service {
	return &Service{db: db, registry: NewRegistry()}
}

// NewWithRegistry builds a Service with a pre-built registry. Useful for
// tests that inject mock providers.
func NewWithRegistry(registry *Registry) *Service {
	return &Service{registry: registry}
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

	var out []Section
	for _, sec := range sections {
		if len(sec.Items) == 0 || sec.IsEmpty {
			continue
		}
		if len(sec.Items) > maxItemsPerSection {
			sec.Items = sec.Items[:maxItemsPerSection]
			sec.ShowAllURL = showAllURLFor(sec.Key)
		}
		out = append(out, sec)
	}
	return out, errs
}

func showAllURLFor(key string) string {
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
	default:
		return ""
	}
}
