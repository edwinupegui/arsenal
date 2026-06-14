package web

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/edwinupegui/arsenal/internal/calendar"
	"github.com/edwinupegui/arsenal/internal/store"
)

// calendarPageData is the unified data envelope for every calendar template
// render path. Using a single struct avoids "can't evaluate field X" template
// errors when the content block dispatches on .Kind / .Event.ID.
type calendarPageData struct {
	pageData
	// Dispatch key
	Kind string
	// List view
	Events []calendarVM
	Filter calendarFilterVM
	// Detail view
	Event calendarVM
	// Form view
	Title       string
	Action      string
	CancelURL   string
	SubmitLabel string
	FormError   string
	Form        calendarFormVM
	Categories  []store.Category
	Recurrences []calendar.Recurrence
}

// calendarRoutes registers all /calendar/* routes on r. Called from server.go.
func (h *Handlers) calendarRoutes(r chi.Router) {
	r.Get("/calendar", h.listCalendar)
	r.Get("/calendar/new", h.newCalendarForm)
	r.Post("/calendar", h.createCalendar)
	r.Get("/calendar/{id}", h.showCalendar)
	r.Get("/calendar/{id}/edit", h.editCalendarForm)
	r.Post("/calendar/{id}", h.updateCalendar)
	r.Post("/calendar/{id}/delete", h.softDeleteCalendar)
	r.Post("/calendar/{id}/restore", h.restoreCalendar)
	r.Post("/calendar/{id}/purge", h.purgeCalendar)
}

// --- list / new / create -----------------------------------------------------

func (h *Handlers) listCalendar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var fromPtr, toPtr, recurrencePtr *string
	if v := q.Get("from"); v != "" {
		fromPtr = &v
	}
	if v := q.Get("to"); v != "" {
		toPtr = &v
	}
	if v := q.Get("recurrence"); v != "" {
		recurrencePtr = &v
	}

	// ?when=today / ?when=upcoming are shortcut filters from Today view links.
	trashed := q.Get("trashed") == "1"
	f := calendar.Filter{
		From:         fromPtr,
		To:           toPtr,
		Recurrence:   recurrencePtr,
		CategorySlug: q.Get("cat"),
		TagName:      q.Get("tag"),
		Trashed:      trashed,
		Limit:        500,
	}

	evts, err := h.calendarService.List(r.Context(), f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filter := calendarFilterVM{
		From:       q.Get("from"),
		To:         q.Get("to"),
		Recurrence: q.Get("recurrence"),
		Cat:        q.Get("cat"),
		Tag:        q.Get("tag"),
		Trashed:    trashed,
		When:       q.Get("when"),
	}
	filter.Active = filter.From != "" || filter.To != "" ||
		filter.Recurrence != "" || filter.Cat != "" ||
		filter.Tag != "" || filter.Trashed || filter.When != ""

	data := calendarPageData{
		pageData: h.commonPage(r, "Calendar", "calendar"),
		Events:   toCalendarVMs(evts),
		Filter:   filter,
		Title:    "Calendar",
	}
	render(w, "calendar", data)
}

func (h *Handlers) newCalendarForm(w http.ResponseWriter, r *http.Request) {
	cats, _ := h.queries.ListCategories(r.Context())
	data := calendarPageData{
		pageData:    h.commonPage(r, "New event", "calendar"),
		Kind:        "new",
		Title:       "New event",
		Action:      "/calendar",
		CancelURL:   "/calendar",
		SubmitLabel: "Create",
		Form:        calendarFormVM{Recurrence: string(calendar.RecurrenceNone)},
		Categories:  cats,
		Recurrences: calendar.AllRecurrences(),
	}
	render(w, "calendar", data)
}

func (h *Handlers) createCalendar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	form := calendarFormFromRequest(r)
	cats, _ := h.queries.ListCategories(r.Context())
	catID, err := h.resolveCategory(r, form.Category)
	if err != nil {
		render(w, "calendar", calendarPageData{
			pageData: h.commonPage(r, "New event", "calendar"),
			Kind: "new", Title: "New event",
			Action: "/calendar", CancelURL: "/calendar", SubmitLabel: "Create",
			FormError: err.Error(), Form: form,
			Categories: cats, Recurrences: calendar.AllRecurrences(),
		})
		return
	}
	startAt, endAt := composeStartEnd(form)
	in := calendar.CreateInput{
		Title:       form.Title,
		Description: form.Description,
		StartAt:     startAt,
		EndAt:       endAt,
		AllDay:      form.AllDay,
		Location:    form.Location,
		CategoryID:  catID,
		Notes:       form.Notes,
		Recurrence:  calendar.Recurrence(form.Recurrence),
		Tags:        form.tagSlice(),
	}
	if _, err := h.calendarService.Create(r.Context(), in); err != nil {
		render(w, "calendar", calendarPageData{
			pageData: h.commonPage(r, "New event", "calendar"),
			Kind: "new", Title: "New event",
			Action: "/calendar", CancelURL: "/calendar", SubmitLabel: "Create",
			FormError: err.Error(), Form: form,
			Categories: cats, Recurrences: calendar.AllRecurrences(),
		})
		return
	}
	http.Redirect(w, r, "/calendar", http.StatusSeeOther)
}

