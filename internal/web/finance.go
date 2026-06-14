package web

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/edwinupegui/arsenal/internal/finance"
	"github.com/edwinupegui/arsenal/internal/store"
)

// financePageData is the unified data envelope for every finance template
// render path. Using a single struct avoids "can't evaluate field X" template
// errors when the content block dispatches on .Kind / .Transaction.ID.
type financePageData struct {
	pageData
	// Dispatch keys (always present so the template can evaluate them safely)
	Kind string
	// List view
	Transactions []financeVM
	Filter       financeFilterVM
	// Detail view
	Transaction financeVM
	// Form view
	Title       string
	Action      string
	CancelURL   string
	SubmitLabel string
	FormError   string
	Form        financeFormVM
	Categories  []store.Category
	Kinds       []finance.Kind
	Recurrences []finance.Recurrence
}

// financeRoutes registers all /finance/* routes on r. Called from server.go
// after todoRoutes so the router builds a coherent prefix tree.
func (h *Handlers) financeRoutes(r chi.Router) {
	r.Get("/finance", h.listFinance)
	r.Get("/finance/new", h.newFinanceForm)
	r.Post("/finance", h.createFinance)
	r.Get("/finance/{id}", h.showFinance)
	r.Get("/finance/{id}/edit", h.editFinanceForm)
	r.Post("/finance/{id}", h.updateFinance)
	r.Post("/finance/{id}/delete", h.softDeleteFinance)
	r.Post("/finance/{id}/restore", h.restoreFinance)
	r.Post("/finance/{id}/purge", h.purgeFinance)
}

// --- list / new / create ----------------------------------------------------

