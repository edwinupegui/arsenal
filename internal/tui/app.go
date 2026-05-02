package tui

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
)

// viewState selects which sub-view the runtime renders.
type viewState int

const (
	viewList viewState = iota
	viewDetail
	viewConfirmDelete
)

// App is the root Bubble Tea model. It owns the DB handles, the list and
// detail sub-models, and the small bit of mode state that decides what
// `View()` returns.
type App struct {
	state viewState

	db      *sql.DB
	queries *store.Queries
	service *resources.Service

	keys    keyMap
	list    list.Model
	detail  detailModel
	confirm confirmModel

	width, height int

	// Whether the current list shows trashed rows (toggled with `t`).
	showTrashed bool

	statusMsg string
	statusErr error
}

// New builds an App backed by the given DB. The caller keeps ownership of the
// DB and is responsible for closing it after the program exits.
func New(db *sql.DB) App {
	keys := defaultKeys()

	delegate := list.NewDefaultDelegate()

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Arsenal"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.Styles.Title = titleStyle
	// Wire global keybindings into the list's help so `?` shows them.
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Detail, keys.Trash, keys.SoftDelete, keys.Star, keys.OpenURL}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.Detail, keys.Trash, keys.SoftDelete, keys.Restore,
			keys.Star, keys.OpenURL, keys.Refresh, keys.Quit,
		}
	}

	return App{
		state:   viewList,
		db:      db,
		queries: store.New(db),
		service: resources.New(db),
		keys:    keys,
		list:    l,
		detail:  newDetailModel(),
	}
}

// Run blocks until the user quits the TUI.
func Run(db *sql.DB) error {
	app := New(db)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// Init kicks off the first load of resources.
func (a App) Init() tea.Cmd {
	return loadResourcesCmd(a.queries, a.showTrashed)
}

// Update is the main message dispatch. Per Bubble Tea's contract it returns
// a NEW model and an optional command — never mutates `a` and returns it.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		// Reserve 2 rows for the status line + breathing room.
		listH := msg.Height - 2
		if listH < 5 {
			listH = 5
		}
		a.list.SetSize(msg.Width, listH)
		a.detail.SetSize(msg.Width, listH)
		return a, nil

	case resourcesLoadedMsg:
		if msg.err != nil {
			a.statusErr = msg.err
			return a, nil
		}
		a.list.SetItems(asItems(msg.items))
		a.statusErr = nil
		title := "Arsenal"
		if a.showTrashed {
			title = "Arsenal — Trash"
		}
		a.list.Title = title
		return a, nil

	case resourceMutatedMsg:
		if msg.err != nil {
			a.statusErr = msg.err
		} else {
			a.statusErr = nil
			a.statusMsg = msg.status
		}
		return a, loadResourcesCmd(a.queries, a.showTrashed)

	case errorMsg:
		a.statusErr = msg.err
		return a, nil
	}

	switch a.state {
	case viewDetail:
		return a.updateDetail(msg)
	case viewConfirmDelete:
		return a.updateConfirm(msg)
	default:
		return a.updateList(msg)
	}
}

// updateList handles key events while the list is the active view. Filter
// mode is delegated entirely to bubbles/list — when it is filtering, only
// the standard list keys (esc to clear filter, enter to confirm) work.
func (a App) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && a.list.FilterState() != list.Filtering {
		switch {
		case key.Matches(km, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(km, a.keys.Detail):
			if it, ok := a.selectedItem(); ok {
				a.detail.SetResource(it.r)
				a.state = viewDetail
				return a, nil
			}
		case key.Matches(km, a.keys.Trash):
			a.showTrashed = !a.showTrashed
			a.statusMsg = ""
			return a, loadResourcesCmd(a.queries, a.showTrashed)
		case key.Matches(km, a.keys.Refresh):
			return a, loadResourcesCmd(a.queries, a.showTrashed)
		case key.Matches(km, a.keys.Star):
			if it, ok := a.selectedItem(); ok {
				return a, toggleFavoriteCmd(a.service, it.r)
			}
		case key.Matches(km, a.keys.OpenURL):
			if it, ok := a.selectedItem(); ok {
				return a, openURLCmd(it.r.Resource.Url)
			}
		case key.Matches(km, a.keys.SoftDelete):
			if a.showTrashed {
				return a, nil
			}
			if it, ok := a.selectedItem(); ok {
				a.confirm = newConfirmModel(
					fmt.Sprintf("Move %q to trash?", it.r.Resource.Title),
					confirmActionDelete, it.r.Resource.ID, it.r.Resource.Title,
				)
				a.state = viewConfirmDelete
				return a, nil
			}
		case key.Matches(km, a.keys.Restore):
			if !a.showTrashed {
				return a, nil
			}
			if it, ok := a.selectedItem(); ok {
				return a, restoreCmd(a.service, it.r)
			}
		}
	}

	var cmd tea.Cmd
	a.list, cmd = a.list.Update(msg)
	return a, cmd
}

