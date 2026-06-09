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

func TestMarkDone_OpenToDone(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.MarkDone(ctx, created.Row.ID); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	q := store.New(db)
	row, err := q.GetTodo(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}
	if row.Status != "done" {
		t.Errorf("status = %q, want done", row.Status)
	}
	if row.DoneAt == nil {
		t.Fatal("expected done_at to be set")
	}
}

func TestMarkDone_AlreadyDone(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.MarkDone(ctx, created.Row.ID); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	q := store.New(db)
	before, err := q.GetTodo(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}

	// Second call should be a no-op
	if err := svc.MarkDone(ctx, created.Row.ID); err != nil {
		t.Fatalf("MarkDone again: %v", err)
	}

	after, err := q.GetTodo(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}
	if after.UpdatedAt != before.UpdatedAt {
		t.Errorf("updated_at changed from %q to %q", before.UpdatedAt, after.UpdatedAt)
	}
	if after.DoneAt == nil || before.DoneAt == nil || *after.DoneAt != *before.DoneAt {
		t.Errorf("done_at changed")
	}
}

func TestMarkOpen_DoneToOpen(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.MarkDone(ctx, created.Row.ID); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	if err := svc.MarkOpen(ctx, created.Row.ID); err != nil {
		t.Fatalf("MarkOpen: %v", err)
	}

	q := store.New(db)
	row, err := q.GetTodo(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}
	if row.Status != "open" {
		t.Errorf("status = %q, want open", row.Status)
	}
	if row.DoneAt != nil {
		t.Errorf("done_at = %v, want nil", row.DoneAt)
	}
}

func TestMarkOpen_AlreadyOpen(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	q := store.New(db)
	before, err := q.GetTodo(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}

	// MarkOpen on open todo is a no-op
	if err := svc.MarkOpen(ctx, created.Row.ID); err != nil {
		t.Fatalf("MarkOpen: %v", err)
	}

	after, err := q.GetTodo(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}
	if after.UpdatedAt != before.UpdatedAt {
		t.Errorf("updated_at changed from %q to %q", before.UpdatedAt, after.UpdatedAt)
	}
	if after.DoneAt != nil {
		t.Errorf("done_at = %v, want nil", after.DoneAt)
	}
}

func TestList_DefaultOpen(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: "open todo 1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "open todo 2"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Mark one done
	q := store.New(db)
	rows, err := q.ListTodos(ctx, store.ListTodosParams{Limit: 100, Offset: 0})
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 todos, got %d", len(rows))
	}
	if err := svc.MarkDone(ctx, rows[0].ID); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	// Default list should return both active todos (open + done)
	got, err := svc.List(ctx, todos.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestList_FilterDone(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	openTodo, err := svc.Create(ctx, todos.CreateInput{Title: "open"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	doneTodo, err := svc.Create(ctx, todos.CreateInput{Title: "done"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.MarkDone(ctx, doneTodo.Row.ID); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	_ = openTodo

	got, err := svc.List(ctx, todos.ListFilter{Status: todos.StatusDone})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != doneTodo.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, doneTodo.Row.ID)
	}
}

func TestList_FilterPriority(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	_, err := svc.Create(ctx, todos.CreateInput{Title: "low", Priority: todos.PriorityLow})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	highTodo, err := svc.Create(ctx, todos.CreateInput{Title: "high", Priority: todos.PriorityHigh})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{Priority: todos.PriorityHigh})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != highTodo.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, highTodo.Row.ID)
	}
}

func TestList_FilterCategory(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)
	q := store.New(db)

	cat, err := q.CreateCategory(ctx, store.CreateCategoryParams{Slug: "work", Name: "Work", Icon: "", SortOrder: 1})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	withCat, err := svc.Create(ctx, todos.CreateInput{Title: "with cat", CategoryID: &cat.ID})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "no cat"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{CategorySlug: "work"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != withCat.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, withCat.Row.ID)
	}
}

func TestList_FilterTag(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	withTag, err := svc.Create(ctx, todos.CreateInput{Title: "tagged", Tags: []string{"urgent"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "untagged"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{TagName: "urgent"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != withTag.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, withTag.Row.ID)
	}
}

func TestList_FilterOverdue(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	past := "2020-01-01"
	future := "2099-01-01"
	overdue, err := svc.Create(ctx, todos.CreateInput{Title: "overdue", DueDate: &past})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "future", DueDate: &future}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "no due"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Mark overdue done — it should not appear
	if err := svc.MarkDone(ctx, overdue.Row.ID); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	// Create another overdue that is open
	overdueOpen, err := svc.Create(ctx, todos.CreateInput{Title: "overdue open", DueDate: &past})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{OnlyOverdue: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != overdueOpen.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, overdueOpen.Row.ID)
	}
}

func TestList_FilterDueBefore(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	past := "2020-01-01"
	future := "2099-01-01"
	before, err := svc.Create(ctx, todos.CreateInput{Title: "before", DueDate: &past})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "after", DueDate: &future}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{DueBefore: "2025-01-01"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != before.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, before.Row.ID)
	}
}

func TestList_FilterTrashed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	trashed, err := svc.Create(ctx, todos.CreateInput{Title: "trashed"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, trashed.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "active"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{Trashed: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != trashed.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, trashed.Row.ID)
	}
}

func TestList_SortOrder(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	// Create todos with and without due dates
	d1 := "2026-06-01"
	d2 := "2026-06-02"
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "no due"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "due 2", DueDate: &d2}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "due 1", DueDate: &d1}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// Due dates ascending, then no due date
	wantOrder := []string{"due 1", "due 2", "no due"}
	for i, want := range wantOrder {
		if got[i].Row.Title != want {
			t.Errorf("[%d] title = %q, want %q", i, got[i].Row.Title, want)
		}
	}
}

func TestList_Pagination(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	for i := 0; i < 5; i++ {
		if _, err := svc.Create(ctx, todos.CreateInput{Title: "todo"}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	got, err := svc.List(ctx, todos.ListFilter{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestSearch_TitlePrefix(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: "pagar luz"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "otra cosa"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{Search: "pag"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.Title != "pagar luz" {
		t.Errorf("title = %q, want %q", got[0].Row.Title, "pagar luz")
	}
}

func TestSearch_Description(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: "t", Description: "monthly invoice payment"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{Search: "invoice"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
}

func TestSearch_Notes(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: "t", Notes: "rutina de mañana"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{Search: "rutina"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
}

func TestSearch_TagNames(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: "t", Tags: []string{"urgente"}}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{Search: "urgente"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
}

func TestSearch_SpecialCharsNoCrash(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	if _, err := svc.Create(ctx, todos.CreateInput{Title: "luz"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// These should not crash
	for _, q := range []string{"c++", "foo*bar", "(test)"} {
		if _, err := svc.List(ctx, todos.ListFilter{Search: q}); err != nil {
			t.Fatalf("List(%q): %v", q, err)
		}
	}
}

func TestSearch_ExcludesTrashed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := todos.New(db)

	trashed, err := svc.Create(ctx, todos.CreateInput{Title: "pagar luz"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, trashed.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "pagar luz otro"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, todos.ListFilter{Search: "pagar"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.Title != "pagar luz otro" {
		t.Errorf("title = %q, want %q", got[0].Row.Title, "pagar luz otro")
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	q := store.New(db)
	got, err := q.SearchTodos(ctx, "", 50)
	if err != nil {
		t.Fatalf("SearchTodos: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
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
