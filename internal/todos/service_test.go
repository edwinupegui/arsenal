package todos_test

import (
	"context"
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
