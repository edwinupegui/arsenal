package tui

import (
	"strings"
	"testing"
)

func TestStatusBarShowsCurrentArea(t *testing.T) {
	app := New(nil)
	app.currentArea = areaResources
	line := app.statusLine()
	if !strings.Contains(line, "Resources") {
		t.Errorf("status bar should show 'Resources', got: %s", line)
	}
	if !strings.Contains(line, "tab") {
		t.Errorf("status bar should show 'tab' hint, got: %s", line)
	}

	app.currentArea = areaTodos
	line = app.statusLine()
	if !strings.Contains(line, "Todos") {
		t.Errorf("status bar should show 'Todos', got: %s", line)
	}
}

func TestStatusBarShowsKeyHints(t *testing.T) {
	app := New(nil)
	line := app.statusLine()
	if !strings.Contains(line, "tab") {
		t.Errorf("status bar should show tab hint, got: %s", line)
	}
	if !strings.Contains(line, "shift+tab") {
		t.Errorf("status bar should show shift+tab hint, got: %s", line)
	}
	if !strings.Contains(line, "1-5") {
		t.Errorf("status bar should show 1-5 hint, got: %s", line)
	}
}
