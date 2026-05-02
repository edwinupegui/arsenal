// Package web wires the local HTTP UI for `arsenal web`. It owns the chi
// router, the html/template-based renderer and the handlers that translate
// form input into resources.Service calls.
//
// All assets (templates, CSS, vendored htmx) are embedded into the binary so
// the user only needs the `arsenal` executable to run the web UI.
package web

import "embed"

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS
