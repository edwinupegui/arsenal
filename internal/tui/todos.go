package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/edwinupegui/arsenal/internal/todos"
)

// todoItem adapts a todos.Todo to bubbles/list's Item interface.
type todoItem struct {
	t *todos.Todo
}

func newTodoItem(t *todos.Todo) todoItem { return todoItem{t: t} }

func (i todoItem) Title() string {
	status := ""
	if i.t.Row.Status == "done" {
		status = "[✓] "
	} else {
		status = "[ ] "
	}
	return status + i.t.Row.Title
}

func (i todoItem) Description() string {
	parts := []string{string(i.t.Row.Priority), i.t.Row.Status}
	if i.t.Row.DueDate != nil && *i.t.Row.DueDate != "" {
		parts = append(parts, "due "+*i.t.Row.DueDate)
	}
	if len(i.t.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(i.t.Tags, " #"))
	}
	return strings.Join(parts, " · ")
}

func (i todoItem) FilterValue() string {
	desc := ""
	if i.t.Row.Description != nil {
		desc = *i.t.Row.Description
	}
	return strings.Join([]string{
		i.t.Row.Title,
		desc,
		strings.Join(i.t.Tags, " "),
	}, " ")
}

func asTodoItems(rows []*todos.Todo) []list.Item {
	out := make([]list.Item, 0, len(rows))
	for _, t := range rows {
		out = append(out, newTodoItem(t))
	}
	return out
}

// todoDetailModel renders a single todo's full info.
type todoDetailModel struct {
	vp        viewport.Model
	todo      *todos.Todo
	hasTodo   bool
	width     int
	maxHeight int
}

func newTodoDetailModel() todoDetailModel {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	return todoDetailModel{vp: vp}
}

func (m *todoDetailModel) SetTodo(t *todos.Todo) {
	m.todo = t
	m.hasTodo = true
	m.vp.SetContent(m.renderBody())
	m.vp.GotoTop()
}

func (m *todoDetailModel) SetSize(width, height int) {
	m.width = width
	m.maxHeight = height
	m.vp.Width = width
	m.vp.Height = height
	if m.hasTodo {
		m.vp.SetContent(m.renderBody())
	}
}

func (m todoDetailModel) Update(msg tea.Msg) (todoDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m todoDetailModel) View() string {
	if !m.hasTodo {
		return mutedStyle.Render("no todo selected")
	}
	return m.vp.View()
}

func (m todoDetailModel) renderBody() string {
	t := m.todo
	var b strings.Builder

	row := func(label, value string) {
		fmt.Fprintf(&b, "%s %s\n", detailLabelStyle.Render(label+":"), detailValueStyle.Render(value))
	}

	row("ID", fmt.Sprintf("%d", t.Row.ID))
	row("Title", t.Row.Title)
	row("Priority", t.Row.Priority)
	row("Status", t.Row.Status)
	if t.Row.DueDate != nil && *t.Row.DueDate != "" {
		row("Due Date", *t.Row.DueDate)
	}
	if t.Row.Recurrence != "" {
		row("Recurrence", t.Row.Recurrence)
	}
	if len(t.Tags) > 0 {
		row("Tags", "#"+strings.Join(t.Tags, " #"))
	}
	row("Created", t.Row.CreatedAt)
	row("Updated", t.Row.UpdatedAt)
	if t.Row.Description != nil && *t.Row.Description != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, detailLabelStyle.Render("Description"))
		fmt.Fprintln(&b, indentLines(*t.Row.Description, "  "))
	}
	if t.Row.Notes != nil && *t.Row.Notes != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, detailLabelStyle.Render("Notes"))
		fmt.Fprintln(&b, indentLines(*t.Row.Notes, "  "))
	}
	return b.String()
}

// --- updateTodos -----------------------------------------------------------

func (a App) updateTodos(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		listH := msg.Height - 2
		if listH < 5 {
			listH = 5
		}
		a.todoList.SetSize(msg.Width, listH)
		a.todoDetail.SetSize(msg.Width, listH)
		return a, nil

	case todosLoadedMsg:
		if msg.err != nil {
			a.statusErr = msg.err
			return a, nil
		}
		a.todoList.SetItems(asTodoItems(msg.items))
		a.statusErr = nil
		title := "Todos"
		if a.todoShowTrashed {
			title = "Todos — Trash"
		}
		if a.todoSearchActive != "" {
			title = fmt.Sprintf("Todos — search: %s", a.todoSearchActive)
		}
		a.todoList.Title = title
		return a, nil

	case todoMutatedMsg:
		if msg.err != nil {
			a.statusErr = msg.err
		} else {
			a.statusErr = nil
			a.statusMsg = msg.status
		}
		return a, loadTodosCmd(a.todosService, a.todoShowTrashed, a.todoSearchActive)
	}

	switch a.todoState {
	case todoStateDetail:
		return a.updateTodoDetail(msg)
	case todoStateSearchInput:
		return a.updateTodoSearchInput(msg)
	case todoStateConfirmDelete:
		return a.updateTodoConfirm(msg)
	default:
		return a.updateTodoList(msg)
	}
}

