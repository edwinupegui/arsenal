package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDefaultAreaIsResources(t *testing.T) {
	app := New(nil)
	if app.currentArea != areaResources {
		t.Errorf("default area = %d, want %d (areaResources)", app.currentArea, areaResources)
	}
}

func TestAreaNames(t *testing.T) {
	want := map[areaID]string{
		areaToday:     "Today",
		areaResources: "Resources",
		areaTodos:     "Todos",
		areaFinance:   "Finance",
		areaCalendar:  "Calendar",
	}
	for id, name := range want {
		if got := areaNames[id]; got != name {
			t.Errorf("areaNames[%d] = %q, want %q", id, got, name)
		}
	}
}

func TestTabCyclesForward(t *testing.T) {
	app := App{currentArea: areaResources, keys: defaultKeys()}
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := model.(App).currentArea; got != areaTodos {
		t.Errorf("Tab from Resources = %d, want %d (areaTodos)", got, areaTodos)
	}
}

func TestShiftTabCyclesBackward(t *testing.T) {
	app := App{currentArea: areaTodos, keys: defaultKeys()}
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if got := model.(App).currentArea; got != areaResources {
		t.Errorf("Shift+Tab from Todos = %d, want %d (areaResources)", got, areaResources)
	}
}

func TestTabWrapsForward(t *testing.T) {
	app := App{currentArea: areaCalendar, keys: defaultKeys()}
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := model.(App).currentArea; got != areaToday {
		t.Errorf("Tab from Calendar = %d, want %d (areaToday)", got, areaToday)
	}
}

func TestShiftTabWrapsBackward(t *testing.T) {
	app := App{currentArea: areaToday, keys: defaultKeys()}
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if got := model.(App).currentArea; got != areaCalendar {
		t.Errorf("Shift+Tab from Today = %d, want %d (areaCalendar)", got, areaCalendar)
	}
}

func TestDirectJumpKeys(t *testing.T) {
	cases := []struct {
		key   string
		want  areaID
	}{
		{"1", areaToday},
		{"2", areaResources},
		{"3", areaTodos},
		{"4", areaFinance},
		{"5", areaCalendar},
	}
	for _, tc := range cases {
		app := App{currentArea: areaResources, keys: defaultKeys()} // start from any area
		model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		if got := model.(App).currentArea; got != tc.want {
			t.Errorf("key %s = %d, want %d", tc.key, got, tc.want)
		}
	}
}
