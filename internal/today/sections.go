package today

// Section is a named group of items within the Today view.
type Section struct {
	Key        string // "overdue", "due-today", "upcoming", "recent"
	Title      string // "Overdue", "Due Today", "Upcoming", "Recent Resources"
	Items      []Item
	ShowAllURL string // empty when Items ≤ 5; otherwise link to domain list
	IsEmpty    bool   // true when provider returned 0 items → omitted from render
}

// Item is the cross-domain common shape rendered by TUI and web.
type Item struct {
	Domain   string   // "todos" | "resources"
	ID       int64
	Title    string
	Subtitle string   // due date, resource type, etc.
	Priority string   // "high" | "med" | "low" | "" (resources have no priority)
	Tags     []string
	URL      string   // "/todos/42" or "/resources/7" — empty for TUI-only items
}

// sectionOrder defines the fixed ordering for v3.0/v3.x. Sections not in
// this map are appended at the end in their provider-defined order.
var sectionOrder = map[string]int{
	"overdue":             1,
	"due-today":           2,
	"upcoming":            3,
	"recent":              4,
	"this-month-spending": 5,
	"recent-transactions": 6,
}
