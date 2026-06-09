package providers_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/today/providers"
	"github.com/edwinupegui/arsenal/internal/todos"
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

func todayStr() string   { return time.Now().UTC().Format("2006-01-02") }
func tomorrowStr() string { return time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02") }
func weekLaterStr() string {
	return time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
}
func yesterdayStr() string {
	return time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
}

func createTodo(t *testing.T, svc *todos.Service, title, dueDate, status string) *todos.Todo {
	t.Helper()
	in := todos.CreateInput{
		Title:    title,
		Priority: todos.PriorityHigh,
		DueDate:  &dueDate,
	}
	ctx := context.Background()
	created, err := svc.Create(ctx, in)
	if err != nil {
		t.Fatalf("create todo %q: %v", title, err)
	}
	if status == "done" {
		if err := svc.MarkDone(ctx, created.Row.ID); err != nil {
			t.Fatalf("mark done: %v", err)
		}
		created.Row.Status = "done"
	}
	return created
}

func TestTodosProvider_OverdueSection(t *testing.T) {
	db := newTestDB(t)
	todoSvc := todos.New(db)
	createTodo(t, todoSvc, "old task", yesterdayStr(), "open")

	p := providers.NewTodosProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if len(secs) != 1 {
		t.Fatalf("expected 1 section, got %d", len(secs))
	}
	if secs[0].Key != "overdue" {
		t.Errorf("key = %q, want overdue", secs[0].Key)
	}
	if len(secs[0].Items) != 1 {
		t.Errorf("items = %d, want 1", len(secs[0].Items))
	}
}

func TestTodosProvider_DueTodaySection(t *testing.T) {
	db := newTestDB(t)
	todoSvc := todos.New(db)
	createTodo(t, todoSvc, "today task", todayStr(), "open")

	p := providers.NewTodosProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	var found bool
	for _, s := range secs {
		if s.Key == "due-today" {
			found = true
			if len(s.Items) != 1 {
				t.Errorf("due-today items = %d, want 1", len(s.Items))
			}
		}
	}
	if !found {
		t.Error("expected due-today section")
	}
}

func TestTodosProvider_UpcomingSection(t *testing.T) {
	db := newTestDB(t)
	todoSvc := todos.New(db)
	createTodo(t, todoSvc, "upcoming task", tomorrowStr(), "open")

	p := providers.NewTodosProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	var found bool
	for _, s := range secs {
		if s.Key == "upcoming" {
			found = true
			if len(s.Items) != 1 {
				t.Errorf("upcoming items = %d, want 1", len(s.Items))
			}
		}
	}
	if !found {
		t.Error("expected upcoming section")
	}
}

func TestTodosProvider_OmitsEmptySections(t *testing.T) {
	db := newTestDB(t)
	todoSvc := todos.New(db)
	// Only overdue
	createTodo(t, todoSvc, "old task", yesterdayStr(), "open")

	p := providers.NewTodosProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	for _, s := range secs {
		if s.Key == "due-today" || s.Key == "upcoming" {
			t.Errorf("unexpected empty section %q", s.Key)
		}
	}
}

func TestTodosProvider_ExcludesDoneAndDeleted(t *testing.T) {
	db := newTestDB(t)
	todoSvc := todos.New(db)
	createTodo(t, todoSvc, "done overdue", yesterdayStr(), "done")
	// soft delete
	openDeleted := createTodo(t, todoSvc, "deleted overdue", yesterdayStr(), "open")
	if err := todoSvc.SoftDelete(context.Background(), openDeleted.Row.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	createTodo(t, todoSvc, "real overdue", yesterdayStr(), "open")

	p := providers.NewTodosProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if len(secs) != 1 || len(secs[0].Items) != 1 {
		t.Fatalf("expected 1 overdue item, got %+v", secs)
	}
	if secs[0].Items[0].Title != "real overdue" {
		t.Errorf("title = %q, want real overdue", secs[0].Items[0].Title)
	}
}

func TestTodosProvider_ItemMappingIncludesURL(t *testing.T) {
	db := newTestDB(t)
	todoSvc := todos.New(db)
	created := createTodo(t, todoSvc, "pay rent", yesterdayStr(), "open")

	p := providers.NewTodosProvider(db)
	secs, err := p.Sections(context.Background())
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if len(secs) == 0 || len(secs[0].Items) == 0 {
		t.Fatal("expected at least one item")
	}
	it := secs[0].Items[0]
	if it.Domain != "todos" {
		t.Errorf("domain = %q, want todos", it.Domain)
	}
	if it.Title != "pay rent" {
		t.Errorf("title = %q, want pay rent", it.Title)
	}
	wantURL := fmt.Sprintf("/todos/%d", created.Row.ID)
	if it.URL != wantURL {
		t.Errorf("url = %q, want %s", it.URL, wantURL)
	}
	if it.Priority != "high" {
		t.Errorf("priority = %q, want high", it.Priority)
	}
}

func TestTodosProvider_ImplementsProvider(t *testing.T) {
	var _ today.Provider = providers.NewTodosProvider(nil)
}
