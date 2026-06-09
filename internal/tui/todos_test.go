package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/todos"
)

func newTestDB(t *testing.T) *todos.Service {
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
	return todos.New(db)
}

func TestTodosAreaShowsList(t *testing.T) {
	svc := newTestDB(t)
	ctx := t.Context()
	_, err := svc.Create(ctx, todos.CreateInput{Title: "comprar leche"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	app := New(nil)
	app.todosService = svc
	app.currentArea = areaTodos

	// Set size
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	// Simulate the load command
	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	view := app.View()
	if !strings.Contains(view, "comprar leche") {
		t.Errorf("todo list should show 'comprar leche', got:\n%s", view)
	}
}

func TestTodosMarkDone(t *testing.T) {
	svc := newTestDB(t)
	ctx := t.Context()
	_, err := svc.Create(ctx, todos.CreateInput{Title: "comprar leche"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	app := New(nil)
	app.todosService = svc
	app.currentArea = areaTodos

	// Set size and load the list
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)
	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	// Select the todo
	app.todoList.Select(0)

	// Press 'x' to mark done
	model, cmd = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	app = model.(App)

	// Verify the mutation command was returned
	if cmd == nil {
		t.Errorf("expected mutation command after 'x' key, got none")
	}
}

func TestTodosSoftDelete(t *testing.T) {
	svc := newTestDB(t)
	ctx := t.Context()
	_, err := svc.Create(ctx, todos.CreateInput{Title: "comprar leche"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	app := New(nil)
	app.todosService = svc
	app.currentArea = areaTodos

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)
	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)
	app.todoList.Select(0)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	app = model.(App)

	if app.todoState != todoStateConfirmDelete {
		t.Errorf("expected confirm delete state, got %d", app.todoState)
	}
}
