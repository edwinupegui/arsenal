package web

import (
	"html/template"
	"net/http"

	"github.com/edwinupegui/arsenal/internal/today"
)

// todayPage renders the Today view with aggregated sections from all providers.
func (h *Handlers) todayPage(w http.ResponseWriter, r *http.Request) {
	sections, _ := h.todayService.Build(r.Context())

	data := struct {
		pageData
		Sections     []today.Section
		EmptyMessage template.HTML
	}{
		pageData: h.commonPage(r, "Today", "today"),
		Sections: sections,
	}
	if today.IsEmptyPage(sections) {
		data.EmptyMessage = template.HTML(today.RenderEmptyState("web"))
	}
	render(w, "today", data)
}
