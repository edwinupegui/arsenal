package cli

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/edwinupegui/arsenal/internal/today"
)

// fakeSection builds a today.Section with the given items for rendering tests.
// Tests don't need a real DB; they exercise the pure output helpers.
func fakeSection(key, title string, items ...today.Item) today.Section {
	return today.Section{
		Key:   key,
		Title: title,
		Items: items,
	}
}

func fakeItem(domain string, id int64, title, subtitle, priority string, tags ...string) today.Item {
	return today.Item{
		Domain:   domain,
		ID:       id,
		Title:    title,
		Subtitle: subtitle,
		Priority: priority,
		Tags:     tags,
		URL:      "/" + domain + "/" + strconv.FormatInt(id, 10),
	}
}

func TestWriteTodayTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	writeTodayTable(&buf, nil)
	got := strings.TrimSpace(buf.String())
	if got != "(nothing today)" {
		t.Errorf("empty sections: got %q, want %q", got, "(nothing today)")
	}
}

func TestWriteTodayTable_MultipleSections(t *testing.T) {
	sections := []today.Section{
		fakeSection("overdue", "Overdue",
			fakeItem("todos", 1, "Fix bug", "2026-06-10", "high", "bugfix", "urgent"),
		),
		fakeSection("due-today", "Due Today",
			fakeItem("todos", 2, "Write spec", "2026-06-11", "med"),
		),
		fakeSection("recent", "Recent Resources",
			fakeItem("resources", 7, "Some video", "video", ""),
		),
	}
	var buf bytes.Buffer
	writeTodayTable(&buf, sections)
	out := buf.String()

	// Each section header should appear
	for _, want := range []string{"Overdue", "Due Today", "Recent Resources"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing section header %q in:\n%s", want, out)
		}
	}
	// Each item title should appear
	for _, want := range []string{"Fix bug", "Write spec", "Some video"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing item title %q in:\n%s", want, out)
		}
	}
	// Domain indicator should appear
	if !strings.Contains(out, "[todos]") {
		t.Errorf("missing domain tag [todos] in:\n%s", out)
	}
	if !strings.Contains(out, "[resources]") {
		t.Errorf("missing domain tag [resources] in:\n%s", out)
	}
	// Priority should appear
	if !strings.Contains(out, "high") {
		t.Errorf("missing priority in:\n%s", out)
	}
}

func TestWriteTodayTable_ShowAllURLAppearsWhenSet(t *testing.T) {
	sections := []today.Section{
		{
			Key:        "overdue",
			Title:      "Overdue",
			Items:      []today.Item{fakeItem("todos", 1, "X", "2026-06-10", "high")},
			ShowAllURL: "/todos?status=open&overdue=true",
		},
	}
	var buf bytes.Buffer
	writeTodayTable(&buf, sections)
	out := buf.String()
	if !strings.Contains(out, "show all") || !strings.Contains(out, "/todos?status=open&overdue=true") {
		t.Errorf("expected 'show all' link to %q in:\n%s", "/todos?status=open&overdue=true", out)
	}
}

func TestWriteTodayTable_NoShowAllWhenEmpty(t *testing.T) {
	sections := []today.Section{
		{
			Key:        "overdue",
			Title:      "Overdue",
			Items:      []today.Item{fakeItem("todos", 1, "X", "2026-06-10", "high")},
			ShowAllURL: "", // ≤ 5 items, no overflow link
		},
	}
	var buf bytes.Buffer
	writeTodayTable(&buf, sections)
	out := buf.String()
	if strings.Contains(out, "show all") {
		t.Errorf("did not expect 'show all' link when ShowAllURL is empty:\n%s", out)
	}
}

func TestWriteTodayJSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := writeTodayJSON(&buf, nil); err != nil {
		t.Fatalf("writeTodayJSON: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v\nraw: %s", err, buf.String())
	}
	if len(got) != 0 {
		t.Errorf("expected empty array, got %d items", len(got))
	}
}

func TestWriteTodayJSON_MultipleSections(t *testing.T) {
	sections := []today.Section{
		fakeSection("overdue", "Overdue",
			fakeItem("todos", 1, "Fix bug", "2026-06-10", "high", "bugfix"),
		),
	}
	var buf bytes.Buffer
	if err := writeTodayJSON(&buf, sections); err != nil {
		t.Fatalf("writeTodayJSON: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v\nraw: %s", err, buf.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 section, got %d", len(got))
	}
	sec := got[0]
	if sec["key"] != "overdue" {
		t.Errorf("key: got %v, want overdue", sec["key"])
	}
	if sec["title"] != "Overdue" {
		t.Errorf("title: got %v, want Overdue", sec["title"])
	}
	items, ok := sec["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items: got %v, want 1 item", sec["items"])
	}
	item := items[0].(map[string]any)
	if item["domain"] != "todos" {
		t.Errorf("item.domain: got %v, want todos", item["domain"])
	}
	if item["title"] != "Fix bug" {
		t.Errorf("item.title: got %v", item["title"])
	}
	tags, _ := item["tags"].([]any)
	if len(tags) != 1 || tags[0] != "bugfix" {
		t.Errorf("item.tags: got %v, want [bugfix]", item["tags"])
	}
}

func TestWriteTodayJSON_ShowAllURLPreserved(t *testing.T) {
	sections := []today.Section{
		{
			Key:        "overdue",
			Title:      "Overdue",
			Items:      []today.Item{fakeItem("todos", 1, "X", "2026-06-10", "high")},
			ShowAllURL: "/todos?status=open&overdue=true",
		},
	}
	var buf bytes.Buffer
	if err := writeTodayJSON(&buf, sections); err != nil {
		t.Fatalf("writeTodayJSON: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got[0]["show_all_url"] != "/todos?status=open&overdue=true" {
		t.Errorf("show_all_url: got %v", got[0]["show_all_url"])
	}
}

func TestNewTodayCmd_Metadata(t *testing.T) {
	cmd := newTodayCmd()
	if cmd.Use != "today" {
		t.Errorf("Use: got %q, want today", cmd.Use)
	}
	if !strings.Contains(cmd.Short, "Today") {
		t.Errorf("Short: got %q, want to mention 'Today'", cmd.Short)
	}
	// --json flag must be registered
	flag := cmd.Flags().Lookup("json")
	if flag == nil {
		t.Error("--json flag not registered")
	}
}
