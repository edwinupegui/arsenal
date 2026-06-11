package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// todayViewState selects the active sub-view within the Today area.
// Default is todayStateView (rendering the aggregated sections).
// todayStateNewForm is the inline new-todo creation form opened with 'n'.
type todayViewState int

const (
	todayStateView todayViewState = iota
	todayStateNewForm
)

// newTodoFormModel is a minimal inline form for creating a todo from the
// Today area. v3.0.1 MVP: title only; priority defaults to "med" and
// due date is unset. Editing/setting these fields is a follow-up.
type newTodoFormModel struct {
	titleIn textinput.Model
}

func newNewTodoFormModel() newTodoFormModel {
	ti := textinput.New()
	ti.Placeholder = "what needs doing?"
	ti.Prompt = "> "
	ti.CharLimit = 200
	ti.Focus()
	return newTodoFormModel{titleIn: ti}
}

// Focus sets focus on the title input (call when opening the form).
func (m *newTodoFormModel) Focus() {
	m.titleIn.Focus()
}

// Blur removes focus from the title input (call when closing the form).
func (m *newTodoFormModel) Blur() {
	m.titleIn.Blur()
}

// Reset clears the title input and refocuses (call when reopening the form).
func (m *newTodoFormModel) Reset() {
	m.titleIn.SetValue("")
	m.titleIn.Focus()
}

// Title returns the trimmed title currently in the input.
func (m newTodoFormModel) Title() string {
	return strings.TrimSpace(m.titleIn.Value())
}

// Update handles key events for the form. Returns the updated model and
// an optional command. The caller decides what to do on submit/cancel
// by inspecting the returned tuple; this model is purely UI state.
func (m newTodoFormModel) Update(msg tea.Msg) (newTodoFormModel, tea.Cmd) {
	var cmd tea.Cmd
	m.titleIn, cmd = m.titleIn.Update(msg)
	return m, cmd
}

// View renders the form inside a centered bordered box.
func (m newTodoFormModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("New todo") + "\n\n")
	b.WriteString(mutedStyle.Render("title") + "\n")
	b.WriteString(m.titleIn.View() + "\n\n")
	b.WriteString(mutedStyle.Render("priority: med (default; edit later with todo edit)") + "\n\n")
	b.WriteString(mutedStyle.Render("[enter] save   [esc] cancel"))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(60).
		Render(b.String())
	return box
}

// FormatNewTodoFormTitle is a helper for status messages.
func FormatNewTodoFormTitle(title string) string {
	return fmt.Sprintf("added: %s", title)
}
