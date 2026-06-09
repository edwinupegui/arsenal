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
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/today/providers"
	"github.com/edwinupegui/arsenal/internal/todos"
)

// areaID selects which functional area is active.
type areaID int

const (
	areaToday areaID = iota
	areaResources
	areaTodos
	areaFinance
	areaCalendar
)

var areaNames = map[areaID]string{
	areaToday:     "Today",
	areaResources: "Resources",
	areaTodos:     "Todos",
	areaFinance:   "Finance",
	areaCalendar:  "Calendar",
}

// viewState selects which sub-view the runtime renders.
type viewState int

const (
	viewList viewState = iota
	viewDetail
	viewConfirmDelete
	viewSearchInput
)

// App is the root Bubble Tea model. It owns the DB handles, the list and
// detail sub-models, and the small bit of mode state that decides what
// `View()` returns.
type App struct {
	state viewState

	currentArea areaID

	db      *sql.DB
	queries *store.Queries
	service *resources.Service

	keys         keyMap
	list         list.Model
	detail       detailModel
	confirm      confirmModel
	searchIn     textinput.Model
	searchActive string // "" → showing default list; non-empty → search results

	width, height int

	// Whether the current list shows trashed rows (toggled with `t`).
	showTrashed bool

	statusMsg string
	statusErr error

	// Today area state
	todayService *today.Service
	todayModel   todayModel

	// Todos area state
	todosService     *todos.Service
	todoList         list.Model
	todoDetail       todoDetailModel
	todoConfirm      confirmModel
	todoSearchIn     textinput.Model
	todoSearchActive string
	todoShowTrashed  bool
	todoState        todoViewState
	todoMutated      todoMutatedMsg
}

type todoViewState int

const (
	todoStateList todoViewState = iota
	todoStateDetail
	todoStateSearchInput
	todoStateConfirmDelete
)

type todoMutatedMsg struct {
	status string
	err    error
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
		return []key.Binding{keys.Detail, keys.Search, keys.Trash, keys.SoftDelete, keys.Star, keys.OpenURL}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.Detail, keys.Search, keys.ClearList,
			keys.Trash, keys.SoftDelete, keys.Restore,
			keys.Star, keys.OpenURL, keys.Refresh, keys.Quit,
		}
	}

	si := textinput.New()
	si.Placeholder = "search title, description, notes, tags…"
	si.Prompt = "/ "
	si.CharLimit = 200

	todoList := list.New(nil, delegate, 0, 0)
	todoList.Title = "Todos"
	todoList.SetShowStatusBar(true)
	todoList.SetFilteringEnabled(true)
	todoList.SetShowHelp(true)
	todoList.Styles.Title = titleStyle

	todoSearch := textinput.New()
	todoSearch.Placeholder = "search todos…"
	todoSearch.Prompt = "/ "
	todoSearch.CharLimit = 200

	initialArea := areaToday
	if db != nil {
		cs := configstore.New(db)
		if v, err := cs.GetDefault(context.Background(), config.KeyLandingSurface); err == nil {
			switch v {
			case "resources":
				initialArea = areaResources
			default:
				initialArea = areaToday
			}
		}
	}

	todaySvc := today.New(db)
	todaySvc.Register(providers.NewTodosProvider(db))
	todaySvc.Register(providers.NewResourcesProvider(db))

	return App{
		state:       viewList,
		currentArea: initialArea,
		db:          db,
		queries:     store.New(db),
		service:     resources.New(db),
		keys:        keys,
		list:        l,
		detail:      newDetailModel(),
		searchIn:    si,
		// Today area initialized here
		todayService: todaySvc,
		// Todos area initialized here
		todosService: todos.New(db),
		todoList:     todoList,
		todoDetail:   newTodoDetailModel(),
		todoSearchIn: todoSearch,
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
		a.todoList.SetSize(msg.Width, listH)
		a.todoDetail.SetSize(msg.Width, listH)
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
		a.searchActive = ""
		return a, nil

	case searchResultsMsg:
		if msg.err != nil {
			a.statusErr = msg.err
			return a, nil
		}
		a.list.SetItems(asItems(msg.items))
		a.list.Title = fmt.Sprintf("Arsenal — search: %s", msg.query)
		a.searchActive = msg.query
		a.statusErr = nil
		a.statusMsg = fmt.Sprintf("%d match", len(msg.items))
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

	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, a.keys.Tab):
			a.currentArea = (a.currentArea + 1) % 5
			return a, a.loadCurrentAreaCmd()
		case key.Matches(km, a.keys.ShiftTab):
			a.currentArea = (a.currentArea + 4) % 5
			return a, a.loadCurrentAreaCmd()
		case key.Matches(km, a.keys.JumpToday):
			a.currentArea = areaToday
			return a, a.loadCurrentAreaCmd()
		case key.Matches(km, a.keys.JumpResources):
			a.currentArea = areaResources
			return a, a.loadCurrentAreaCmd()
		case key.Matches(km, a.keys.JumpTodos):
			a.currentArea = areaTodos
			return a, a.loadCurrentAreaCmd()
		case key.Matches(km, a.keys.JumpFinance):
			a.currentArea = areaFinance
			return a, a.loadCurrentAreaCmd()
		case key.Matches(km, a.keys.JumpCalendar):
			a.currentArea = areaCalendar
			return a, a.loadCurrentAreaCmd()
		}
	}

	switch a.currentArea {
	case areaToday:
		return a.updateToday(msg)
	case areaTodos:
		return a.updateTodos(msg)
	case areaResources:
		switch a.state {
		case viewDetail:
			return a.updateDetail(msg)
		case viewConfirmDelete:
			return a.updateConfirm(msg)
		case viewSearchInput:
			return a.updateSearchInput(msg)
		default:
			return a.updateList(msg)
		}
	default:
		return a, nil
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
		case key.Matches(km, a.keys.Search):
			a.searchIn.SetValue("")
			a.searchIn.Focus()
			a.state = viewSearchInput
			a.statusMsg = ""
			return a, textinput.Blink
		case key.Matches(km, a.keys.ClearList):
			a.statusMsg = ""
			a.searchActive = ""
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

// updateSearchInput handles the FTS5 search overlay. Enter dispatches the
// search; esc cancels and reloads the default list.
func (a App) updateSearchInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			a.state = viewList
			a.searchIn.Blur()
			return a, nil
		case "enter":
			query := strings.TrimSpace(a.searchIn.Value())
			a.state = viewList
			a.searchIn.Blur()
			if query == "" {
				return a, loadResourcesCmd(a.queries, a.showTrashed)
			}
			return a, searchResourcesCmd(a.queries, query)
		}
	}
	var cmd tea.Cmd
	a.searchIn, cmd = a.searchIn.Update(msg)
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
			a.statusMsg = "canceled"
			return a, nil
		}
	}
	return a, nil
}

