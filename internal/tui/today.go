package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/edwinupegui/arsenal/internal/today"
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

func (a App) updateToday(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case km.String() == "r":
			return a, reloadTodayCmd(a.todayService)
		case km.String() == "n":
			a.currentArea = areaTodos
			a.statusMsg = "switch to Todos to add a new todo"
			return a, a.loadCurrentAreaCmd()
		}
	}
	return a, nil
}

func (a App) viewToday() string {
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
