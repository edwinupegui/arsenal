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
	}
}