// View composes the active view with a status line at the bottom.
func (a App) View() string {
	var body string
	switch a.currentArea {
	case areaToday:
		body = a.viewToday()
	case areaFinance:
		body = placeholderView("Finance (coming soon — v3.x)", a.width, a.height)
	case areaCalendar:
		body = placeholderView("Calendar (coming soon — v3.x)", a.width, a.height)
	case areaTodos:
		switch a.todoState {
		case todoStateDetail:
			body = a.todoDetail.View()
		case todoStateConfirmDelete:
			body = a.todoConfirm.view(a.width, a.height)
		case todoStateSearchInput:
			body = renderSearchOverlay(a.todoSearchIn.View(), a.width, a.height)
		default:
			header := ""
			if a.todoShowTrashed {
				header = trashBannerStyle.Render(" TRASH ") + "\n"
			}
			if a.todoSearchActive != "" {
				header += mutedStyle.Render(fmt.Sprintf("  search: %q  (c to clear)", a.todoSearchActive)) + "\n"
			}
			body = header + a.todoList.View()
		}
	case areaResources:
		switch a.state {
		case viewDetail:
			body = a.detail.View()
		case viewConfirmDelete:
			body = a.confirm.view(a.width, a.height)
		case viewSearchInput:
			body = renderSearchOverlay(a.searchIn.View(), a.width, a.height)
		default:
			header := ""
			if a.showTrashed {
				header = trashBannerStyle.Render(" TRASH ") + "\n"
			}
			if a.searchActive != "" {
				header += mutedStyle.Render(fmt.Sprintf("  search: %q  (c to clear)", a.searchActive)) + "\n"
			}
			body = header + a.list.View()
		}
	}
	return body + "\n" + a.statusLine()
}

func placeholderView(text string, width, height int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Padding(1, 2).
		Render(text)
	return lipgloss.Place(width, height-1, lipgloss.Center, lipgloss.Center, box)
}

// renderSearchOverlay centers a small bordered prompt over the screen.
// The list is hidden underneath while the user types.
func renderSearchOverlay(input string, width, height int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(60).
		Render("Search\n\n" + input + "\n\n" + mutedStyle.Render("[enter] search   [esc] cancel"))
	return lipgloss.Place(width, height-1, lipgloss.Center, lipgloss.Center, box)
}

func (a App) statusLine() string {
	if a.statusErr != nil {
		return statusErrorStyle.Render("error: ") + a.statusErr.Error()
	}
	if a.statusMsg != "" {
		return statusOKStyle.Render(a.statusMsg)
	}
	area := areaNames[a.currentArea]
	parts := []string{
		keyStyle.Render(area),
		keyStyle.Render("tab") + " next",
		keyStyle.Render("shift+tab") + " prev",
		keyStyle.Render("1-5") + " jump",
	}
	if a.currentArea == areaToday {
		parts = append(parts, keyStyle.Render("r")+" refresh", keyStyle.Render("n")+" new")
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

// loadCurrentAreaCmd returns a command to load data for the current area.
func (a App) loadCurrentAreaCmd() tea.Cmd {
	switch a.currentArea {
	case areaToday:
		return reloadTodayCmd(a.todayService)
	case areaResources:
		return loadResourcesCmd(a.queries, a.showTrashed)
	case areaTodos:
		return loadTodosCmd(a.todosService, a.todoShowTrashed, a.todoSearchActive)
	default:
		return nil
	}
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

// searchResourcesCmd dispatches an FTS5 query and wraps the result so the
// list view can swap to it. Empty results aren't an error — the status
// line handles "0 match" rendering.
func searchResourcesCmd(q *store.Queries, query string) tea.Cmd {
	return func() tea.Msg {
		items, err := q.SearchResources(context.Background(), query, 200)
		return searchResultsMsg{query: query, items: items, err: err}
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