func (h *Handlers) listFinance(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var fromPtr, toPtr, kindPtr *string
	if v := q.Get("from"); v != "" {
		fromPtr = &v
	}
	if v := q.Get("to"); v != "" {
		toPtr = &v
	}
	if v := q.Get("kind"); v != "" {
		kindPtr = &v
	}

	f := finance.Filter{
		From:         fromPtr,
		To:           toPtr,
		Kind:         kindPtr,
		CategorySlug: q.Get("cat"),
		TagName:      q.Get("tag"),
		Trashed:      q.Get("trashed") == "1",
		Limit:        500,
	}

	txns, err := h.financeService.List(r.Context(), f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filter := financeFilterVM{
		From:    q.Get("from"),
		To:      q.Get("to"),
		Kind:    q.Get("kind"),
		Cat:     q.Get("cat"),
		Tag:     q.Get("tag"),
		Trashed: f.Trashed,
	}
	filter.Active = filter.From != "" || filter.To != "" ||
		filter.Kind != "" || filter.Cat != "" ||
		filter.Tag != "" || filter.Trashed

	data := financePageData{
		pageData:     h.commonPage(r, "Finance", "finance"),
		Transactions: toFinanceVMs(txns),
		Filter:       filter,
		Title:        "Finance",
	}
	render(w, "finance", data)
}

func (h *Handlers) newFinanceForm(w http.ResponseWriter, r *http.Request) {
	cats, _ := h.queries.ListCategories(r.Context())
	data := financePageData{
		pageData:    h.commonPage(r, "New transaction", "finance"),
		Kind:        "new",
		Title:       "New transaction",
		Action:      "/finance",
		CancelURL:   "/finance",
		SubmitLabel: "Create",
		Form:        financeFormVM{Kind: string(finance.KindExpense), Recurrence: string(finance.RecurrenceNone)},
		Categories:  cats,
		Kinds:       finance.AllKinds(),
		Recurrences: finance.AllRecurrences(),
	}
	render(w, "finance", data)
}

func (h *Handlers) createFinance(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	form := financeFormFromRequest(r)
	cats, _ := h.queries.ListCategories(r.Context())
	catID, err := h.resolveCategory(r, form.Category)
	if err != nil {
		render(w, "finance", financePageData{
			pageData: h.commonPage(r, "New transaction", "finance"),
			Kind: "new", Title: "New transaction",
			Action: "/finance", CancelURL: "/finance", SubmitLabel: "Create",
			FormError: err.Error(), Form: form,
			Categories: cats, Kinds: finance.AllKinds(), Recurrences: finance.AllRecurrences(),
		})
		return
	}
	amount, err := strconv.ParseFloat(form.Amount, 64)
	if err != nil || amount <= 0 {
		render(w, "finance", financePageData{
			pageData: h.commonPage(r, "New transaction", "finance"),
			Kind: "new", Title: "New transaction",
			Action: "/finance", CancelURL: "/finance", SubmitLabel: "Create",
			FormError: "amount must be a positive number", Form: form,
			Categories: cats, Kinds: finance.AllKinds(), Recurrences: finance.AllRecurrences(),
		})
		return
	}
	in := finance.CreateInput{
		Date:       form.Date,
		Amount:     amount,
		Kind:       finance.Kind(form.Kind),
		Account:    form.Account,
		CategoryID: catID,
		Notes:      form.Notes,
		Recurrence: finance.Recurrence(form.Recurrence),
		Tags:       form.tagSlice(),
	}
	if _, err := h.financeService.Create(r.Context(), in); err != nil {
		render(w, "finance", financePageData{
			pageData: h.commonPage(r, "New transaction", "finance"),
			Kind: "new", Title: "New transaction",
			Action: "/finance", CancelURL: "/finance", SubmitLabel: "Create",
			FormError: err.Error(), Form: form,
			Categories: cats, Kinds: finance.AllKinds(), Recurrences: finance.AllRecurrences(),
		})
		return
	}
	http.Redirect(w, r, "/finance", http.StatusSeeOther)
}

// --- show / edit / update ---------------------------------------------------

func (h *Handlers) showFinance(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	txn, err := h.financeService.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var catName, catSlug string
	if txn.Row.CategoryID.Valid {
		c, cerr := h.queries.GetCategory(r.Context(), txn.Row.CategoryID.Int64)
		if cerr == nil {
			catName, catSlug = c.Name, c.Slug
		}
	}
	vm := toFinanceVM(txn, catName, catSlug)
	data := financePageData{
		pageData:    h.commonPage(r, fmt.Sprintf("%s %.2f", vm.Kind, vm.Amount), "finance"),
		Transaction: vm,
	}
	render(w, "finance", data)
}

func (h *Handlers) editFinanceForm(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	txn, err := h.financeService.Get(r.Context(), id)
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
	if txn.Row.CategoryID.Valid {
		c, cerr := h.queries.GetCategory(r.Context(), txn.Row.CategoryID.Int64)
		if cerr == nil {
			catSlug = c.Slug
		}
	}
	notes := ""
	if txn.Row.Notes.Valid {
		notes = txn.Row.Notes.String
	}
	form := financeFormVM{
		Date:       txn.Row.Date,
		Amount:     strconv.FormatFloat(txn.Row.Amount, 'f', 2, 64),
		Kind:       txn.Row.Kind,
		Account:    txn.Row.Account,
		Category:   catSlug,
		Notes:      notes,
		Recurrence: txn.Row.Recurrence,
		Tags:       strings.Join(txn.Tags, ", "),
	}
	action := fmt.Sprintf("/finance/%d", id)
	data := financePageData{
		pageData:    h.commonPage(r, "Edit transaction", "finance"),
		Kind:        "edit",
		Title:       "Edit transaction",
		Action:      action,
		CancelURL:   action,
		SubmitLabel: "Save",
		Form:        form,
		Categories:  cats,
		Kinds:       finance.AllKinds(),
		Recurrences: finance.AllRecurrences(),
	}
	render(w, "finance", data)
}

func (h *Handlers) updateFinance(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	form := financeFormFromRequest(r)
	cats, _ := h.queries.ListCategories(r.Context())
	action := fmt.Sprintf("/finance/%d", id)
	catID, err := h.resolveCategory(r, form.Category)
	if err != nil {
		render(w, "finance", financePageData{
			pageData: h.commonPage(r, "Edit transaction", "finance"),
			Kind: "edit", Title: "Edit transaction",
			Action: action, CancelURL: action, SubmitLabel: "Save",
			FormError: err.Error(), Form: form,
			Categories: cats, Kinds: finance.AllKinds(), Recurrences: finance.AllRecurrences(),
		})
		return
	}
	amount, err := strconv.ParseFloat(form.Amount, 64)
	if err != nil || amount <= 0 {
		render(w, "finance", financePageData{
			pageData: h.commonPage(r, "Edit transaction", "finance"),
			Kind: "edit", Title: "Edit transaction",
			Action: action, CancelURL: action, SubmitLabel: "Save",
			FormError: "amount must be a positive number", Form: form,
			Categories: cats, Kinds: finance.AllKinds(), Recurrences: finance.AllRecurrences(),
		})
		return
	}
	in := finance.CreateInput{
		Date:       form.Date,
		Amount:     amount,
		Kind:       finance.Kind(form.Kind),
		Account:    form.Account,
		CategoryID: catID,
		Notes:      form.Notes,
		Recurrence: finance.Recurrence(form.Recurrence),
		Tags:       form.tagSlice(),
	}
	if _, err := h.financeService.Update(r.Context(), id, in); err != nil {
		render(w, "finance", financePageData{
			pageData: h.commonPage(r, "Edit transaction", "finance"),
			Kind: "edit", Title: "Edit transaction",
			Action: action, CancelURL: action, SubmitLabel: "Save",
			FormError: err.Error(), Form: form,
			Categories: cats, Kinds: finance.AllKinds(), Recurrences: finance.AllRecurrences(),
		})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/finance/%d", id), http.StatusSeeOther)
}

// --- delete / restore / purge -----------------------------------------------

// softDeleteFinance soft-deletes a transaction. With HX-Request it returns an
// empty fragment so HTMX swaps the card out of the list. Without, it redirects.
func (h *Handlers) softDeleteFinance(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.financeService.SoftDelete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(""))
		return
	}
	http.Redirect(w, r, "/finance", http.StatusSeeOther)
}

