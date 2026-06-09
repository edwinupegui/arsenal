package web

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

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
