package web

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/todos"
)

func newTestDB(t *testing.T) *sql.DB {
	// Reuse the same test DB pattern from resources tests
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestTodoRoutes(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	// Create a todo first
	svc := todos.New(db)
	todo, err := svc.Create(t.Context(), todos.CreateInput{Title: "Test todo", Priority: todos.PriorityHigh})
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"list get", "GET", "/todos", "", http.StatusOK},
		{"new form", "GET", "/todos/new", "", http.StatusOK},
		{"create post", "POST", "/todos", "title=New+todo&priority=med", http.StatusSeeOther},
		{"show", "GET", "/todos/" + strconv.FormatInt(todo.Row.ID, 10), "", http.StatusOK},
		{"edit form", "GET", "/todos/" + strconv.FormatInt(todo.Row.ID, 10) + "/edit", "", http.StatusOK},
		{"update post", "POST", "/todos/" + strconv.FormatInt(todo.Row.ID, 10), "title=Updated&priority=med", http.StatusSeeOther},
		{"mark done", "POST", "/todos/" + strconv.FormatInt(todo.Row.ID, 10) + "/done", "", http.StatusOK},
		{"mark open", "POST", "/todos/" + strconv.FormatInt(todo.Row.ID, 10) + "/open", "", http.StatusOK},
		{"soft delete", "POST", "/todos/" + strconv.FormatInt(todo.Row.ID, 10) + "/delete", "", http.StatusSeeOther},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var body *strings.Reader
			if c.body != "" {
				body = strings.NewReader(c.body)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(c.method, c.path, body)
			if c.body != "" {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			rr := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rr, req)
			if rr.Code != c.want {
				t.Fatalf("%s %s: want %d, got %d", c.method, c.path, c.want, rr.Code)
			}
		})
	}
}

func TestOverdueBadge_Timezone(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	cs := configstore.New(db)
	if err := cs.Set(ctx, config.KeyUserTimezone, "America/Argentina/Buenos_Aires"); err != nil {
		t.Fatalf("Set timezone: %v", err)
	}

	svc := todos.New(db)
	borderline := "2026-06-11"
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "borderline", DueDate: &borderline}); err != nil {
		t.Fatalf("Create borderline: %v", err)
	}

	// 2026-06-12 02:00 UTC == 2026-06-11 23:00 in America/Argentina/Buenos_Aires.
	fixed := time.Date(2026, 6, 12, 2, 0, 0, 0, time.UTC)
	got, err := countOverdueTodos(ctx, db, fixed)
	if err != nil {
		t.Fatalf("countOverdueTodos: %v", err)
	}
	if got != 0 {
		t.Errorf("count = %d, want 0 (borderline todo is due-today in America/Argentina/Buenos_Aires)", got)
	}

	// A day earlier is still overdue regardless of timezone.
	overdue := "2026-06-10"
	if _, err := svc.Create(ctx, todos.CreateInput{Title: "overdue", DueDate: &overdue}); err != nil {
		t.Fatalf("Create overdue: %v", err)
	}
	got, err = countOverdueTodos(ctx, db, fixed)
	if err != nil {
		t.Fatalf("countOverdueTodos: %v", err)
	}
	if got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
}
