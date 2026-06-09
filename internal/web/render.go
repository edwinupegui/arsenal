package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

// pageNames lists the renderable templates. Each is a content fragment
// composed against templates/layout.html via Go's template inheritance.
var pageNames = []string{
	"list",
	"detail",
	"form",
	"categories",
	"tags",
	"todos",
}

// pages is populated at package init with the parsed template tree per page.
// Cached so each request just walks straight to ExecuteTemplate.
var pages = map[string]*template.Template{}

func init() {
	tplFS, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		panic(fmt.Errorf("web: sub templates fs: %w", err))
	}
	for _, name := range pageNames {
		t := template.New("layout").Funcs(funcMap)
		t = template.Must(t.ParseFS(tplFS, "layout.html", name+".html"))
		pages[name] = t
	}
}

// funcMap adds tiny helpers templates need that aren't in the stdlib defaults.
var funcMap = template.FuncMap{}

// render writes a parsed page to w using "layout" as the entry point.
// The data map MUST include a Title and a Nav key to keep the header sane.
func render(w http.ResponseWriter, page string, data any) {
	t, ok := pages[page]
	if !ok {
		http.Error(w, "unknown template: "+page, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
