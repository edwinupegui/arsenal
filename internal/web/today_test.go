package web

import (
	"net/http"
	"net/http/httptest"
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
