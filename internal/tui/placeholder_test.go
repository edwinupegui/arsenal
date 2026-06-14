package tui

import (
	"strings"
	"testing"
)

func TestPlaceholderToday(t *testing.T) {
	app := App{currentArea: areaToday, width: 80, height: 24}
	view := app.View()
	if !strings.Contains(view, "Nothing on your plate today") {
		t.Errorf("Today empty state not found in view:\n%s", view)
	}
}

func TestPlaceholderFinance(t *testing.T) {
	// Finance is now a real sub-model — the placeholder must NOT appear.
	app := App{currentArea: areaFinance, width: 80, height: 24, keys: defaultKeys()}
	view := app.View()
	if strings.Contains(view, "coming soon") {
		t.Errorf("Finance placeholder should be gone but was found in view:\n%s", view)
	}
	// The finance list header or status line must appear instead.
	if !strings.Contains(view, "Finance") {
		t.Errorf("Finance area label not found in view:\n%s", view)
	}
}

func TestPlaceholderCalendar(t *testing.T) {
	app := App{currentArea: areaCalendar, width: 80, height: 24}
	view := app.View()
	if !strings.Contains(view, "Calendar (coming soon — v3.x)") {
		t.Errorf("Calendar placeholder not found in view:\n%s", view)
	}
}
