package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/today/providers"
	"github.com/edwinupegui/arsenal/internal/todos"
)

// Handlers holds the shared dependencies every HTTP handler reaches for.
type Handlers struct {
	db           *sql.DB
	queries      *store.Queries
	service      *resources.Service
	todoService  *todos.Service
	todayService *today.Service
	now          func() time.Time
}

func newHandlers(db *sql.DB) *Handlers {
	todaySvc := today.New(db)
	todaySvc.Register(providers.NewTodosProvider(db))
	todaySvc.Register(providers.NewResourcesProvider(db))
	return &Handlers{
		db:           db,
		queries:      store.New(db),
		service:      resources.New(db),
		todoService:  todos.New(db),
		todayService: todaySvc,
		now:          time.Now,
	}
}

// commonPage builds the layout envelope. Counts are best-effort; their
// failure shouldn't take a page down.
func (h *Handlers) commonPage(r *http.Request, title, nav string) pageData {
	pd := pageData{Title: title, Nav: nav, Query: r.URL.Query().Get("q")}
	if active, err := h.queries.CountResources(r.Context()); err == nil {
		pd.Counts.Active = active
	}
	if trashed, err := countTrashed(r.Context(), h.db); err == nil {
		pd.Counts.Trashed = trashed
	}
	if open, err := h.queries.CountOpenTodos(r.Context()); err == nil {
		pd.TodoCounts.Open = open
	}
	if overdue, err := countOverdueTodos(r.Context(), h.db, h.now()); err == nil {
		pd.TodoCounts.Overdue = overdue
	}
	pd.Sidebar = h.buildSidebar(r)
	return pd
}

// buildSidebar assembles the persistent left rail with the current filter
// reflected as Active markers. Best-effort: query failures degrade to empty
// sections rather than blocking the page.
func (h *Handlers) buildSidebar(r *http.Request) sidebarVM {
	q := r.URL.Query()
	curCat := q.Get("cat")
	curTag := q.Get("tag")
	curType := q.Get("type")
	curFav := q.Get("fav") == "1"
	anyFilter := curCat != "" || curTag != "" || curType != "" || curFav ||
		q.Get("lang") != ""
	isResources := r.URL.Path == "/resources"

	sb := sidebarVM{
		AllActive: isResources && !anyFilter,
		FavActive: isResources && curFav,
	}

	if cats, err := h.queries.ListCategoriesWithCounts(r.Context()); err == nil {
		for _, c := range cats {
			if c.ResourceCount == 0 {
				continue
			}
			sb.Categories = append(sb.Categories, sidebarLinkVM{
				Href:   "/resources?cat=" + c.Slug,
				Label:  c.Name,
				Icon:   c.Icon,
				Count:  c.ResourceCount,
				Active: isResources && curCat == c.Slug,
			})
		}
	}

	for _, t := range domain.AllResourceTypes() {
		s := string(t)
		sb.Types = append(sb.Types, sidebarLinkVM{
			Href:   "/resources?type=" + s,
			Label:  s,
			Active: isResources && curType == s,
		})
	}

	if tags, err := h.queries.ListTags(r.Context()); err == nil {
		const maxTags = 15
		for i, t := range tags {
			if i >= maxTags {
				break
			}
			sb.Tags = append(sb.Tags, sidebarLinkVM{
				Href:   "/resources?tag=" + t.Name,
				Label:  t.Name,
				Count:  t.ResourceCount,
				Active: isResources && curTag == t.Name,
			})
		}
	}

	return sb
}

// buildAside builds the right-side assistive panel for list views. Returns
// nil when there is nothing useful to show, so the template suppresses the
// column entirely.
func (h *Handlers) buildAside(r *http.Request) *asideVM {
	rows, err := h.queries.ListResourcesFiltered(r.Context(), store.ListFilter{Limit: 5})
	if err != nil || len(rows) == 0 {
		return nil
	}
	return &asideVM{Recent: toVMs(rows)}
}

// --- list / search / trash ---------------------------------------------------

