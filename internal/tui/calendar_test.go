package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/edwinupegui/arsenal/internal/calendar"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
)

// newCalendarTestDB opens a temp SQLite DB, runs migrations, and returns a
// *calendar.Service backed by it.
func newCalendarTestDB(t *testing.T) (*calendar.Service, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "arsenal.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		_ = db.Close()
		t.Fatalf("migrate: %v", err)
	}
	svc := calendar.New(db)
	return svc, func() { _ = db.Close() }
}

// seedCalendarEvent creates a timed event in the DB and returns it.
func seedCalendarEvent(t *testing.T, svc *calendar.Service, title string) *calendar.Event {
	t.Helper()
	ev, err := svc.Create(t.Context(), calendar.CreateInput{
		Title:      title,
		StartAt:    "2026-06-15T09:00:00",
		Recurrence: calendar.RecurrenceNone,
	})
	if err != nil {
		t.Fatalf("create calendar event: %v", err)
	}
	return ev
}

// seedCalendarAllDay creates an all-day event in the DB and returns it.
func seedCalendarAllDay(t *testing.T, svc *calendar.Service, title string) *calendar.Event {
	t.Helper()
	ev, err := svc.Create(t.Context(), calendar.CreateInput{
		Title:      title,
		StartAt:    "2026-06-15",
		AllDay:     true,
		Recurrence: calendar.RecurrenceNone,
	})
	if err != nil {
		t.Fatalf("create all-day calendar event: %v", err)
	}
	return ev
}

// --- Scenario: Calendar area renders event list --------------------------------

func TestCalendarAreaRendersEventList(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	seedCalendarEvent(t, svc, "Team standup")

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	view := app.View()
	if !strings.Contains(view, "Team standup") {
		t.Errorf("calendar list should show 'Team standup', got:\n%s", view)
	}
}

// --- Scenario: Placeholder message no longer appears --------------------------

func TestCalendarPlaceholderGone(t *testing.T) {
	app := New(nil)
	app.currentArea = areaCalendar

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	view := app.View()
	if strings.Contains(view, "coming soon") {
		t.Errorf("calendar placeholder should be gone, got:\n%s", view)
	}
}

// --- Scenario: Tab cycles to Calendar -----------------------------------------

func TestTabCyclesToCalendar(t *testing.T) {
	app := App{currentArea: areaFinance, keys: defaultKeys()}
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := model.(App).currentArea; got != areaCalendar {
		t.Errorf("Tab from Finance = %d, want %d (areaCalendar)", got, areaCalendar)
	}
}

// --- Scenario: j/k navigate ---------------------------------------------------

func TestCalendarJKeyMovesDown(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	seedCalendarEvent(t, svc, "Meeting A")
	seedCalendarEvent(t, svc, "Meeting B")

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	initialIdx := app.calendarList.Index()

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	app = model.(App)

	if app.calendarList.Index() <= initialIdx {
		t.Errorf("j key should move selection down: was %d, now %d", initialIdx, app.calendarList.Index())
	}
}

// --- Scenario: enter opens detail ---------------------------------------------

func TestCalendarEnterOpensDetail(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	seedCalendarEvent(t, svc, "Team standup")

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	app.calendarList.Select(0)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(App)

	if app.calendarState != calendarStateDetail {
		t.Errorf("enter should switch to calendarStateDetail, got %d", app.calendarState)
	}
}

// --- Scenario: detail view shows all-day indicator ----------------------------

func TestCalendarDetailAllDayIndicator(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	ev := seedCalendarAllDay(t, svc, "Birthday Party")

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar
	app.calendarState = calendarStateDetail

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	app.calendarDetail.SetEvent(ev)

	view := app.View()
	if !strings.Contains(view, "All day") {
		t.Errorf("detail should show 'All day' for all-day events, got:\n%s", view)
	}
}

// --- Scenario: detail view shows timed range ----------------------------------

func TestCalendarDetailTimedRange(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	ev, err := svc.Create(t.Context(), calendar.CreateInput{
		Title:      "Sprint Review",
		StartAt:    "2026-06-15T10:00:00",
		EndAt:      "2026-06-15T11:00:00",
		Recurrence: calendar.RecurrenceNone,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar
	app.calendarState = calendarStateDetail

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	app.calendarDetail.SetEvent(ev)

	view := app.View()
	if !strings.Contains(view, "10:00") {
		t.Errorf("detail should show start time '10:00', got:\n%s", view)
	}
	if !strings.Contains(view, "11:00") {
		t.Errorf("detail should show end time '11:00', got:\n%s", view)
	}
}

// --- Scenario: detail view shows open-ended event with dash -------------------

func TestCalendarDetailOpenEnded(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	ev := seedCalendarEvent(t, svc, "Open-ended event")

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar
	app.calendarState = calendarStateDetail

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	app.calendarDetail.SetEvent(ev)

	view := app.View()
	if !strings.Contains(view, "—") {
		t.Errorf("detail should show '—' for open-ended event end, got:\n%s", view)
	}
}

// --- Scenario: detail view shows tags -----------------------------------------

func TestCalendarDetailShowsTags(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	ev, err := svc.Create(t.Context(), calendar.CreateInput{
		Title:      "Team sync",
		StartAt:    "2026-06-15T09:00:00",
		Recurrence: calendar.RecurrenceNone,
		Tags:       []string{"work", "standup"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar
	app.calendarState = calendarStateDetail

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	app.calendarDetail.SetEvent(ev)

	view := app.View()
	if !strings.Contains(view, "work") {
		t.Errorf("detail should show tag 'work', got:\n%s", view)
	}
}

// --- Scenario: d key triggers soft-delete confirm -----------------------------

func TestCalendarSoftDeleteKey(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	seedCalendarEvent(t, svc, "Team standup")

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	app.calendarList.Select(0)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	app = model.(App)

	if app.calendarState != calendarStateConfirmDelete {
		t.Errorf("d key should enter calendarStateConfirmDelete, got %d", app.calendarState)
	}
}

// --- Scenario: r key in trash view restores -----------------------------------

func TestCalendarRestoreKeyInTrashView(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	ev := seedCalendarEvent(t, svc, "Team standup")
	if err := svc.SoftDelete(t.Context(), ev.Row.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar
	app.calendarShowTrashed = true

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	app.calendarList.Select(0)

	model, cmd = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	_ = model.(App)

	if cmd == nil {
		t.Errorf("r in trash view should return a restore command, got nil")
	}
}

// --- Scenario: x key in trash view triggers purge confirm ---------------------

func TestCalendarPurgeKeyInTrashView(t *testing.T) {
	svc, cleanup := newCalendarTestDB(t)
	defer cleanup()

	ev := seedCalendarEvent(t, svc, "Team standup")
	if err := svc.SoftDelete(t.Context(), ev.Row.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	app := New(nil)
	app.calendarService = svc
	app.currentArea = areaCalendar
	app.calendarShowTrashed = true

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	app.calendarList.Select(0)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	app = model.(App)

	if app.calendarState != calendarStateConfirmPurge {
		t.Errorf("x in trash view should enter calendarStateConfirmPurge, got %d", app.calendarState)
	}
}

// --- Scenario: status bar shows Calendar hints --------------------------------

func TestCalendarStatusBarHints(t *testing.T) {
	app := New(nil)
	app.currentArea = areaCalendar

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	view := app.View()
	if !strings.Contains(view, "Calendar") {
		t.Errorf("status bar should show 'Calendar', got:\n%s", view)
	}
}
