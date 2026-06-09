package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/todos"
)

func TestTodayPage_Renders(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	req := httptest.NewRequest("GET", "/today", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /today: want %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestTodayPage_ShowsAllSections(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	// Seed overdue todo
	todoSvc := todos.New(db)
	_, err := todoSvc.Create(t.Context(), todos.CreateInput{
		Title:    "Overdue todo",
		Priority: todos.PriorityHigh,
		DueDate:  strPtr("2020-01-01"),
	})
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}

	// Seed resource
	resSvc := resources.New(db)
	_, err = resSvc.Create(t.Context(), resources.CreateInput{
		Title:    "Recent resource",
		URL:      "https://example.com",
		Type:     domain.TypeArticle,
		Language: domain.LangEN,
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	req := httptest.NewRequest("GET", "/today", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /today: want %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Overdue") {
		t.Errorf("expected body to contain 'Overdue', got: %s", body)
	}
	if !strings.Contains(body, "Recent Resources") {
		t.Errorf("expected body to contain 'Recent Resources', got: %s", body)
	}
}

func TestTodayPage_EmptyState(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	req := httptest.NewRequest("GET", "/today", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /today: want %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Nothing on your plate today") {
		t.Errorf("expected empty state message, got: %s", body)
	}
	if !strings.Contains(body, "Add a todo") {
		t.Errorf("expected empty state to include 'Add a todo' link, got: %s", body)
	}
}

func TestTodayPage_SectionOrdering(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	// Seed overdue todo
	todoSvc := todos.New(db)
	_, err := todoSvc.Create(t.Context(), todos.CreateInput{
		Title:    "Overdue todo",
		Priority: todos.PriorityHigh,
		DueDate:  strPtr("2020-01-01"),
	})
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}

	// Seed resource
	resSvc := resources.New(db)
	_, err = resSvc.Create(t.Context(), resources.CreateInput{
		Title:    "Recent resource",
		URL:      "https://example.com",
		Type:     domain.TypeArticle,
		Language: domain.LangEN,
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	req := httptest.NewRequest("GET", "/today", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	body := rr.Body.String()

	// Verify ordering: Overdue should appear before Recent Resources
	overdueIdx := strings.Index(body, "Overdue")
	recentIdx := strings.Index(body, "Recent Resources")
	if overdueIdx == -1 || recentIdx == -1 {
		t.Fatalf("expected both sections in body")
	}
	if overdueIdx > recentIdx {
		t.Errorf("expected Overdue before Recent Resources, got Overdue at %d, Recent at %d", overdueIdx, recentIdx)
	}
}

func TestTodayPage_SectionIDs(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	// Seed resource
	resSvc := resources.New(db)
	_, err := resSvc.Create(t.Context(), resources.CreateInput{
		Title:    "Recent resource",
		URL:      "https://example.com",
		Type:     domain.TypeArticle,
		Language: domain.LangEN,
	})
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	req := httptest.NewRequest("GET", "/today", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `id="section-recent"`) {
		t.Errorf("expected section to have id attribute, got: %s", body)
	}
	if !strings.Contains(body, `id="today-item-`) {
		t.Errorf("expected item to have id attribute, got: %s", body)
	}
}

func TestSidebar_TodayEntryWithBadge(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	// Seed 2 overdue todos
	todoSvc := todos.New(db)
	for i := 0; i < 2; i++ {
		_, err := todoSvc.Create(t.Context(), todos.CreateInput{
			Title:    "Overdue todo",
			Priority: todos.PriorityHigh,
			DueDate:  strPtr("2020-01-01"),
		})
		if err != nil {
			t.Fatalf("create todo: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/resources", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Today") {
		t.Errorf("expected sidebar to contain 'Today' entry, got: %s", body)
	}
	if !strings.Contains(body, "2</span>") {
		t.Errorf("expected sidebar badge to show '2', got: %s", body)
	}
}

func TestSidebar_BadgeHiddenWhenZero(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	req := httptest.NewRequest("GET", "/resources", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Today") {
		t.Errorf("expected sidebar to contain 'Today' entry, got: %s", body)
	}
	// The badge should not appear when overdue count is 0
	// Check that the specific pattern with "Today" followed by a count badge is absent
	if strings.Contains(body, "Today</span>\n          <span class=\"sidebar-link-count\">0</span>") {
		t.Errorf("expected badge to be hidden when zero, got: %s", body)
	}
}

func TestMarkTodoDone_UpdatesTodayBadgeViaOOB(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	// Seed 2 overdue todos
	todoSvc := todos.New(db)
	for i := 0; i < 2; i++ {
		_, err := todoSvc.Create(t.Context(), todos.CreateInput{
			Title:    "Overdue todo",
			Priority: todos.PriorityHigh,
			DueDate:  strPtr("2020-01-01"),
		})
		if err != nil {
			t.Fatalf("create todo: %v", err)
		}
	}

	// Get the ID of the first todo
	rows, err := todoSvc.List(t.Context(), todos.ListFilter{Status: todos.StatusOpen, Limit: 10})
	if err != nil {
		t.Fatalf("list todos: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one todo")
	}
	id := rows[0].Row.ID

	req := httptest.NewRequest("POST", "/todos/"+strconv.FormatInt(id, 10)+"/done", nil)
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /todos/%d/done: want %d, got %d", id, http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Today") {
		t.Errorf("expected OOB response to contain 'Today' entry, got: %s", body)
	}
	// After marking one done, the badge should show 1
	if !strings.Contains(body, "1</span>") {
		t.Errorf("expected OOB badge to show '1', got: %s", body)
	}
}

func TestMarkTodoOpen_UpdatesTodayBadgeViaOOB(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	// Seed 1 overdue todo
	todoSvc := todos.New(db)
	_, err := todoSvc.Create(t.Context(), todos.CreateInput{
		Title:    "Overdue todo",
		Priority: todos.PriorityHigh,
		DueDate:  strPtr("2020-01-01"),
	})
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}

	// Get the ID
	rows, err := todoSvc.List(t.Context(), todos.ListFilter{Status: todos.StatusOpen, Limit: 10})
	if err != nil {
		t.Fatalf("list todos: %v", err)
	}
	id := rows[0].Row.ID

	// Mark done first
	if err := todoSvc.MarkDone(t.Context(), id); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	// Now mark open via HTMX
	req := httptest.NewRequest("POST", "/todos/"+strconv.FormatInt(id, 10)+"/open", nil)
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /todos/%d/open: want %d, got %d", id, http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Today") {
		t.Errorf("expected OOB response to contain 'Today' entry, got: %s", body)
	}
	// After marking open, the badge should show 1
	if !strings.Contains(body, "1</span>") {
		t.Errorf("expected OOB badge to show '1', got: %s", body)
	}
}
