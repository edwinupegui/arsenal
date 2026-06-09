package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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

func (h *Handlers) listTodos(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) newTodoForm(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) createTodo(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) showTodo(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) editTodoForm(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) updateTodo(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) markTodoDone(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) markTodoOpen(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) softDeleteTodo(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) restoreTodo(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handlers) purgeTodo(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
