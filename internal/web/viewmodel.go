package web

import (
	"sort"
	"strings"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/store"
)

// resourceVM is the flat view-model handed to templates. The store layer's
// rows use sql.NullString / NullInt64 which are awkward in templates; the
// view model unwraps them to plain strings/bools.
type resourceVM struct {
	ID           int64
	Title        string
	URL          string
	Description  string
	Notes        string
	Type         string
	Language     string
	CategorySlug string
	CategoryName string
	Tags         []string
	Favorite     bool
	CreatedAt    string
	UpdatedAt    string
	DeletedAt    string
	Trashed      bool
}

// todoVM is the flat view-model for todos. Mirrors resourceVM pattern.
type todoVM struct {
	ID           int64
	Title        string
	Description  string
	Priority     string
	Status       string
	DueDate      string
	Overdue      bool
	CategorySlug string
	CategoryName string
	Tags         []string
	Notes        string
	Recurrence   string
	DoneAt       string
	CreatedAt    string
	UpdatedAt    string
	DeletedAt    string
	Trashed      bool
}

func toTodoVM(t store.Todo, tags []string, catName, catSlug string) todoVM {
	vm := todoVM{
		ID:         t.ID,
		Title:      t.Title,
		Priority:   t.Priority,
		Status:     t.Status,
		Tags:       append([]string(nil), tags...),
		Notes:      nullStrPtr(t.Notes),
		Recurrence: t.Recurrence,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
	if t.Description != nil {
		vm.Description = *t.Description
	}
	if t.DueDate != nil {
		vm.DueDate = *t.DueDate
	}
	if t.DoneAt != nil {
		vm.DoneAt = *t.DoneAt
	}
	if t.DeletedAt != nil {
		vm.DeletedAt = *t.DeletedAt
		vm.Trashed = true
	}
	vm.CategoryName = catName
	vm.CategorySlug = catSlug
	return vm
}

func nullStrPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toVM(lr store.ListedResource) resourceVM {
	vm := resourceVM{
		ID:        lr.Resource.ID,
		Title:     lr.Resource.Title,
		URL:       lr.Resource.Url,
		Type:      lr.Resource.Type,
		Language:  lr.Resource.Language,
		Tags:      append([]string(nil), lr.Tags...),
		Favorite:  lr.Resource.Favorite == 1,
		CreatedAt: lr.Resource.CreatedAt,
		UpdatedAt: lr.Resource.UpdatedAt,
	}
	if lr.Resource.Description.Valid {
		vm.Description = lr.Resource.Description.String
	}
	if lr.Resource.Notes.Valid {
		vm.Notes = lr.Resource.Notes.String
	}
	if lr.CategorySlug.Valid {
		vm.CategorySlug = lr.CategorySlug.String
	}
	if lr.CategoryName.Valid {
		vm.CategoryName = lr.CategoryName.String
	}
	if lr.Resource.DeletedAt.Valid {
		vm.DeletedAt = lr.Resource.DeletedAt.String
		vm.Trashed = true
	}
	return vm
}

func toVMs(rows []store.ListedResource) []resourceVM {
	out := make([]resourceVM, 0, len(rows))
	for _, r := range rows {
		out = append(out, toVM(r))
	}
	return out
}

// typeGroupVM groups resources sharing the same Type so the list view can
// render a header per group. Empty groups are filtered out by groupByType.
type typeGroupVM struct {
	Type      string
	Resources []resourceVM
}

// groupByType buckets vms by their Type field. Groups are returned sorted
// ascending by resource count (fewest first); ties broken by type name in
// canonical declaration order from domain.AllResourceTypes(), with unknown /
// blank types falling through alphabetically.
func groupByType(vms []resourceVM) []typeGroupVM {
	if len(vms) == 0 {
		return nil
	}
	buckets := make(map[string][]resourceVM, len(domain.AllResourceTypes())+1)
	for _, vm := range vms {
		buckets[vm.Type] = append(buckets[vm.Type], vm)
	}
	rank := make(map[string]int, len(domain.AllResourceTypes()))
	for i, t := range domain.AllResourceTypes() {
		rank[string(t)] = i
	}
	out := make([]typeGroupVM, 0, len(buckets))
	for k, rs := range buckets {
		label := k
		if label == "" {
			label = "untyped"
		}
		out = append(out, typeGroupVM{Type: label, Resources: rs})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Resources) != len(out[j].Resources) {
			return len(out[i].Resources) < len(out[j].Resources)
		}
		ri, oki := rank[out[i].Type]
		rj, okj := rank[out[j].Type]
		switch {
		case oki && okj:
			return ri < rj
		case oki:
			return true
		case okj:
			return false
		default:
			return out[i].Type < out[j].Type
		}
	})
	return out
}

// flash is a minimal one-shot status banner passed to layout. Pages set it
// inline (no cookies / sessions); the lifetime is one render.
type flash struct {
	Type    string // ok | warning | danger
	Message string
}

// counts goes into the footer; cheap to compute.
type counts struct {
	Active  int64
	Trashed int64
}

// todoCounts tracks todo-specific counters for the sidebar.
type todoCounts struct {
	Open    int64
	Overdue int64
}

// pageData is the shared envelope every render call passes to the layout.
type pageData struct {
	Title      string
	Nav        string
	Query      string
	Flash      flash
	Counts     counts
	TodoCounts todoCounts
	Sidebar    sidebarVM
	Aside      *asideVM // nil => don't render the right column
}

// sidebarLinkVM is one navigable entry in the persistent left sidebar.
type sidebarLinkVM struct {
	Href   string
	Label  string
	Icon   string
	Count  int64
	Active bool
}

// sidebarVM groups the left-sidebar sections. Each section is a flat slice
// of links so the template iterates without nested logic.
type sidebarVM struct {
	AllActive  bool
	FavActive  bool
	Categories []sidebarLinkVM
	Types      []sidebarLinkVM
	Tags       []sidebarLinkVM
}

// asideVM is the optional right-side assistive panel. List views populate it;
// detail/form pages leave it nil.
type asideVM struct {
	Recent []resourceVM
}

// listFilterVM exposes the active filter to the list view header so the user
// can see what they're scoped to and clear it.
type listFilterVM struct {
	Active bool
	Cat    string
	Tag    string
	Type   string
	Lang   string
	Fav    bool
}

// formVM is the round-trippable form data: rendered both on GET (empty or
// pre-filled) and on POST validation failure (the user's last input).
type formVM struct {
	Title       string
	URL         string
	Description string
	Notes       string
	Type        string
	Language    string
	Category    string // slug
	Tags        string // comma-separated for the input
	Favorite    bool
}

func (f formVM) tagSlice() []string {
	if strings.TrimSpace(f.Tags) == "" {
		return nil
	}
	parts := strings.Split(f.Tags, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