// --- show / edit / update ----------------------------------------------------

func (h *Handlers) showCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	ev, err := h.calendarService.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var catName, catSlug string
	if ev.Row.CategoryID.Valid {
		c, cerr := h.queries.GetCategory(r.Context(), ev.Row.CategoryID.Int64)
		if cerr == nil {
			catName, catSlug = c.Name, c.Slug
		}
	}
	vm := toCalendarVM(ev, catName, catSlug)
	data := calendarPageData{
		pageData: h.commonPage(r, vm.Title, "calendar"),
		Event:    vm,
	}
	render(w, "calendar", data)
}

func (h *Handlers) editCalendarForm(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	ev, err := h.calendarService.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cats, _ := h.queries.ListCategories(r.Context())
	var catSlug string
	if ev.Row.CategoryID.Valid {
		c, cerr := h.queries.GetCategory(r.Context(), ev.Row.CategoryID.Int64)
		if cerr == nil {
			catSlug = c.Slug
		}
	}

	// Split start_at into date + time components for the form.
	startDate, startTime := splitStartAt(ev.Row.StartAt)
	endDate, endTime := "", ""
	if ev.Row.EndAt.Valid && ev.Row.EndAt.String != "" {
		endDate, endTime = splitStartAt(ev.Row.EndAt.String)
	}

	form := calendarFormVM{
		Title:       ev.Row.Title,
		Description: nullStr(ev.Row.Description),
		StartDate:   startDate,
		StartTime:   startTime,
		EndDate:     endDate,
		EndTime:     endTime,
		AllDay:      ev.Row.AllDay == 1,
		Location:    ev.Row.Location,
		Category:    catSlug,
		Notes:       nullStr(ev.Row.Notes),
		Recurrence:  ev.Row.Recurrence,
		Tags:        strings.Join(ev.Tags, ", "),
	}
	action := fmt.Sprintf("/calendar/%d", id)
	data := calendarPageData{
		pageData:    h.commonPage(r, "Edit event", "calendar"),
		Kind:        "edit",
		Title:       "Edit event",
		Action:      action,
		CancelURL:   action,
		SubmitLabel: "Save",
		Form:        form,
		Categories:  cats,
		Recurrences: calendar.AllRecurrences(),
	}
	render(w, "calendar", data)
}

func (h *Handlers) updateCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	form := calendarFormFromRequest(r)
	cats, _ := h.queries.ListCategories(r.Context())
	action := fmt.Sprintf("/calendar/%d", id)
	catID, err := h.resolveCategory(r, form.Category)
	if err != nil {
		render(w, "calendar", calendarPageData{
			pageData: h.commonPage(r, "Edit event", "calendar"),
			Kind: "edit", Title: "Edit event",
			Action: action, CancelURL: action, SubmitLabel: "Save",
			FormError: err.Error(), Form: form,
			Categories: cats, Recurrences: calendar.AllRecurrences(),
		})
		return
	}
	startAt, endAt := composeStartEnd(form)
	in := calendar.CreateInput{
		Title:       form.Title,
		Description: form.Description,
		StartAt:     startAt,
		EndAt:       endAt,
		AllDay:      form.AllDay,
		Location:    form.Location,
		CategoryID:  catID,
		Notes:       form.Notes,
		Recurrence:  calendar.Recurrence(form.Recurrence),
		Tags:        form.tagSlice(),
	}
	if _, err := h.calendarService.Update(r.Context(), id, in); err != nil {
		render(w, "calendar", calendarPageData{
			pageData: h.commonPage(r, "Edit event", "calendar"),
			Kind: "edit", Title: "Edit event",
			Action: action, CancelURL: action, SubmitLabel: "Save",
			FormError: err.Error(), Form: form,
			Categories: cats, Recurrences: calendar.AllRecurrences(),
		})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/calendar/%d", id), http.StatusSeeOther)
}

// --- delete / restore / purge ------------------------------------------------

