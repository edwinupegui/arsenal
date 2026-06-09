package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap groups the global keybindings the app reacts to.
// View-local bindings (search input, list nav) come from the embedded
// bubbles components and are documented in their own help output.
type keyMap struct {
	Quit       key.Binding
	Back       key.Binding
	Detail     key.Binding
	Trash      key.Binding
	Star       key.Binding
	SoftDelete key.Binding
	Restore    key.Binding
	Refresh    key.Binding
	Help       key.Binding
	OpenURL    key.Binding
	Search     key.Binding
	ClearList  key.Binding
	Tab        key.Binding
	ShiftTab   key.Binding
	JumpToday     key.Binding
	JumpResources key.Binding
	JumpTodos     key.Binding
	JumpFinance   key.Binding
	JumpCalendar  key.Binding
	MarkDone   key.Binding
	MarkOpen   key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Detail:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open detail")),
		Trash:      key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle trash view")),
		Star:       key.NewBinding(key.WithKeys("*"), key.WithHelp("*", "toggle favorite")),
		SoftDelete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "move to trash")),
		Restore:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restore from trash")),
		Refresh:    key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		OpenURL:    key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open url in browser")),
		Search:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "FTS5 search")),
		ClearList:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clear search/filter")),
		Tab:        key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next area")),
		ShiftTab:   key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev area")),
		JumpToday:     key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "Today")),
		JumpResources: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "Resources")),
		JumpTodos:     key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "Todos")),
		JumpFinance:   key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "Finance")),
		JumpCalendar:  key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "Calendar")),
		MarkDone: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "mark done")),
		MarkOpen: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "mark open")),
	}
}