func (a App) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, a.keys.Quit), key.Matches(km, a.keys.Back):
			a.state = viewList
			return a, nil
		case key.Matches(km, a.keys.OpenURL):
			return a, openURLCmd(a.detail.res.Resource.Url)
		case key.Matches(km, a.keys.Star):
			return a, toggleFavoriteCmd(a.service, a.detail.res)
		}
	}
	var cmd tea.Cmd
	a.detail, cmd = a.detail.Update(msg)
	return a, cmd
}

func (a App) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			id := a.confirm.id
			a.state = viewList
			return a, softDeleteCmd(a.service, id, a.confirm.title)
		case "n", "N", "esc":
			a.state = viewList
			a.statusMsg = "cancelled"
			return a, nil
		}
	}
	return a, nil
}

// View composes the active view with a status line at the bottom.
func (a App) View() string {
	var body string
	switch a.state {
	case viewDetail:
		body = a.detail.View()
	case viewConfirmDelete:
		body = a.confirm.view(a.width, a.height)
	default:
		if a.showTrashed {
			body = trashBannerStyle.Render(" TRASH ") + "\n" + a.list.View()
		} else {
			body = a.list.View()
		}
	}
	return body + "\n" + a.statusLine()
}

func (a App) statusLine() string {
	if a.statusErr != nil {
		return statusErrorStyle.Render("error: ") + a.statusErr.Error()
	}
	if a.statusMsg != "" {
		return statusOKStyle.Render(a.statusMsg)
	}
	parts := []string{
		keyStyle.Render("enter") + " detail",
		keyStyle.Render("/") + " filter",
		keyStyle.Render("t") + " trash",
		keyStyle.Render("d") + " del",
		keyStyle.Render("*") + " fav",
		keyStyle.Render("o") + " open",
		keyStyle.Render("q") + " quit",
	}
	return mutedStyle.Render(strings.Join(parts, "  "))
}

func (a App) selectedItem() (resourceItem, bool) {
	cur := a.list.SelectedItem()
	if cur == nil {
		return resourceItem{}, false
	}
	it, ok := cur.(resourceItem)
	return it, ok
}

// --- commands ---------------------------------------------------------------

// loadResourcesCmd queries either the active or the trashed list and returns
// it as a resourcesLoadedMsg. Runs off the UI goroutine.
func loadResourcesCmd(q *store.Queries, trashed bool) tea.Cmd {
	return func() tea.Msg {
		filter := store.ListFilter{
			Trashed: trashed,
			Limit:   500,
		}
		items, err := q.ListResourcesFiltered(context.Background(), filter)
		return resourcesLoadedMsg{items: items, err: err}
	}
}

func toggleFavoriteCmd(svc *resources.Service, r store.ListedResource) tea.Cmd {
	return func() tea.Msg {
		next := r.Resource.Favorite != 1
		if err := svc.SetFavorite(context.Background(), r.Resource.ID, next); err != nil {
			return resourceMutatedMsg{err: err}
		}
		msg := "unstarred"
		if next {
			msg = "starred"
		}
		return resourceMutatedMsg{status: fmt.Sprintf("%s: %s", msg, r.Resource.Title)}
	}
}

func softDeleteCmd(svc *resources.Service, id int64, title string) tea.Cmd {
	return func() tea.Msg {
		if err := svc.SoftDelete(context.Background(), id); err != nil {
			return resourceMutatedMsg{err: err}
		}
		return resourceMutatedMsg{status: fmt.Sprintf("moved to trash: %s", title)}
	}
}

func restoreCmd(svc *resources.Service, r store.ListedResource) tea.Cmd {
	return func() tea.Msg {
		if err := svc.Restore(context.Background(), r.Resource.ID); err != nil {
			return resourceMutatedMsg{err: err}
		}
		return resourceMutatedMsg{status: fmt.Sprintf("restored: %s", r.Resource.Title)}
	}
}

// openURLCmd shells out to the platform-native URL opener so users can pop
// the highlighted resource straight into their browser.
func openURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Start(); err != nil {
			return errorMsg{err: fmt.Errorf("open browser: %w", err)}
		}
		return resourceMutatedMsg{status: "opened in browser"}
	}
}

// --- confirmation modal ------------------------------------------------------

type confirmAction int

const (
	confirmActionDelete confirmAction = iota
)

type confirmModel struct {
	prompt string
	action confirmAction
	id     int64
	title  string
}

func newConfirmModel(prompt string, action confirmAction, id int64, title string) confirmModel {
	return confirmModel{prompt: prompt, action: action, id: id, title: title}
}

func (c confirmModel) view(width, height int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorWarning).
		Padding(1, 2).
		Render(c.prompt + "\n\n" + mutedStyle.Render("[y] yes   [n] no / esc"))
	return lipgloss.Place(width, height-1, lipgloss.Center, lipgloss.Center, box)
}

