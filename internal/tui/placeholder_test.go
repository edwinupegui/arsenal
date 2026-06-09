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
	app := App{currentArea: areaFinance, width: 80, height: 24}
	view := app.View()
	if !strings.Contains(view, "Finance (coming soon — v3.x)") {
		t.Errorf("Finance placeholder not found in view:\n%s", view)
	}
}

func TestPlaceholderCalendar(t *testing.T) {
	app := App{currentArea: areaCalendar, width: 80, height: 24}
	view := app.View()
	if !strings.Contains(view, "Calendar (coming soon — v3.x)") {
		t.Errorf("Calendar placeholder not found in view:\n%s", view)
	}
}
