package todos_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
)

func newTestDB(t *testing.T) *sql.DB {
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

func TestSmoke_CreateAndGetRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	q := store.New(db)

	created, err := q.CreateTodo(ctx, store.CreateTodoParams{
		Title:       "pagar luz",
		Description: strPtr("mensual"),
		Priority:    "high",
		Status:      "open",
		DueDate:     strPtr("2026-06-10"),
		CategoryID:  nil,
		Notes:       strPtr("mensual"),
		Recurrence:  "weekly",
	})
	if err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if created.Title != "pagar luz" {
		t.Errorf("title = %q, want %q", created.Title, "pagar luz")
	}
	if created.Priority != "high" {
		t.Errorf("priority = %q, want %q", created.Priority, "high")
	}
	if created.Status != "open" {
		t.Errorf("status = %q, want %q", created.Status, "open")
	}
	if created.Recurrence != "weekly" {
		t.Errorf("recurrence = %q, want %q", created.Recurrence, "weekly")
	}
	if created.DueDate == nil || *created.DueDate != "2026-06-10" {
		t.Errorf("due_date = %v, want 2026-06-10", created.DueDate)
	}
	if created.DoneAt != nil {
		t.Errorf("done_at = %v, want nil", created.DoneAt)
	}
	if created.DeletedAt != nil {
		t.Errorf("deleted_at = %v, want nil", created.DeletedAt)
	}

	// Read back
	got, err := q.GetTodo(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("id = %d, want %d", got.ID, created.ID)
	}
	if got.Title != created.Title {
		t.Errorf("title = %q, want %q", got.Title, created.Title)
	}
	if got.Priority != created.Priority {
		t.Errorf("priority = %q, want %q", got.Priority, created.Priority)
	}
}

func TestSmoke_CreateWithDefaults(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	q := store.New(db)

	// The sqlc query requires valid values for CHECK columns; default
	// substitution happens at the service layer (WU B).
	created, err := q.CreateTodo(ctx, store.CreateTodoParams{
		Title:      "leer ADR",
		Priority:   "med",
		Status:     "open",
		Recurrence: "none",
	})
	if err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}
	if created.Priority != "med" {
		t.Errorf("priority = %q, want med", created.Priority)
	}
	if created.Recurrence != "none" {
		t.Errorf("recurrence = %q, want none", created.Recurrence)
	}
	if created.Description != nil {
		t.Errorf("description = %v, want nil", created.Description)
	}
	if created.DueDate != nil {
		t.Errorf("due_date = %v, want nil", created.DueDate)
	}
	if created.CategoryID != nil {
		t.Errorf("category_id = %v, want nil", created.CategoryID)
	}
	if created.Status != "open" {
		t.Errorf("status = %q, want open", created.Status)
	}
}

func strPtr(s string) *string { return &s }