// restoreFinance restores a trashed transaction. With HX-Request it re-renders
// the card fragment. Without, it redirects to the show page.
func (h *Handlers) restoreFinance(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.financeService.Restore(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		h.renderFinanceCard(w, r, id)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/finance/%d", id), http.StatusSeeOther)
}

// purgeFinance permanently deletes a transaction and redirects to /finance.
func (h *Handlers) purgeFinance(w http.ResponseWriter, r *http.Request) {
	id, err := chiID(r)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.financeService.Purge(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(""))
		return
	}
	http.Redirect(w, r, "/finance", http.StatusSeeOther)
}

// --- helpers ----------------------------------------------------------------

func (h *Handlers) renderFinanceCard(w http.ResponseWriter, r *http.Request, id int64) {
	txn, err := h.financeService.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var catName, catSlug string
	if txn.Row.CategoryID.Valid {
		c, _ := h.queries.GetCategory(r.Context(), txn.Row.CategoryID.Int64)
		catName, catSlug = c.Name, c.Slug
	}
	vm := toFinanceVM(txn, catName, catSlug)
	w.Header().Set("Content-Type", "text/html")
	t := pages["finance"]
	_ = t.ExecuteTemplate(w, "finance-card", vm)
}

// financeFormVM is the round-trippable finance form data.
type financeFormVM struct {
	Date       string
	Amount     string // string for form round-trip
	Kind       string
	Account    string
	Category   string // slug
	Notes      string
	Recurrence string
	Tags       string // comma-separated
}

func (f financeFormVM) tagSlice() []string {
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

// financeFilterVM exposes the active filter to the list view header.
type financeFilterVM struct {
	Active  bool
	From    string
	To      string
	Kind    string
	Cat     string
	Tag     string
	Trashed bool
}

func financeFormFromRequest(r *http.Request) financeFormVM {
	if err := r.ParseForm(); err != nil {
		return financeFormVM{}
	}
	return financeFormVM{
		Date:       strings.TrimSpace(r.Form.Get("date")),
		Amount:     strings.TrimSpace(r.Form.Get("amount")),
		Kind:       strings.TrimSpace(r.Form.Get("kind")),
		Account:    strings.TrimSpace(r.Form.Get("account")),
		Category:   strings.TrimSpace(r.Form.Get("category")),
		Notes:      strings.TrimSpace(r.Form.Get("notes")),
		Recurrence: strings.TrimSpace(r.Form.Get("recurrence")),
		Tags:       strings.TrimSpace(r.Form.Get("tags")),
	}
}
