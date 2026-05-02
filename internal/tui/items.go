package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"

	"github.com/edwinupegui/arsenal/internal/store"
)

// resourceItem adapts a store.ListedResource to bubbles/list's Item interface.
// Title and Description are what bubbles renders by default; FilterValue is
// what its built-in fuzzy filter searches over.
type resourceItem struct {
	r store.ListedResource
}

func newResourceItem(r store.ListedResource) resourceItem { return resourceItem{r: r} }

func (i resourceItem) Title() string {
	star := ""
	if i.r.Resource.Favorite == 1 {
		star = "★ "
	}
	return star + i.r.Resource.Title
}

func (i resourceItem) Description() string {
	parts := []string{i.r.Resource.Type, i.r.Resource.Language}
	if i.r.CategorySlug.Valid {
		parts = append(parts, i.r.CategorySlug.String)
	}
	if len(i.r.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(i.r.Tags, " #"))
	}
	return fmt.Sprintf("%s — %s", strings.Join(parts, " · "), i.r.Resource.Url)
}

// FilterValue feeds the bubbles/list incremental filter. Title + URL + tags
// gives a wide enough surface for typical "find that one resource" jumps.
func (i resourceItem) FilterValue() string {
	return strings.Join([]string{
		i.r.Resource.Title,
		i.r.Resource.Url,
		strings.Join(i.r.Tags, " "),
	}, " ")
}

// asItems converts a slice of ListedResource into the slice type bubbles/list
// expects. Done in one place so callers don't sprinkle the conversion.
func asItems(rows []store.ListedResource) []list.Item {
	out := make([]list.Item, 0, len(rows))
	for _, r := range rows {
		out = append(out, newResourceItem(r))
	}
	return out
}
