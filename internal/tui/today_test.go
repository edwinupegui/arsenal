package tui

import (
	"database/sql"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
)

func newSQLDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "arsenal.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestApp_AreaToday_DispatchesToUpdateToday(t *testing.T) {
	app := New(nil)
	app.currentArea = areaToday
	msg := todayReloadedMsg{sections: []today.Section{{Key: "overdue", Items: []today.Item{{Title: "x"}}}}}
	model, _ := app.Update(msg)
	if len(model.(App).todayModel.sections) != 1 {
		t.Error("expected todayModel.sections updated")
	}
}

func TestApp_AreaToday_RKeyTriggersReload(t *testing.T) {
	app := New(nil)
	app.currentArea = areaToday
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Error("expected non-nil command for r key in areaToday")
	}
}

func TestApp_AreaToday_NKeyOpensNewTodo(t *testing.T) {
	app := New(nil)
	app.currentArea = areaToday
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	a := model.(App)
	// v3.0.1: n in areaToday opens an inline new-todo form (no longer
	// switches to areaTodos). See today_new_form_test.go for the form
	// behavior tests.
	if a.todayState != todayStateNewForm {
		t.Errorf("n key: todayState = %d, want todayStateNewForm (%d)",
			a.todayState, todayStateNewForm)
	}
}

func TestApp_StatusBar_TodayHints(t *testing.T) {
	app := New(nil)
	app.currentArea = areaToday
	line := app.statusLine()
	if !contains(line, "Today") {
		t.Errorf("status bar missing 'Today', got: %q", line)
	}
	if !contains(line, "r") {
		t.Errorf("status bar missing 'r' hint, got: %q", line)
	}
	if !contains(line, "n") {
		t.Errorf("status bar missing 'n' hint, got: %q", line)
	}
}

func TestApp_DefaultLanding_Today(t *testing.T) {
	app := New(nil)
	if app.currentArea != areaToday {
		t.Errorf("default landing = %d, want areaToday", app.currentArea)
	}
}

func TestApp_LandingSurface_Resources(t *testing.T) {
	db := newSQLDB(t)
	if _, err := db.Exec(`INSERT INTO arsenal_config (k, v) VALUES (?, ?)`, "landing_surface", "resources"); err != nil {
		t.Fatalf("insert config: %v", err)
	}
	app := New(db)
	if app.currentArea != areaResources {
		t.Errorf("landing = %d, want areaResources", app.currentArea)
	}
}

func TestApp_LandingSurface_InvalidFallback(t *testing.T) {
	db := newSQLDB(t)
	if _, err := db.Exec(`INSERT INTO arsenal_config (k, v) VALUES (?, ?)`, "landing_surface", "finance"); err != nil {
		t.Fatalf("insert config: %v", err)
	}
	app := New(db)
	if app.currentArea != areaToday {
		t.Errorf("invalid landing fallback = %d, want areaToday", app.currentArea)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSub(s, substr))
}

func containsSub(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
