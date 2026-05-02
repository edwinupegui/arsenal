package web

import (
	"strings"

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

// pageData is the shared envelope every render call passes to the layout.
type pageData struct {
	Title  string
	Nav    string
	Query  string
	Flash  flash
	Counts counts
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
