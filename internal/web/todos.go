package web

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/todos"
)

func (h *Handlers) todoRoutes(r chi.Router) {
	r.Get("/todos", h.listTodos)
	r.Get("/todos/new", h.newTodoForm)
	r.Post("/todos", h.createTodo)
	r.Get("/todos/{id}", h.showTodo)
	r.Get("/todos/{id}/edit", h.editTodoForm)
	r.Post("/todos/{id}", h.updateTodo)
	r.Post("/todos/{id}/done", h.markTodoDone)
	r.Post("/todos/{id}/open", h.markTodoOpen)
	r.Post("/todos/{id}/delete", h.softDeleteTodo)
	r.Post("/todos/{id}/restore", h.restoreTodo)
	r.Post("/todos/{id}/purge", h.purgeTodo)
}

// --- list / new / create ----------------------------------------------------

func (h *Handlers) listTodos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := todos.ListFilter{
		Status:       todos.Status(q.Get("status")),
		Priority:     todos.Priority(q.Get("priority")),
		CategorySlug: q.Get("cat"),
		TagName:      q.Get("tag"),
		OnlyOverdue:  q.Get("overdue") == "1" || q.Get("overdue") == "true",
		DueBefore:    q.Get("due_before"),
		Trashed:      q.Get("trashed") == "1",
		Limit:        50,
	}

	// Support due=today and due=upcoming shortcuts from the Today view.
	loc, err := today.UserLocation(r.Context(), h.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := h.now().In(loc)
	todayStr := now.Format("2006-01-02")
	switch q.Get("due") {
	case "today":
		filter.DueBefore = now.AddDate(0, 0, 1).Format("2006-01-02")
	case "upcoming":
		filter.DueBefore = now.AddDate(0, 0, 7).Format("2006-01-02")
	}
	if offset, err := strconv.Atoi(q.Get("offset")); err == nil && offset > 0 {
		filter.Offset = offset
	}
	rows, err := h.todoService.List(r.Context(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vms := make([]todoVM, 0, len(rows))
	for _, t := range rows {
		var catName, catSlug string
		if t.Row.CategoryID != nil {
			c, _ := h.queries.GetCategory(r.Context(), *t.Row.CategoryID)
			catName, catSlug = c.Name, c.Slug
		}
		vm := toTodoVM(t.Row, t.Tags, catName, catSlug)
		vm.Overdue = isOverdue(vm.DueDate, vm.Status, todayStr)
		vms = append(vms, vm)
	}
	data := struct {
		pageData
		Todos  []todoVM
		Filter todoFilterVM
	}{
		pageData: h.commonPage(r, "Todos", "todos"),
		Todos:    vms,
		Filter: todoFilterVM{
			Status:    string(filter.Status),
			Priority:  string(filter.Priority),
			Overdue:   filter.OnlyOverdue,
			Cat:       filter.CategorySlug,
			Tag:       filter.TagName,
			Trashed:   filter.Trashed,
			DueBefore: filter.DueBefore,
		},
	}
	data.Filter.Active = data.Filter.Status != "" || data.Filter.Priority != "" ||
		data.Filter.Overdue || data.Filter.Cat != "" || data.Filter.Tag != "" ||
		data.Filter.Trashed || data.Filter.DueBefore != ""
	render(w, "todos", data)
}

func (h *Handlers) newTodoForm(w http.ResponseWriter, r *http.Request) {
	cats, _ := h.queries.ListCategories(r.Context())
	data := struct {
		pageData
		Title      string
		Action     string
		CancelURL  string
		SubmitLabel string
		FormError  string
		Form       todoFormVM
		Categories []store.Category
		Priorities []todos.Priority
		Recurrences []todos.Recurrence
		Kind       string
	}{
		pageData:    h.commonPage(r, "New todo", "todos"),
		Title:       "New todo",
		Action:      "/todos",
		CancelURL:   "/todos",
		SubmitLabel: "Create",
		Form:        todoFormVM{Priority: string(todos.PriorityMed), Recurrence: string(todos.RecurrenceNone)},
		Categories:  cats,
		Priorities:  todos.AllPriorities(),
		Recurrences: todos.AllRecurrences(),
		Kind:        "new",
	}
	render(w, "todos", data)
}

func (h *Handlers) createTodo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	form := todoFormFromRequest(r)
	catID, err := h.resolveCategory(r, form.Category)
	if err != nil {
		h.renderTodoForm(w, r, form, "New todo", "/todos", "/todos", "Create", err.Error(), "new")
		return
	}
	in := todos.CreateInput{
		Title:       form.Title,
		Description: form.Description,
		Priority:    todos.Priority(form.Priority),
		DueDate:     strPtr(form.DueDate),
		CategoryID:  catID,
		Notes:       form.Notes,
		Recurrence:  todos.Recurrence(form.Recurrence),
		Tags:        form.tagSlice(),
	}
	if _, err := h.todoService.Create(r.Context(), in); err != nil {
		h.renderTodoForm(w, r, form, "New todo", "/todos", "/todos", "Create", err.Error(), "new")
		return
	}
	http.Redirect(w, r, "/todos", http.StatusSeeOther)
}

// --- show / edit / update ----------------------------------------------------

func (h *Handlers) showTodo(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	todo, err := h.todoService.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	loc, err := today.UserLocation(r.Context(), h.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	todayStr := h.now().In(loc).Format("2006-01-02")
	var catName, catSlug string
	if todo.Row.CategoryID != nil {
		c, _ := h.queries.GetCategory(r.Context(), *todo.Row.CategoryID)
		catName, catSlug = c.Name, c.Slug
	}
	vm := toTodoVM(todo.Row, todo.Tags, catName, catSlug)
	vm.Overdue = isOverdue(vm.DueDate, vm.Status, todayStr)
	data := struct {
		pageData
		Todo todoVM
	}{
		pageData: h.commonPage(r, vm.Title, "todos"),
		Todo:     vm,
	}
	render(w, "todos", data)
}

func (h *Handlers) editTodoForm(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	todo, err := h.todoService.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var catSlug string
	if todo.Row.CategoryID != nil {
		c, _ := h.queries.GetCategory(r.Context(), *todo.Row.CategoryID)
		catSlug = c.Slug
	}
	form := todoFormVM{
		Title:       todo.Row.Title,
		Description: nullStrPtr(todo.Row.Description),
		Priority:    todo.Row.Priority,
		DueDate:     nullStrPtr(todo.Row.DueDate),
		Category:    catSlug,
		Notes:       nullStrPtr(todo.Row.Notes),
		Recurrence:  todo.Row.Recurrence,
		Tags:        strings.Join(todo.Tags, ", "),
	}
	action := fmt.Sprintf("/todos/%d", id)
	h.renderTodoForm(w, r, form, "Edit todo", action, action, "Save", "", "edit")
}

func (h *Handlers) updateTodo(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	form := todoFormFromRequest(r)
	catID, err := h.resolveCategory(r, form.Category)
	if err != nil {
		h.renderTodoForm(w, r, form, "Edit todo",
			fmt.Sprintf("/todos/%d", id), fmt.Sprintf("/todos/%d", id), "Save", err.Error(), "edit")
		return
	}
	in := todos.CreateInput{
		Title:       form.Title,
		Description: form.Description,
		Priority:    todos.Priority(form.Priority),
		DueDate:     strPtr(form.DueDate),
		CategoryID:  catID,
		Notes:       form.Notes,
		Recurrence:  todos.Recurrence(form.Recurrence),
		Tags:        form.tagSlice(),
	}
	if _, err := h.todoService.Update(r.Context(), id, in); err != nil {
		h.renderTodoForm(w, r, form, "Edit todo",
			fmt.Sprintf("/todos/%d", id), fmt.Sprintf("/todos/%d", id), "Save", err.Error(), "edit")
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/todos/%d", id), http.StatusSeeOther)
}

// --- HTMX status transitions ------------------------------------------------

func (h *Handlers) markTodoDone(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.todoService.MarkDone(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderTodoCard(w, r, id)
}

func (h *Handlers) markTodoOpen(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.todoService.MarkOpen(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderTodoCard(w, r, id)
}

// --- delete / restore / purge ------------------------------------------------

func (h *Handlers) softDeleteTodo(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.todoService.SoftDelete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html")
		pd := h.commonPage(r, "Todos", "todos")
		t := pages["todos"]
		_ = t.ExecuteTemplate(w, "sidebar-oob", pd)
		return
	}
	http.Redirect(w, r, "/todos", http.StatusSeeOther)
}

func (h *Handlers) restoreTodo(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.todoService.Restore(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		h.renderTodoCard(w, r, id)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/todos/%d", id), http.StatusSeeOther)
}

func (h *Handlers) purgeTodo(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.todoService.Purge(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(""))
		return
	}
	http.Redirect(w, r, "/todos", http.StatusSeeOther)
}

// --- helpers ----------------------------------------------------------------

func (h *Handlers) renderTodoCard(w http.ResponseWriter, r *http.Request, id int64) {
	todo, err := h.todoService.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	loc, err := today.UserLocation(r.Context(), h.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	todayStr := h.now().In(loc).Format("2006-01-02")
	var catName, catSlug string
	if todo.Row.CategoryID != nil {
		c, _ := h.queries.GetCategory(r.Context(), *todo.Row.CategoryID)
		catName, catSlug = c.Name, c.Slug
	}
	vm := toTodoVM(todo.Row, todo.Tags, catName, catSlug)
	vm.Overdue = isOverdue(vm.DueDate, vm.Status, todayStr)
	w.Header().Set("Content-Type", "text/html")
	t := pages["todos"]
	_ = t.ExecuteTemplate(w, "todo-card", vm)
	pd := h.commonPage(r, "Todos", "todos")
	_ = t.ExecuteTemplate(w, "sidebar-oob", pd)
}

func (h *Handlers) renderTodoForm(w http.ResponseWriter, r *http.Request, form todoFormVM,
	pageTitle, action, cancel, submit, errMsg, kind string) {
	cats, _ := h.queries.ListCategories(r.Context())
	data := struct {
		pageData
		Title       string
		Action      string
		CancelURL   string
		SubmitLabel string
		FormError   string
		Form        todoFormVM
		Categories  []store.Category
		Priorities  []todos.Priority
		Recurrences []todos.Recurrence
		Kind        string
	}{
		pageData:    h.commonPage(r, pageTitle, "todos"),
		Title:       pageTitle,
		Action:      action,
		CancelURL:   cancel,
		SubmitLabel: submit,
		FormError:   errMsg,
		Form:        form,
		Categories:  cats,
		Priorities:  todos.AllPriorities(),
		Recurrences: todos.AllRecurrences(),
		Kind:        kind,
	}
	render(w, "todos", data)
}

func todoFormFromRequest(r *http.Request) todoFormVM {
	if err := r.ParseForm(); err != nil {
		return todoFormVM{}
	}
	return todoFormVM{
		Title:       strings.TrimSpace(r.Form.Get("title")),
		Description: strings.TrimSpace(r.Form.Get("description")),
		Priority:    strings.TrimSpace(r.Form.Get("priority")),
		DueDate:     strings.TrimSpace(r.Form.Get("due_date")),
		Category:    strings.TrimSpace(r.Form.Get("category")),
		Notes:       strings.TrimSpace(r.Form.Get("notes")),
		Recurrence:  strings.TrimSpace(r.Form.Get("recurrence")),
		Tags:        strings.TrimSpace(r.Form.Get("tags")),
	}
}

func isOverdue(dueDate, status, todayStr string) bool {
	return status == "open" && dueDate != "" && dueDate < todayStr
}

func strPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

// todoFormVM is the round-trippable todo form data.
type todoFormVM struct {
	Title       string
	Description string
	Priority    string
	DueDate     string
	Category    string
	Notes       string
	Recurrence  string
	Tags        string
}

func (f todoFormVM) tagSlice() []string {
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

// todoFilterVM exposes the active todo filter to the list view.
type todoFilterVM struct {
	Active bool
	Status string
	Priority string
	Overdue bool
	Cat string
	Tag string
	Trashed bool
	DueBefore string
}
