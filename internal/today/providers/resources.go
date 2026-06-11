package providers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
)

// ResourcesProvider contributes a recent-resources section.
type ResourcesProvider struct {
	queries *store.Queries
}

// NewResourcesProvider builds a ResourcesProvider backed by db.
func NewResourcesProvider(db *sql.DB) *ResourcesProvider {
	return &ResourcesProvider{queries: store.New(db)}
}

// Name returns the provider identifier.
func (p *ResourcesProvider) Name() string { return "resources" }

// Sections returns one section: recent resources. Limit is a practical
// upper bound; today.Service.Build caps to density (5) and sets the
// "show all" URL when the provider returns more.
func (p *ResourcesProvider) Sections(ctx context.Context) ([]today.Section, error) {
	rows, err := p.queries.ListResourcesFiltered(ctx, store.ListFilter{Limit: 100})
	if err != nil {
		return nil, fmt.Errorf("recent resources query: %w", err)
	}
	items := mapResourceItems(rows)
	if len(items) == 0 {
		return nil, nil
	}
	return []today.Section{{
		Key:     "recent",
		Title:   "Recent Resources",
		Items:   items,
		IsEmpty: false,
	}}, nil
}

func mapResourceItem(row store.ListedResource) today.Item {
	return today.Item{
		Domain:   "resources",
		ID:       row.Resource.ID,
		Title:    row.Resource.Title,
		Subtitle: row.Resource.Type,
		Priority: "",
		Tags:     row.Tags,
		URL:      fmt.Sprintf("/resources/%d", row.Resource.ID),
	}
}

func mapResourceItems(rows []store.ListedResource) []today.Item {
	out := make([]today.Item, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapResourceItem(row))
	}
	return out
}
