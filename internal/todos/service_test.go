package todos_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/todos"
)

func validCreate() todos.CreateInput {
	return todos.CreateInput{
		Title:       "pagar luz",
		Description: "mensual",
		Priority:    todos.PriorityHigh,
		DueDate:     strPtr("2026-06-10"),
		CategoryID:  nil,
		Notes:       "notas",
		Recurrence:  todos.RecurrenceWeekly,
		Tags:        []string{"urgente", "casa", "  CASA  "}, // dup + whitespace
	}
}

func TestCreate_HappyPath(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	got, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if got.Row.Title != "pagar luz" {
		t.Errorf("title = %q, want %q", got.Row.Title, "pagar luz")
	}
	if got.Row.Description == nil || *got.Row.Description != "mensual" {
		t.Errorf("description = %v, want mensual", got.Row.Description)
	}
	if got.Row.Priority != "high" {
		t.Errorf("priority = %q, want high", got.Row.Priority)
	}
	if got.Row.Status != "open" {
		t.Errorf("status = %q, want open", got.Row.Status)
	}
	if got.Row.DueDate == nil || *got.Row.DueDate != "2026-06-10" {
		t.Errorf("due_date = %v, want 2026-06-10", got.Row.DueDate)
	}
	if got.Row.CategoryID != nil {
		t.Errorf("category_id = %v, want nil", got.Row.CategoryID)
	}
	if got.Row.Notes == nil || *got.Row.Notes != "notas" {
		t.Errorf("notes = %v, want notas", got.Row.Notes)
	}
	if got.Row.Recurrence != "weekly" {
		t.Errorf("recurrence = %q, want weekly", got.Row.Recurrence)
	}
	if got.Row.DoneAt != nil {
		t.Errorf("done_at = %v, want nil", got.Row.DoneAt)
	}
	if got.Row.DeletedAt != nil {
		t.Errorf("deleted_at = %v, want nil", got.Row.DeletedAt)
	}

	wantTags := []string{"casa", "urgente"}
	if !equalStrings(got.Tags, wantTags) {
		t.Errorf("tags = %v, want %v", got.Tags, wantTags)
	}

	// Round-trip via Get
	round, err := svc.Get(ctx, got.Row.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !equalStrings(round.Tags, wantTags) {
		t.Errorf("Get tags = %v, want %v", round.Tags, wantTags)
	}
}

func TestCreate_Defaults(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	got, err := svc.Create(ctx, todos.CreateInput{Title: "leer ADR"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.Priority != "med" {
		t.Errorf("priority = %q, want med", got.Row.Priority)
	}
	if got.Row.Recurrence != "none" {
		t.Errorf("recurrence = %q, want none", got.Row.Recurrence)
	}
	if got.Row.Status != "open" {
		t.Errorf("status = %q, want open", got.Row.Status)
	}
	if got.Row.Description != nil {
		t.Errorf("description = %v, want nil", got.Row.Description)
	}
	if got.Row.DueDate != nil {
		t.Errorf("due_date = %v, want nil", got.Row.DueDate)
	}
	if got.Row.CategoryID != nil {
		t.Errorf("category_id = %v, want nil", got.Row.CategoryID)
	}
	if got.Row.Notes != nil {
		t.Errorf("notes = %v, want nil", got.Row.Notes)
	}
	if len(got.Tags) != 0 {
		t.Errorf("tags = %v, want empty", got.Tags)
	}
}

func TestCreate_EmptyTitle(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: "   "}); err == nil {
		t.Fatal("expected error, got nil")
	}

	q := store.New(db)
	count, err := q.CountOpenTodos(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 todos, got %d", count)
	}
}

func TestCreate_TitleTooLong(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: strings.Repeat("a", domain.MaxTitleLength+1)}); err == nil {
		t.Fatal("expected error, got nil")
	}

	q := store.New(db)
	count, err := q.CountOpenTodos(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 todos, got %d", count)
	}
}

func TestCreate_InvalidPriority(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: "test", Priority: "invalid"}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreate_InvalidRecurrence(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: "test", Recurrence: "yearly"}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGet_Found(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Get(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Row.ID != created.Row.ID {
		t.Errorf("id = %d, want %d", got.Row.ID, created.Row.ID)
	}
	if got.Row.Title != created.Row.Title {
		t.Errorf("title = %q, want %q", got.Row.Title, created.Row.Title)
	}
	wantTags := []string{"casa", "urgente"}
	if !equalStrings(got.Tags, wantTags) {
		t.Errorf("tags = %v, want %v", got.Tags, wantTags)
	}
}

func TestGet_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Get(ctx, 999); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdate_ChangesPriority(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Row.ID, todos.CreateInput{
		Title:    created.Row.Title,
		Priority: todos.PriorityLow,
		Recurrence: todos.RecurrenceWeekly,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Row.Priority != "low" {
		t.Errorf("priority = %q, want low", updated.Row.Priority)
	}
}

func TestUpdate_TagReplacementPrunesOrphans(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Row.ID, todos.CreateInput{
		Title:      created.Row.Title,
		Priority:   todos.PriorityHigh,
		Recurrence: todos.RecurrenceWeekly,
		Tags:       []string{"ddd", "patterns"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !equalStrings(updated.Tags, []string{"ddd", "patterns"}) {
		t.Errorf("after Update tags = %v", updated.Tags)
	}

	q := store.New(db)
	tags, err := q.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	names := make(map[string]bool, len(tags))
	for _, tag := range tags {
		names[tag.Name] = true
	}
	for _, want := range []string{"ddd", "patterns"} {
		if !names[want] {
			t.Errorf("missing tag %q", want)
		}
	}
	for _, gone := range []string{"casa", "urgente"} {
		if names[gone] {
			t.Errorf("orphan tag %q should have been pruned", gone)
		}
	}
}

func TestUpdate_NonExistentFails(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Update(ctx, 999, todos.CreateInput{
		Title:      "ghost",
		Priority:   todos.PriorityMed,
		Recurrence: todos.RecurrenceNone,
	}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSoftDelete_Active(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	q := store.New(db)
	row, err := q.GetTodo(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}
	if row.DeletedAt == nil {
		t.Fatal("expected deleted_at to be set")
	}
}

func TestSoftDelete_AlreadyDeleted(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	// Second call should be a no-op (no error)
	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete again: %v", err)
	}
}

func TestRestore_SoftDeleted(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	if err := svc.Restore(ctx, created.Row.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	q := store.New(db)
	row, err := q.GetTodo(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}
	if row.DeletedAt != nil {
		t.Fatal("expected deleted_at to be nil after restore")
	}
}

func TestRestore_Active(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Restoring an active todo is a no-op
	if err := svc.Restore(ctx, created.Row.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	q := store.New(db)
	row, err := q.GetTodo(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}
	if row.DeletedAt != nil {
		t.Fatal("expected deleted_at to be nil")
	}
}

func TestPurge_AfterSoftDelete(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	if err := svc.Purge(ctx, created.Row.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	q := store.New(db)
	if _, err := q.GetTodo(ctx, created.Row.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestPurge_Active(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Purge(ctx, created.Row.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	q := store.New(db)
	if _, err := q.GetTodo(ctx, created.Row.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
