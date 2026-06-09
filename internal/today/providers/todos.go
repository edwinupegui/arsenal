package providers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
)

// TodosProvider contributes overdue, due-today, and upcoming todo sections.
type TodosProvider struct {
	queries *store.Queries
}

// NewTodosProvider builds a TodosProvider backed by db.
func NewTodosProvider(db *sql.DB) *TodosProvider {
	return &TodosProvider{queries: store.New(db)}
}

// Name returns the provider identifier.
func (p *TodosProvider) Name() string { return "todos" }

// Sections returns up to three sections: overdue, due-today, upcoming.
func (p *TodosProvider) Sections(ctx context.Context) ([]today.Section, error) {
	now := time.Now().UTC()
	todayStr := now.Format("2006-01-02")
	tomorrowStr := now.AddDate(0, 0, 1).Format("2006-01-02")
	weekLaterStr := now.AddDate(0, 0, 7).Format("2006-01-02")

	var sections []today.Section

	// Overdue
	overdueRows, err := p.queries.ListTodosFiltered(ctx, store.TodoListFilter{
		OnlyOverdue: true,
		Today:       todayStr,
		Status:      "open",
		Limit:       5,
	})
	if err != nil {
		return nil, fmt.Errorf("overdue query: %w", err)
	}
	if items := mapTodoItems(overdueRows); len(items) > 0 {
		sections = append(sections, today.Section{
			Key:     "overdue",
			Title:   "Overdue",
			Items:   items,
			IsEmpty: false,
		})
	}

	// Due Today
	dueTodayRows, err := p.queries.ListTodosFiltered(ctx, store.TodoListFilter{
		DueBefore: tomorrowStr,
		Status:    "open",
		Limit:     50,
	})
	if err != nil {
		return nil, fmt.Errorf("due-today query: %w", err)
	}
	var dueTodayItems []today.Item
	for _, row := range dueTodayRows {
		if row.Todo.DueDate != nil && *row.Todo.DueDate == todayStr {
			dueTodayItems = append(dueTodayItems, mapTodoItem(row))
		}
	}
	if len(dueTodayItems) > 5 {
		dueTodayItems = dueTodayItems[:5]
	}
	if len(dueTodayItems) > 0 {
		sections = append(sections, today.Section{
			Key:     "due-today",
			Title:   "Due Today",
			Items:   dueTodayItems,
			IsEmpty: false,
		})
	}

	// Upcoming
	upcomingRows, err := p.queries.ListTodosFiltered(ctx, store.TodoListFilter{
		Status: "open",
		Limit:  50,
	})
	if err != nil {
		return nil, fmt.Errorf("upcoming query: %w", err)
	}
	var upcomingItems []today.Item
	for _, row := range upcomingRows {
		if row.Todo.DueDate != nil && *row.Todo.DueDate >= tomorrowStr && *row.Todo.DueDate <= weekLaterStr {
			upcomingItems = append(upcomingItems, mapTodoItem(row))
		}
	}
	if len(upcomingItems) > 5 {
		upcomingItems = upcomingItems[:5]
	}
	if len(upcomingItems) > 0 {
		sections = append(sections, today.Section{
			Key:     "upcoming",
			Title:   "Upcoming",
			Items:   upcomingItems,
			IsEmpty: false,
		})
	}

	return sections, nil
}

func mapTodoItem(row store.ListedTodo) today.Item {
	subtitle := ""
	if row.Todo.DueDate != nil {
		subtitle = *row.Todo.DueDate
	}
	return today.Item{
		Domain:   "todos",
		ID:       row.Todo.ID,
		Title:    row.Todo.Title,
		Subtitle: subtitle,
		Priority: row.Todo.Priority,
		Tags:     row.Tags,
		URL:      fmt.Sprintf("/todos/%d", row.Todo.ID),
	}
}

func mapTodoItems(rows []store.ListedTodo) []today.Item {
	out := make([]today.Item, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapTodoItem(row))
	}
	return out
}
