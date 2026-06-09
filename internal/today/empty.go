package today

import "fmt"

// IsEmptyPage returns true when every section has zero items.
func IsEmptyPage(sections []Section) bool {
	for _, s := range sections {
		if len(s.Items) > 0 {
			return false
		}
	}
	return true
}

// RenderEmptyState returns a friendly empty-state message for the given surface.
// Supported surfaces: "tui", "web".
func RenderEmptyState(surface string) string {
	msg := "Nothing on your plate today."
	switch surface {
	case "tui":
		return fmt.Sprintf("%s\n\nn  add a todo   2  browse resources", msg)
	case "web":
		return fmt.Sprintf("%s <a href=\"/todos/new\">Add a todo</a> <a href=\"/resources\">Browse resources</a>", msg)
	default:
		return msg
	}
}
