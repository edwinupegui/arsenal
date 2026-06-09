package today

import (
	"strings"
	"testing"
)

func TestIsEmptyPage_AllEmpty(t *testing.T) {
	sections := []Section{
		{Key: "overdue", Items: []Item{}},
		{Key: "recent", Items: []Item{}},
	}
	if !IsEmptyPage(sections) {
		t.Error("expected IsEmptyPage true when all sections empty")
	}
}

func TestIsEmptyPage_PartialData(t *testing.T) {
	sections := []Section{
		{Key: "overdue", Items: []Item{}},
		{Key: "recent", Items: []Item{{ID: 1, Title: "x"}}},
	}
	if IsEmptyPage(sections) {
		t.Error("expected IsEmptyPage false when at least one section has items")
	}
}

func TestRenderEmptyState_TUI(t *testing.T) {
	out := RenderEmptyState("tui")
	if !strings.Contains(out, "Nothing on your plate today") {
		t.Errorf("expected message in TUI empty state, got: %q", out)
	}
	if !strings.Contains(out, "n") {
		t.Error("expected 'n' hint in TUI empty state")
	}
	if !strings.Contains(out, "2") {
		t.Error("expected '2' hint in TUI empty state")
	}
}

func TestRenderEmptyState_Web(t *testing.T) {
	out := RenderEmptyState("web")
	if !strings.Contains(out, "Nothing on your plate today") {
		t.Errorf("expected message in web empty state, got: %q", out)
	}
	if !strings.Contains(out, "Add a todo") {
		t.Error("expected 'Add a todo' link in web empty state")
	}
	if !strings.Contains(out, "Browse resources") {
		t.Error("expected 'Browse resources' link in web empty state")
	}
}
