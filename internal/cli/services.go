package cli

import "github.com/edwinupegui/arsenal/internal/resources"

// resourcesService is a tiny accessor so cobra command files don't sprinkle
// `resources.New(app.DB)` everywhere. Lives here instead of root.go so
// individual commands can stay focused on their own logic.
func resourcesService(app *AppContext) *resources.Service {
	return resources.New(app.DB)
}
