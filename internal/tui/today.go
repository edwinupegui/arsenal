package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/todos"
)

// todayModel holds the rendered state for the Today area.
type todayModel struct {
	sections []today.Section
	errs     []today.ProviderError
	width    int
	height   int
}

// todayReloadedMsg is delivered after a background reload of Today data.
type todayReloadedMsg struct {
	sections []today.Section
	errs     []today.ProviderError
}

func reloadTodayCmd(svc *today.Service) tea.Cmd {
	return func() tea.Msg {
		secs, errs := svc.Build(context.Background())
		return todayReloadedMsg{sections: secs, errs: errs}
	}
}

// todoCreatedFromTodayMsg is delivered after a successful create-from-today.
type todoCreatedFromTodayMsg struct {
	status string
	err    error
}

// createTodoFromTodayCmd creates a new todo with the given title using the
// default "med" priority and no due date. The caller is expected to be in
// the areaToday area; the result is a tea.Msg that triggers a Today reload
// on success.
func createTodoFromTodayCmd(svc *todos.Service, title string) tea.Cmd {
	return func() tea.Msg {
		t, err := svc.Create(context.Background(), todos.CreateInput{
			Title:    title,
			Priority: todos.PriorityMed,
		})
		if err != nil {
			return todoCreatedFromTodayMsg{err: fmt.Errorf("create todo: %w", err)}
		}
		return todoCreatedFromTodayMsg{status: FormatNewTodoFormTitle(t.Row.Title)}
	}
}

func (a App) updateToday(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Form state takes precedence over the underlying view.
	if a.todayState == todayStateNewForm {
		return a.updateTodayNewForm(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.todayModel.width = msg.Width
		a.todayModel.height = msg.Height
		return a, nil

	case todayReloadedMsg:
		if len(msg.errs) > 0 {
			a.statusErr = fmt.Errorf("today provider error: %s", msg.errs[0].Name)
		} else {
			a.statusErr = nil
		}
		a.todayModel.sections = msg.sections
		return a, nil

	case todoCreatedFromTodayMsg:
		if msg.err != nil {
			a.statusErr = msg.err
			return a, nil
		}
		a.statusErr = nil
		a.statusMsg = msg.status
		// Reload Today so the new todo appears in the right section.
		return a, reloadTodayCmd(a.todayService)
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case km.String() == "r":
			return a, reloadTodayCmd(a.todayService)
		case km.String() == "n":
			a.todayState = todayStateNewForm
			a.newTodoForm.Reset()
			a.statusErr = nil
			a.statusMsg = ""
			return a, nil
		}
	}
	return a, nil
}

// updateTodayNewForm handles key events while the new-todo form is open.
func (a App) updateTodayNewForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			a.todayState = todayStateView
			a.newTodoForm.Blur()
			a.statusErr = nil
			a.statusMsg = "canceled"
			return a, nil
		case "enter":
			title := a.newTodoForm.Title()
			if title == "" {
				a.statusErr = fmt.Errorf("title is required")
				return a, nil
			}
			a.todayState = todayStateView
			a.newTodoForm.Blur()
			return a, createTodoFromTodayCmd(a.todosService, title)
		}
	}
	var cmd tea.Cmd
	a.newTodoForm, cmd = a.newTodoForm.Update(msg)
	return a, cmd
}

func (a App) viewToday() string {
	// Form is rendered as a centered overlay; the underlying sections are
	// shown faintly behind it via placeholderView.
	if a.todayState == todayStateNewForm {
		return lipgloss.Place(
			a.todayModel.width, a.todayModel.height-1,
			lipgloss.Center, lipgloss.Center,
			a.newTodoForm.View(),
		)
	}
	if len(a.todayModel.sections) == 0 {
		return placeholderView(today.RenderEmptyState("tui"), a.todayModel.width, a.todayModel.height)
	}

	var b strings.Builder
	for _, sec := range a.todayModel.sections {
		b.WriteString(sectionHeaderStyle.Render(sec.Title) + "\n")
		for _, it := range sec.Items {
			line := "  " + it.Title
			if it.Subtitle != "" {
				line += "  " + mutedStyle.Render(it.Subtitle)
			}
			if it.Priority != "" {
				line += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b")).Render(it.Priority)
			}
			b.WriteString(line + "\n")
		}
		if sec.ShowAllURL != "" {
			b.WriteString("  " + mutedStyle.Render("show all → "+sec.ShowAllURL) + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

var sectionHeaderStyle = lipgloss.NewStyle().
	Foreground(colorAccent).
	Bold(true).
	Underline(true)