func (a App) updateTodoList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && a.todoList.FilterState() != list.Filtering {
		switch {
		case key.Matches(km, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(km, a.keys.Detail):
			if it, ok := a.selectedTodoItem(); ok {
				a.todoDetail.SetTodo(it.t)
				a.todoState = todoStateDetail
				return a, nil
			}
		case key.Matches(km, a.keys.Trash):
			a.todoShowTrashed = !a.todoShowTrashed
			a.statusMsg = ""
			return a, loadTodosCmd(a.todosService, a.todoShowTrashed, a.todoSearchActive)
		case key.Matches(km, a.keys.Search):
			a.todoSearchIn.SetValue("")
			a.todoSearchIn.Focus()
			a.todoState = todoStateSearchInput
			a.statusMsg = ""
			return a, textinput.Blink
		case key.Matches(km, a.keys.SoftDelete):
			if a.todoShowTrashed {
				return a, nil
			}
			if it, ok := a.selectedTodoItem(); ok {
				a.todoConfirm = newConfirmModel(
					fmt.Sprintf("Move %q to trash?", it.t.Row.Title),
					confirmActionDelete, it.t.Row.ID, it.t.Row.Title,
				)
				a.todoState = todoStateConfirmDelete
				return a, nil
			}
		case key.Matches(km, a.keys.Restore):
			if !a.todoShowTrashed {
				return a, nil
			}
			if it, ok := a.selectedTodoItem(); ok {
				return a, restoreTodoCmd(a.todosService, it.t)
			}
		case key.Matches(km, a.keys.MarkDone):
			if it, ok := a.selectedTodoItem(); ok {
				if it.t.Row.Status == "open" {
					return a, markTodoDoneCmd(a.todosService, it.t)
				}
				return a, markTodoOpenCmd(a.todosService, it.t)
			}
		}
	}
	var cmd tea.Cmd
	a.todoList, cmd = a.todoList.Update(msg)
	return a, cmd
}

func (a App) updateTodoDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, a.keys.Quit), key.Matches(km, a.keys.Back):
			a.todoState = todoStateList
			return a, nil
		case key.Matches(km, a.keys.MarkDone):
			if a.todoDetail.todo != nil {
				if a.todoDetail.todo.Row.Status == "open" {
					return a, markTodoDoneCmd(a.todosService, a.todoDetail.todo)
				}
				return a, markTodoOpenCmd(a.todosService, a.todoDetail.todo)
			}
		}
	}
	var cmd tea.Cmd
	a.todoDetail, cmd = a.todoDetail.Update(msg)
	return a, cmd
}

func (a App) updateTodoSearchInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			a.todoState = todoStateList
			a.todoSearchIn.Blur()
			return a, nil
		case "enter":
			query := strings.TrimSpace(a.todoSearchIn.Value())
			a.todoState = todoStateList
			a.todoSearchIn.Blur()
			a.todoSearchActive = query
			return a, searchTodosCmd(a.todosService, query)
		}
	}
	var cmd tea.Cmd
	a.todoSearchIn, cmd = a.todoSearchIn.Update(msg)
	return a, cmd
}

func (a App) updateTodoConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			id := a.todoConfirm.id
			a.todoState = todoStateList
			return a, softDeleteTodoCmd(a.todosService, id, a.todoConfirm.title)
		case "n", "N", "esc":
			a.todoState = todoStateList
			a.statusMsg = "canceled"
			return a, nil
		}
	}
	return a, nil
}

func (a App) selectedTodoItem() (todoItem, bool) {
	cur := a.todoList.SelectedItem()
	if cur == nil {
		return todoItem{}, false
	}
	it, ok := cur.(todoItem)
	return it, ok
}

// --- commands ---------------------------------------------------------------

// todosLoadedMsg arrives after a (re)load of the todo list completes.
type todosLoadedMsg struct {
	items []*todos.Todo
	err   error
}

func loadTodosCmd(svc *todos.Service, trashed bool, search string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		filter := todos.ListFilter{
			Trashed: trashed,
			Limit:   500,
			Search:  search,
		}
		items, err := svc.List(ctx, filter)
		return todosLoadedMsg{items: items, err: err}
	}
}

func searchTodosCmd(svc *todos.Service, query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		filter := todos.ListFilter{Search: query, Limit: 200}
		items, err := svc.List(ctx, filter)
		return todosLoadedMsg{items: items, err: err}
	}
}

func markTodoDoneCmd(svc *todos.Service, t *todos.Todo) tea.Cmd {
	return func() tea.Msg {
		if err := svc.MarkDone(context.Background(), t.Row.ID); err != nil {
			return todoMutatedMsg{err: err}
		}
		return todoMutatedMsg{status: fmt.Sprintf("done: %s", t.Row.Title)}
	}
}

func markTodoOpenCmd(svc *todos.Service, t *todos.Todo) tea.Cmd {
	return func() tea.Msg {
		if err := svc.MarkOpen(context.Background(), t.Row.ID); err != nil {
			return todoMutatedMsg{err: err}
		}
		return todoMutatedMsg{status: fmt.Sprintf("open: %s", t.Row.Title)}
	}
}

func softDeleteTodoCmd(svc *todos.Service, id int64, title string) tea.Cmd {
	return func() tea.Msg {
		if err := svc.SoftDelete(context.Background(), id); err != nil {
			return todoMutatedMsg{err: err}
		}
		return todoMutatedMsg{status: fmt.Sprintf("moved to trash: %s", title)}
	}
}

func restoreTodoCmd(svc *todos.Service, t *todos.Todo) tea.Cmd {
	return func() tea.Msg {
		if err := svc.Restore(context.Background(), t.Row.ID); err != nil {
			return todoMutatedMsg{err: err}
		}
		return todoMutatedMsg{status: fmt.Sprintf("restored: %s", t.Row.Title)}
	}
}
