package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestApp_AreaToday_NKeyOpensNewTodoForm verifies that pressing 'n' in the
// Today area opens an inline new-todo form (previously it just switched
// to the areaTodos; that was flagged as a v3.0 follow-up in sdd-verify).
func TestApp_AreaToday_NKeyOpensNewTodoForm(t *testing.T) {
	app := New(nil)
	app.currentArea = areaToday
	// Start clean: no form open yet.
	if app.todayState == todayStateNewForm {
		t.Fatalf("precondition: todayState should not be todayStateNewForm before pressing n")
	}
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	a := model.(App)
	if a.todayState != todayStateNewForm {
		t.Errorf("after 'n' key: todayState = %d, want todayStateNewForm (%d)",
			a.todayState, todayStateNewForm)
	}
	if a.currentArea != areaToday {
		t.Errorf("after 'n' key: currentArea = %d, want areaToday (must NOT switch)", a.currentArea)
	}
}

// TestApp_AreaToday_NewTodoForm_EscCancels verifies that pressing 'esc' while
// the form is open returns to the Today view without creating a todo.
func TestApp_AreaToday_NewTodoForm_EscCancels(t *testing.T) {
	app := New(nil)
	app.currentArea = areaToday
	app.todayState = todayStateNewForm
	app.newTodoForm.titleIn.SetValue("some draft")
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)
	if a.todayState == todayStateNewForm {
		t.Errorf("after esc: todayState still = todayStateNewForm, expected to exit form")
	}
	if a.currentArea != areaToday {
		t.Errorf("after esc: currentArea = %d, want areaToday", a.currentArea)
	}
}

// TestApp_AreaToday_NewTodoForm_EmptyTitleIsNoop verifies that submitting
// an empty title does NOT create a todo (returns an error status instead).
func TestApp_AreaToday_NewTodoForm_EmptyTitleIsNoop(t *testing.T) {
	app := New(nil)
	app.currentArea = areaToday
	app.todayState = todayStateNewForm
	app.newTodoForm.titleIn.SetValue("   ") // whitespace only
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(App)
	// Form should remain open (validation failed) and no creation cmd should run.
	if a.todayState != todayStateNewForm {
		t.Errorf("after empty enter: todayState = %d, want todayStateNewForm (form should stay open on validation error)",
			a.todayState)
	}
	if cmd != nil {
		t.Errorf("after empty enter: expected nil cmd (no creation), got non-nil")
	}
	if a.statusErr == nil {
		t.Errorf("after empty enter: expected a status error for empty title")
	}
}

// TestApp_AreaToday_NewTodoForm_ViewContainsPlaceholder verifies the form
// renders the placeholder text and the title input prompt.
func TestApp_AreaToday_NewTodoForm_ViewContainsPlaceholder(t *testing.T) {
	app := New(nil)
	app.currentArea = areaToday
	app.todayState = todayStateNewForm
	app.width = 80
	app.height = 24
	view := app.View()
	if !strings.Contains(strings.ToLower(view), "new todo") {
		t.Errorf("form view missing 'new todo' header. View:\n%s", view)
	}
	if !strings.Contains(view, "title") {
		t.Errorf("form view missing 'title' label. View:\n%s", view)
	}
}

// TestApp_AreaToday_NewTodoForm_StatusBarShowsFormHints verifies that the
// status line shows the form-specific key hints when the form is open.
func TestApp_AreaToday_NewTodoForm_StatusBarShowsFormHints(t *testing.T) {
	app := New(nil)
	app.currentArea = areaToday
	app.todayState = todayStateNewForm
	line := app.statusLine()
	// Form hints should mention esc to cancel and enter to submit
	if !contains(line, "esc") {
		t.Errorf("status bar missing 'esc' hint when form is open. Got: %q", line)
	}
	if !contains(line, "enter") {
		t.Errorf("status bar missing 'enter' hint when form is open. Got: %q", line)
	}
}
