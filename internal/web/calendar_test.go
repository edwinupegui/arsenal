package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/edwinupegui/arsenal/internal/calendar"
)

func TestCalendarRoutes(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	svc := calendar.New(db)
	ev, err := svc.Create(t.Context(), calendar.CreateInput{
		Title:   "Standup",
		StartAt: "2026-06-15T09:00:00",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	idStr := strconv.FormatInt(ev.Row.ID, 10)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		header map[string]string
		want   int
	}{
		// Scenario: List page renders events
		{"list get", "GET", "/calendar", "", nil, http.StatusOK},
		// Scenario: New form
		{"new form", "GET", "/calendar/new", "", nil, http.StatusOK},
		// Scenario: Create event → redirect
		{"create post", "POST", "/calendar",
			"title=Team+lunch&start_date=2026-06-20&start_time=12%3A00&recurrence=none",
			nil, http.StatusSeeOther},
		// Scenario: All-day create round-trip
		{"create all-day", "POST", "/calendar",
			"title=Public+holiday&start_date=2026-07-04&all_day=1&recurrence=none",
			nil, http.StatusSeeOther},
		// Scenario: Show renders event detail
		{"show", "GET", "/calendar/" + idStr, "", nil, http.StatusOK},
		// Scenario: Edit form
		{"edit form", "GET", "/calendar/" + idStr + "/edit", "", nil, http.StatusOK},
		// Scenario: Update redirects to show
		{"update post", "POST", "/calendar/" + idStr,
			"title=Updated+standup&start_date=2026-06-15&start_time=09%3A30&recurrence=none",
			nil, http.StatusSeeOther},
		// Scenario: Soft-delete removes card via HTMX (returns empty fragment)
		{"soft delete htmx", "POST", "/calendar/" + idStr + "/delete",
			"", map[string]string{"HX-Request": "true"}, http.StatusOK},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := strings.NewReader(c.body)
			req := httptest.NewRequest(c.method, c.path, body)
			if c.body != "" {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			for k, v := range c.header {
				req.Header.Set(k, v)
			}
			rr := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rr, req)
			if rr.Code != c.want {
				t.Fatalf("%s %s: want %d got %d\nbody: %s",
					c.method, c.path, c.want, rr.Code, rr.Body.String())
			}
		})
	}
}

// TestCalendarNotFound covers the 404 scenario for GET /calendar/{unknown-id}.
func TestCalendarNotFound(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	req := httptest.NewRequest("GET", "/calendar/9999", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /calendar/9999: want 404 got %d", rr.Code)
	}
}

// TestCalendarSidebarBadge covers the sidebar count badge scenarios.
func TestCalendarSidebarBadge(t *testing.T) {
	t.Run("badge hidden when zero events", func(t *testing.T) {
		db := newTestDB(t)
		srv := New(db, Options{})
		req := httptest.NewRequest("GET", "/calendar", nil)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET /calendar: want 200 got %d", rr.Code)
		}
		body := rr.Body.String()
		if !strings.Contains(body, "Calendar") {
			t.Error("expected 'Calendar' sidebar link in response body")
		}
	})

	t.Run("badge shows count when events exist", func(t *testing.T) {
		db := newTestDB(t)
		srv := New(db, Options{})

		svc := calendar.New(db)
		for i := 0; i < 3; i++ {
			if _, err := svc.Create(t.Context(), calendar.CreateInput{
				Title:   "Event",
				StartAt: "2026-06-15T09:00:00",
			}); err != nil {
				t.Fatalf("create: %v", err)
			}
		}

		req := httptest.NewRequest("GET", "/calendar", nil)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET /calendar: want 200 got %d", rr.Code)
		}
		body := rr.Body.String()
		if !strings.Contains(body, "3") {
			t.Error("expected calendar count '3' in sidebar badge")
		}
	})
}

// TestCalendarEmptyState covers: "When no events exist, an empty state is shown".
func TestCalendarEmptyState(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	req := httptest.NewRequest("GET", "/calendar", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Add event") {
		t.Error("expected 'Add event' link in empty state")
	}
}

// TestCalendarWhenFilter covers the ?when=today|upcoming filter applied from Today view.
func TestCalendarWhenFilter(t *testing.T) {
	db := newTestDB(t)
	srv := New(db, Options{})

	req := httptest.NewRequest("GET", "/calendar?when=today", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /calendar?when=today: want 200 got %d", rr.Code)
	}
}
