package tui

import "github.com/edwinupegui/arsenal/internal/store"

// Message types crossing the Update boundary. Concentrated here so the model
// in app.go can see every shape the runtime can hand it without scrolling.

// resourcesLoadedMsg arrives after a (re)load of the active list completes.
type resourcesLoadedMsg struct {
	items []store.ListedResource
	err   error
}

// resourceMutatedMsg fires after a mutation (delete, restore, star). The
// caller passes a human-readable status message; the runtime takes that as
// a cue to refresh the list.
type resourceMutatedMsg struct {
	status string
	err    error
}

// errorMsg is a generic bag for "something blew up, show it on the status line".
type errorMsg struct {
	err error
}