// listResources renders the main resource grid, filtered by query string params.
func (h *Handlers) listResources(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.ListFilter{
		CategorySlug: q.Get("cat"),
		TagName:      q.Get("tag"),
		Type:         q.Get("type"),
		Language:     q.Get("lang"),
		OnlyFavorite: q.Get("fav") == "1",
		Limit:        500,
	}
	rows, err := h.queries.ListResourcesFiltered(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	vms := toVMs(rows)
	data := struct {
		pageData
		Resources []resourceVM
		Groups    []typeGroupVM
		Filter    listFilterVM
	}{
		pageData:  h.commonPage(r, "Resources", "resources"),
		Resources: vms,
		Groups:    groupByType(vms),
		Filter: listFilterVM{
			Cat: filter.CategorySlug, Tag: filter.TagName,
			Type: filter.Type, Lang: filter.Language, Fav: filter.OnlyFavorite,
		},
	}
	data.Filter.Active = data.Filter.Cat != "" || data.Filter.Tag != "" ||
		data.Filter.Type != "" || data.Filter.Lang != "" || data.Filter.Fav
	if data.Filter.Active && data.Title == "Resources" {
		data.Title = "Resources (filtered)"
	}
	data.Aside = h.buildAside(r)
	render(w, "list", data)
}

func (h *Handlers) trashList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListResourcesFiltered(r.Context(), store.ListFilter{
		Trashed: true, Limit: 500,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vms := toVMs(rows)
	data := struct {
		pageData
		Resources []resourceVM
		Groups    []typeGroupVM
		Filter    listFilterVM
	}{
		pageData:  h.commonPage(r, "Trash", "trash"),
		Resources: vms,
		Groups:    groupByType(vms),
	}
	data.Aside = h.buildAside(r)
	render(w, "list", data)
}

func (h *Handlers) searchResources(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Redirect(w, r, "/resources", http.StatusSeeOther)
		return
	}
	rows, err := h.queries.SearchResources(r.Context(), q, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vms := toVMs(rows)
	data := struct {
		pageData
		Resources []resourceVM
		Groups    []typeGroupVM
		Filter    listFilterVM
	}{
		pageData:  h.commonPage(r, fmt.Sprintf("Search: %s", q), "resources"),
		Resources: vms,
		Groups:    groupByType(vms),
	}
	data.Aside = h.buildAside(r)
	render(w, "list", data)
}

// --- detail ------------------------------------------------------------------

func (h *Handlers) showResource(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	res, err := h.queries.GetResource(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tags, err := h.queries.ListTagsForResource(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tagNames := make([]string, 0, len(tags))
	for _, t := range tags {
		tagNames = append(tagNames, t.Name)
	}
	var catSlug, catName string
	if res.CategoryID.Valid {
		c, cerr := h.queries.GetCategory(r.Context(), res.CategoryID.Int64)
		if cerr == nil {
			catSlug, catName = c.Slug, c.Name
		}
	}
	vm := resourceVM{
		ID: res.ID, Title: res.Title, URL: res.Url,
		Type: res.Type, Language: res.Language,
		Tags: tagNames, CategorySlug: catSlug, CategoryName: catName,
		Favorite:  res.Favorite == 1,
		CreatedAt: res.CreatedAt, UpdatedAt: res.UpdatedAt,
	}
	if res.Description.Valid {
		vm.Description = res.Description.String
	}
	if res.Notes.Valid {
		vm.Notes = res.Notes.String
	}
	if res.DeletedAt.Valid {
		vm.DeletedAt, vm.Trashed = res.DeletedAt.String, true
	}

	data := struct {
		pageData
		Resource resourceVM
	}{
		pageData: h.commonPage(r, vm.Title, "resources"),
		Resource: vm,
	}
	render(w, "detail", data)
}

// --- new / edit / create / update -------------------------------------------

func (h *Handlers) newResourceForm(w http.ResponseWriter, r *http.Request) {
	h.renderForm(w, r, formVM{Type: string(domain.TypeArticle), Language: string(domain.LangEN)},
		"Add resource", "/resources", "/resources", "Add", "")
}

func (h *Handlers) createResource(w http.ResponseWriter, r *http.Request) {
	form := formFromRequest(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	catID, err := h.resolveCategory(r, form.Category)
	if err != nil {
		h.renderForm(w, r, form, "Add resource", "/resources", "/resources", "Add", err.Error())
		return
	}
	res, err := h.service.Create(r.Context(), resources.CreateInput{
		Title:       form.Title,
		URL:         form.URL,
		Description: form.Description,
		Type:        domain.ResourceType(form.Type),
		Language:    domain.Language(form.Language),
		CategoryID:  catID,
		Notes:       form.Notes,
		Favorite:    form.Favorite,
		Tags:        form.tagSlice(),
	})
	if err != nil {
		h.renderForm(w, r, form, "Add resource", "/resources", "/resources", "Add", err.Error())
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/resources/%d", res.Row.ID), http.StatusSeeOther)
}

func (h *Handlers) editResourceForm(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	res, err := h.queries.GetResource(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tags, _ := h.queries.ListTagsForResource(r.Context(), id)
	tagStrs := make([]string, 0, len(tags))
	for _, t := range tags {
		tagStrs = append(tagStrs, t.Name)
	}
	var catSlug string
	if res.CategoryID.Valid {
		c, cerr := h.queries.GetCategory(r.Context(), res.CategoryID.Int64)
		if cerr == nil {
			catSlug = c.Slug
		}
	}
	form := formVM{
		Title: res.Title, URL: res.Url,
		Description: nullStr(res.Description),
		Notes:       nullStr(res.Notes),
		Type:        res.Type, Language: res.Language,
		Category: catSlug,
		Tags:     strings.Join(tagStrs, ", "),
		Favorite: res.Favorite == 1,
	}
	action := fmt.Sprintf("/resources/%d", id)
	cancel := action
	h.renderForm(w, r, form, "Edit resource", action, cancel, "Save", "")
}

func (h *Handlers) updateResource(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	form := formFromRequest(r)
	catID, err := h.resolveCategory(r, form.Category)
	if err != nil {
		h.renderForm(w, r, form, "Edit resource",
			fmt.Sprintf("/resources/%d", id), fmt.Sprintf("/resources/%d", id),
			"Save", err.Error())
		return
	}
	if _, err := h.service.Update(r.Context(), id, resources.UpdateInput{
		Title:       form.Title,
		URL:         form.URL,
		Description: form.Description,
		Type:        domain.ResourceType(form.Type),
		Language:    domain.Language(form.Language),
		CategoryID:  catID,
		Notes:       form.Notes,
		Tags:        form.tagSlice(),
	}); err != nil {
		h.renderForm(w, r, form, "Edit resource",
			fmt.Sprintf("/resources/%d", id), fmt.Sprintf("/resources/%d", id),
			"Save", err.Error())
		return
	}
	if form.Favorite {
		_ = h.service.SetFavorite(r.Context(), id, true)
	} else {
		_ = h.service.SetFavorite(r.Context(), id, false)
	}
	http.Redirect(w, r, fmt.Sprintf("/resources/%d", id), http.StatusSeeOther)
}

// renderForm centralizes the data envelope for the new / edit form so both
// callers stay tiny and consistent.
func (h *Handlers) renderForm(w http.ResponseWriter, r *http.Request, form formVM,
	pageTitle, action, cancel, submit, errMsg string) {
	cats, _ := h.queries.ListCategories(r.Context())
	data := struct {
		pageData
		Title       string
		Action      string
		CancelURL   string
		SubmitLabel string
		FormError   string
		Form        formVM
		Categories  []store.Category
		Types       []domain.ResourceType
		Languages   []domain.Language
	}{
		pageData:    h.commonPage(r, pageTitle, "resources"),
		Title:       pageTitle,
		Action:      action,
		CancelURL:   cancel,
		SubmitLabel: submit,
		FormError:   errMsg,
		Form:        form,
		Categories:  cats,
		Types:       domain.AllResourceTypes(),
		Languages:   domain.AllLanguages(),
	}
	render(w, "form", data)
}

// --- mutations: delete / restore / star -------------------------------------

func (h *Handlers) softDeleteResource(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.service.SoftDelete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		// HTMX swaps the card with empty content so the row disappears in place.
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(""))
		return
	}
	http.Redirect(w, r, "/resources", http.StatusSeeOther)
}

func (h *Handlers) purgeResource(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.service.Purge(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(""))
		return
	}
	http.Redirect(w, r, "/trash", http.StatusSeeOther)
}

func (h *Handlers) restoreResource(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.service.Restore(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/resources/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (h *Handlers) toggleStar(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	res, err := h.queries.GetResource(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next := res.Favorite != 1
	if err := h.service.SetFavorite(r.Context(), id, next); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		// Re-render only the card so the page doesn't reload.
		rows, _ := h.queries.ListResourcesFiltered(r.Context(), store.ListFilter{
			Limit: 1,
		})
		var lr store.ListedResource
		for _, row := range rows {
			if row.Resource.ID == id {
				lr = row
				break
			}
		}
		if lr.Resource.ID == 0 {
			// Fallback: rebuild minimal card.
			tags, _ := h.queries.ListTagsForResource(r.Context(), id)
			names := make([]string, 0, len(tags))
			for _, t := range tags {
				names = append(names, t.Name)
			}
			updated, _ := h.queries.GetResource(r.Context(), id)
			lr = store.ListedResource{Resource: updated, Tags: names}
		}
		t := pages["list"]
		_ = t.ExecuteTemplate(w, "card", toVM(lr))
		return
	}
	http.Redirect(w, r, "/resources/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// --- categories / tags lists -------------------------------------------------

func (h *Handlers) listCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListCategoriesWithCounts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		pageData
		Categories []store.ListCategoriesWithCountsRow
	}{
		pageData:   h.commonPage(r, "Categories", "categories"),
		Categories: rows,
	}
	render(w, "categories", data)
}

func (h *Handlers) listTags(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListTags(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		pageData
		Tags []store.ListTagsRow
	}{
		pageData: h.commonPage(r, "Tags", "tags"),
		Tags:     rows,
	}
	render(w, "tags", data)
}

// --- helpers -----------------------------------------------------------------

// chiID parses the {id} URL parameter as an int64. Every handler that
// takes a positional resource id reaches for this exact lookup, so
// hard-coding the parameter name keeps the call sites tiny.
func chiID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func nullStr(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	return s.String
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

func formFromRequest(r *http.Request) formVM {
	if err := r.ParseForm(); err != nil {
		return formVM{}
	}
	return formVM{
		Title:       strings.TrimSpace(r.Form.Get("title")),
		URL:         strings.TrimSpace(r.Form.Get("url")),
		Description: strings.TrimSpace(r.Form.Get("description")),
		Notes:       strings.TrimSpace(r.Form.Get("notes")),
		Type:        strings.TrimSpace(r.Form.Get("type")),
		Language:    strings.TrimSpace(r.Form.Get("language")),
		Category:    strings.TrimSpace(r.Form.Get("category")),
		Tags:        strings.TrimSpace(r.Form.Get("tags")),
		Favorite:    r.Form.Get("favorite") == "1",
	}
}

func (h *Handlers) resolveCategory(r *http.Request, slug string) (*int64, error) {
	if slug == "" {
		return nil, nil
	}
	cat, err := h.queries.GetCategoryBySlug(r.Context(), slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("category %q not found", slug)
	}
	if err != nil {
		return nil, err
	}
	return &cat.ID, nil
}

func countTrashed(ctx context.Context, db *sql.DB) (int64, error) {
	var n int64
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM resources WHERE deleted_at IS NOT NULL`,
	).Scan(&n)
	return n, err
}

func countOverdueTodos(ctx context.Context, db *sql.DB, now time.Time) (int64, error) {
	loc, err := today.UserLocation(ctx, db)
	if err != nil {
		return 0, fmt.Errorf("user location: %w", err)
	}
	todayStr := now.In(loc).Format("2006-01-02")
	var n int64
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM todos WHERE status = 'open' AND deleted_at IS NULL AND due_date < ?`,
		todayStr,
	).Scan(&n)
	return n, err
}