// softDeleteCalendar soft-deletes an event. With HX-Request it returns an
// empty fragment so HTMX swaps the card out of the list. Without, it redirects.
func (h *Handlers) softDeleteCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.calendarService.SoftDelete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(""))
		return
	}
	http.Redirect(w, r, "/calendar", http.StatusSeeOther)
}

// restoreCalendar restores a trashed event. With HX-Request it re-renders
// the card fragment. Without, it redirects to the show page.
func (h *Handlers) restoreCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.calendarService.Restore(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		h.renderCalendarCard(w, r, id)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/calendar/%d", id), http.StatusSeeOther)
}

// purgeCalendar permanently deletes an event and redirects to /calendar.
func (h *Handlers) purgeCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.calendarService.Purge(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(""))
		return
	}
	http.Redirect(w, r, "/calendar", http.StatusSeeOther)
}

// --- helpers -----------------------------------------------------------------

func (h *Handlers) renderCalendarCard(w http.ResponseWriter, r *http.Request, id int64) {
	ev, err := h.calendarService.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var catName, catSlug string
	if ev.Row.CategoryID.Valid {
		c, _ := h.queries.GetCategory(r.Context(), ev.Row.CategoryID.Int64)
		catName, catSlug = c.Name, c.Slug
	}
	vm := toCalendarVM(ev, catName, catSlug)
	w.Header().Set("Content-Type", "text/html")
	t := pages["calendar"]
	_ = t.ExecuteTemplate(w, "calendar-card", vm)
}

// calendarFormVM is the round-trippable calendar event form data.
type calendarFormVM struct {
	Title       string
	Description string
	StartDate   string // YYYY-MM-DD
	StartTime   string // HH:MM (empty for all-day)
	EndDate     string
	EndTime     string
	AllDay      bool
	Location    string
	Category    string // slug
	Notes       string
	Recurrence  string
	Tags        string // comma-separated
}

func (f calendarFormVM) tagSlice() []string {
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

// calendarFilterVM exposes the active filter to the list view header.
type calendarFilterVM struct {
	Active     bool
	From       string
	To         string
	Recurrence string
	Cat        string
	Tag        string
	Trashed    bool
	When       string // "today" | "upcoming" (from Today view links)
}

func calendarFormFromRequest(r *http.Request) calendarFormVM {
	if err := r.ParseForm(); err != nil {
		return calendarFormVM{}
	}
	return calendarFormVM{
		Title:       strings.TrimSpace(r.Form.Get("title")),
		Description: strings.TrimSpace(r.Form.Get("description")),
		StartDate:   strings.TrimSpace(r.Form.Get("start_date")),
		StartTime:   strings.TrimSpace(r.Form.Get("start_time")),
		EndDate:     strings.TrimSpace(r.Form.Get("end_date")),
		EndTime:     strings.TrimSpace(r.Form.Get("end_time")),
		AllDay:      r.Form.Get("all_day") == "1",
		Location:    strings.TrimSpace(r.Form.Get("location")),
		Category:    strings.TrimSpace(r.Form.Get("category")),
		Notes:       strings.TrimSpace(r.Form.Get("notes")),
		Recurrence:  strings.TrimSpace(r.Form.Get("recurrence")),
		Tags:        strings.TrimSpace(r.Form.Get("tags")),
	}
}

// composeStartEnd builds the start_at / end_at storage strings from form inputs.
// When all_day is true both values are date-only ("YYYY-MM-DD").
// When timed, start_at is "YYYY-MM-DDTHH:MM:SS" and end_at is empty when
// EndDate/EndTime are blank.
func composeStartEnd(f calendarFormVM) (startAt, endAt string) {
	if f.AllDay {
		startAt = f.StartDate
		if f.EndDate != "" {
			endAt = f.EndDate
		}
		return
	}
	// timed
	t := "00:00"
	if f.StartTime != "" {
		t = f.StartTime
	}
	startAt = f.StartDate + "T" + t + ":00"
	if f.EndDate != "" {
		et := "00:00"
		if f.EndTime != "" {
			et = f.EndTime
		}
		endAt = f.EndDate + "T" + et + ":00"
	}
	return
}

// splitStartAt splits a stored start_at value into date + time components.
// For "YYYY-MM-DDTHH:MM:SS" returns ("YYYY-MM-DD", "HH:MM").
// For "YYYY-MM-DD" returns ("YYYY-MM-DD", "").
func splitStartAt(s string) (date, timeStr string) {
	if len(s) >= 16 && s[10] == 'T' {
		return s[:10], s[11:16]
	}
	return s, ""
}
