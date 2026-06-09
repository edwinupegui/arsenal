package web

import (
	"testing"

	"github.com/edwinupegui/arsenal/internal/store"
)

func TestToTodoVM(t *testing.T) {
	t.Run("basic fields", func(t *testing.T) {
		row := store.Todo{
			ID:         1,
			Title:      "Test",
			Priority:   "high",
			Status:     "open",
			Recurrence: "none",
			CreatedAt:  "2024-01-01",
			UpdatedAt:  "2024-01-02",
		}
		vm := toTodoVM(row, nil, "", "")
		if vm.ID != 1 || vm.Title != "Test" || vm.Priority != "high" || vm.Status != "open" {
			t.Fatalf("unexpected basic fields: %+v", vm)
		}
	})

	t.Run("null handling", func(t *testing.T) {
		row := store.Todo{ID: 2, Title: "N", Priority: "med", Status: "done"}
		vm := toTodoVM(row, nil, "", "")
		if vm.Description != "" || vm.DueDate != "" || vm.DoneAt != "" || vm.Notes != "" {
			t.Fatalf("expected empty strings for null fields, got %+v", vm)
		}
	})

	t.Run("tag resolution", func(t *testing.T) {
		row := store.Todo{ID: 3, Title: "T", Priority: "low", Status: "open"}
		vm := toTodoVM(row, []string{"a", "b"}, "cat", "slug")
		if len(vm.Tags) != 2 || vm.Tags[0] != "a" || vm.Tags[1] != "b" {
			t.Fatalf("unexpected tags: %+v", vm.Tags)
		}
		if vm.CategoryName != "cat" || vm.CategorySlug != "slug" {
			t.Fatalf("unexpected category: %s / %s", vm.CategoryName, vm.CategorySlug)
		}
	})

	t.Run("trashed flag", func(t *testing.T) {
		deleted := "2024-01-03"
		row := store.Todo{ID: 4, Title: "X", DeletedAt: &deleted}
		vm := toTodoVM(row, nil, "", "")
		if !vm.Trashed || vm.DeletedAt != deleted {
			t.Fatalf("expected trashed=true, got %+v", vm)
		}
	})

	t.Run("recurrence passthrough", func(t *testing.T) {
		row := store.Todo{ID: 5, Title: "R", Recurrence: "daily"}
		vm := toTodoVM(row, nil, "", "")
		if vm.Recurrence != "daily" {
			t.Fatalf("unexpected recurrence: %s", vm.Recurrence)
		}
	})
}
