package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ListFilter narrows the result set returned by ListResourcesFiltered.
// All fields are optional. The zero value lists every active (non-trashed)
// resource ordered by created_at DESC.
type ListFilter struct {
	CategorySlug string // empty = no filter; matches categories.slug COLLATE NOCASE
	TagName      string // empty = no filter; matches tags.name COLLATE NOCASE
	Type         string // empty = no filter; exact match on resources.type
	Language     string // empty = no filter; exact match on resources.language
	OnlyFavorite bool   // true = include only favorite=1
	Trashed      bool   // false = exclude trashed (default), true = ONLY trashed

	// Limit caps the number of rows. <=0 means default (50).
	Limit int
	// Offset for pagination; ignored when <=0.
	Offset int
}

// ListedResource bundles a resource row with its resolved category name and
// tag list, so the CLI can render a complete table without N+1 lookups.
type ListedResource struct {
	Resource     Resource
	CategoryName sql.NullString
	CategorySlug sql.NullString
	Tags         []string
}

// ListResourcesFiltered runs a dynamic SQL query against the destination DB
// using filter. Tags are aggregated in-query via GROUP_CONCAT to avoid one
// follow-up roundtrip per row.
func (q *Queries) ListResourcesFiltered(ctx context.Context, filter ListFilter) ([]ListedResource, error) {
	const base = `
SELECT r.id, r.title, r.url, r.description, r.type, r.language,
       r.category_id, r.notes, r.favorite, r.created_at, r.updated_at, r.deleted_at,
       c.name AS category_name, c.slug AS category_slug,
       (
         SELECT COALESCE(GROUP_CONCAT(t.name, ','), '')
         FROM resource_tags rt
         JOIN tags t ON t.id = rt.tag_id
         WHERE rt.resource_id = r.id
       ) AS tag_csv
FROM resources r
LEFT JOIN categories c ON c.id = r.category_id`

	conds := []string{}
	args := []any{}

	if filter.Trashed {
		conds = append(conds, "r.deleted_at IS NOT NULL")
	} else {
		conds = append(conds, "r.deleted_at IS NULL")
	}
	if filter.CategorySlug != "" {
		conds = append(conds, "c.slug = ? COLLATE NOCASE")
		args = append(args, filter.CategorySlug)
	}
	if filter.Type != "" {
		conds = append(conds, "r.type = ?")
		args = append(args, filter.Type)
	}
	if filter.Language != "" {
		conds = append(conds, "r.language = ?")
		args = append(args, filter.Language)
	}
	if filter.OnlyFavorite {
		conds = append(conds, "r.favorite = 1")
	}
	if filter.TagName != "" {
		conds = append(conds, `EXISTS (
			SELECT 1 FROM resource_tags rt
			JOIN tags t ON t.id = rt.tag_id
			WHERE rt.resource_id = r.id AND t.name = ? COLLATE NOCASE
		)`)
		args = append(args, filter.TagName)
	}

	q1 := base
	if len(conds) > 0 {
		q1 += "\nWHERE " + strings.Join(conds, " AND ")
	}
	q1 += "\nORDER BY r.created_at DESC, r.id DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	q1 += fmt.Sprintf("\nLIMIT %d", limit)
	if filter.Offset > 0 {
		q1 += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := q.db.QueryContext(ctx, q1, args...)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()

	var out []ListedResource
	for rows.Next() {
		var lr ListedResource
		var tagCSV string
		if err := rows.Scan(
			&lr.Resource.ID, &lr.Resource.Title, &lr.Resource.Url,
			&lr.Resource.Description, &lr.Resource.Type, &lr.Resource.Language,
			&lr.Resource.CategoryID, &lr.Resource.Notes, &lr.Resource.Favorite,
			&lr.Resource.CreatedAt, &lr.Resource.UpdatedAt, &lr.Resource.DeletedAt,
			&lr.CategoryName, &lr.CategorySlug,
			&tagCSV,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if tagCSV != "" {
			lr.Tags = strings.Split(tagCSV, ",")
		}
		out = append(out, lr)
	}
	return out, rows.Err()
}

// TodoListFilter drives the dynamic listing query for todos.
type TodoListFilter struct {
	CategorySlug string
	TagName      string
	Status       string
	Priority     string
	OnlyOverdue  bool
	Today        string // ISO-8601 date used when OnlyOverdue is true
	DueBefore    string
	DueAfter     string // ISO-8601 date; rows with due_date < DueAfter are excluded
	Trashed      bool
	Limit        int
	Offset       int
}

// ListedTodo bundles a todo row with its resolved category name and tag list.
type ListedTodo struct {
	Todo         Todo
	CategoryName sql.NullString
	CategorySlug sql.NullString
	Tags         []string
}

// FinanceListFilter drives the dynamic listing query for finance transactions.
type FinanceListFilter struct {
	From         *string // ISO date lower bound (inclusive)
	To           *string // ISO date upper bound (inclusive)
	Kind         *string // exact match
	CategorySlug string  // empty = no filter
	TagName      string  // empty = no filter
	Trashed      bool
	Limit        int
	Offset       int
}

// ListedFinance bundles a finance transaction row with its resolved category
// name/slug and tag list.
type ListedFinance struct {
	Finance      FinanceTransaction
	CategoryName sql.NullString
	CategorySlug sql.NullString
	Tags         []string
}

// ListFinanceFiltered runs a dynamic SQL query against the destination DB using
// filter. Tags are aggregated in-query via GROUP_CONCAT.
func (q *Queries) ListFinanceFiltered(ctx context.Context, filter FinanceListFilter) ([]ListedFinance, error) {
	const base = `
SELECT f.id, f.date, f.amount, f.kind, f.account, f.category_id,
       f.notes, f.recurrence, f.currency, f.created_at, f.updated_at, f.deleted_at,
       c.name AS category_name, c.slug AS category_slug,
       (
         SELECT COALESCE(GROUP_CONCAT(tag.name, ','), '')
         FROM finance_tags ft
         JOIN tags tag ON tag.id = ft.tag_id
         WHERE ft.finance_id = f.id
       ) AS tag_csv
FROM finance_transactions f
LEFT JOIN categories c ON c.id = f.category_id`

	conds := []string{}
	args := []any{}

	if filter.Trashed {
		conds = append(conds, "f.deleted_at IS NOT NULL")
	} else {
		conds = append(conds, "f.deleted_at IS NULL")
	}
	if filter.From != nil && *filter.From != "" {
		conds = append(conds, "f.date >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil && *filter.To != "" {
		conds = append(conds, "f.date <= ?")
		args = append(args, *filter.To)
	}
	if filter.Kind != nil && *filter.Kind != "" {
		conds = append(conds, "f.kind = ?")
		args = append(args, *filter.Kind)
	}
	if filter.CategorySlug != "" {
		conds = append(conds, "c.slug = ? COLLATE NOCASE")
		args = append(args, filter.CategorySlug)
	}
	if filter.TagName != "" {
		conds = append(conds, `EXISTS (
			SELECT 1 FROM finance_tags ft
			JOIN tags tg ON tg.id = ft.tag_id
			WHERE ft.finance_id = f.id AND tg.name = ? COLLATE NOCASE
		)`)
		args = append(args, filter.TagName)
	}

	q1 := base
	if len(conds) > 0 {
		q1 += "\nWHERE " + strings.Join(conds, " AND ")
	}
	q1 += "\nORDER BY f.date DESC, f.created_at DESC, f.id DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	q1 += fmt.Sprintf("\nLIMIT %d", limit)
	if filter.Offset > 0 {
		q1 += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := q.db.QueryContext(ctx, q1, args...)
	if err != nil {
		return nil, fmt.Errorf("list finance: %w", err)
	}
	defer rows.Close()

	var out []ListedFinance
	for rows.Next() {
		var lf ListedFinance
		var tagCSV string
		if err := rows.Scan(
			&lf.Finance.ID, &lf.Finance.Date, &lf.Finance.Amount, &lf.Finance.Kind,
			&lf.Finance.Account, &lf.Finance.CategoryID, &lf.Finance.Notes,
			&lf.Finance.Recurrence, &lf.Finance.Currency, &lf.Finance.CreatedAt,
			&lf.Finance.UpdatedAt, &lf.Finance.DeletedAt,
			&lf.CategoryName, &lf.CategorySlug,
			&tagCSV,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if tagCSV != "" {
			lf.Tags = strings.Split(tagCSV, ",")
		}
		out = append(out, lf)
	}
	return out, rows.Err()
}

// CalendarListFilter drives the dynamic listing query for calendar events.
type CalendarListFilter struct {
	From         *string // start_at lower bound (inclusive)
	To           *string // start_at upper bound (inclusive)
	Recurrence   *string // exact match
	CategorySlug string  // empty = no filter
	TagName      string  // empty = no filter
	Trashed      bool
	Limit        int
	Offset       int
}

// ListedCalendar bundles a calendar event row with its resolved category
// name/slug and tag list.
type ListedCalendar struct {
	Calendar     CalendarEvent
	CategoryName sql.NullString
	CategorySlug sql.NullString
	Tags         []string
}

// ListCalendarFiltered runs a dynamic SQL query against the destination DB using
// filter. Tags are aggregated in-query via GROUP_CONCAT.
func (q *Queries) ListCalendarFiltered(ctx context.Context, filter CalendarListFilter) ([]ListedCalendar, error) {
	const base = `
SELECT e.id, e.title, e.description, e.start_at, e.end_at, e.all_day, e.location,
       e.category_id, e.notes, e.recurrence, e.created_at, e.updated_at, e.deleted_at,
       c.name AS category_name, c.slug AS category_slug,
       (
         SELECT COALESCE(GROUP_CONCAT(tag.name, ','), '')
         FROM calendar_tags ct
         JOIN tags tag ON tag.id = ct.tag_id
         WHERE ct.event_id = e.id
       ) AS tag_csv
FROM calendar_events e
LEFT JOIN categories c ON c.id = e.category_id`

	conds := []string{}
	args := []any{}

	if filter.Trashed {
		conds = append(conds, "e.deleted_at IS NOT NULL")
	} else {
		conds = append(conds, "e.deleted_at IS NULL")
	}
	if filter.From != nil && *filter.From != "" {
		conds = append(conds, "e.start_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil && *filter.To != "" {
		conds = append(conds, "e.start_at <= ?")
		args = append(args, *filter.To)
	}
	if filter.Recurrence != nil && *filter.Recurrence != "" {
		conds = append(conds, "e.recurrence = ?")
		args = append(args, *filter.Recurrence)
	}
	if filter.CategorySlug != "" {
		conds = append(conds, "c.slug = ? COLLATE NOCASE")
		args = append(args, filter.CategorySlug)
	}
	if filter.TagName != "" {
		conds = append(conds, `EXISTS (
			SELECT 1 FROM calendar_tags ct
			JOIN tags tg ON tg.id = ct.tag_id
			WHERE ct.event_id = e.id AND tg.name = ? COLLATE NOCASE
		)`)
		args = append(args, filter.TagName)
	}

	q1 := base
	if len(conds) > 0 {
		q1 += "\nWHERE " + strings.Join(conds, " AND ")
	}
	q1 += "\nORDER BY e.start_at ASC, e.created_at DESC, e.id DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	q1 += fmt.Sprintf("\nLIMIT %d", limit)
	if filter.Offset > 0 {
		q1 += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := q.db.QueryContext(ctx, q1, args...)
	if err != nil {
		return nil, fmt.Errorf("list calendar: %w", err)
	}
	defer rows.Close()

	var out []ListedCalendar
	for rows.Next() {
		var lc ListedCalendar
		var tagCSV string
		if err := rows.Scan(
			&lc.Calendar.ID, &lc.Calendar.Title, &lc.Calendar.Description,
			&lc.Calendar.StartAt, &lc.Calendar.EndAt, &lc.Calendar.AllDay,
			&lc.Calendar.Location, &lc.Calendar.CategoryID, &lc.Calendar.Notes,
			&lc.Calendar.Recurrence, &lc.Calendar.CreatedAt, &lc.Calendar.UpdatedAt,
			&lc.Calendar.DeletedAt,
			&lc.CategoryName, &lc.CategorySlug,
			&tagCSV,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if tagCSV != "" {
			lc.Tags = strings.Split(tagCSV, ",")
		}
		out = append(out, lc)
	}
	return out, rows.Err()
}

// ListTodosFiltered runs a dynamic SQL query against the destination DB using
// filter. Tags are aggregated in-query via GROUP_CONCAT.
func (q *Queries) ListTodosFiltered(ctx context.Context, filter TodoListFilter) ([]ListedTodo, error) {
	const base = `
SELECT t.id, t.title, t.description, t.priority, t.status, t.due_date,
       t.category_id, t.notes, t.recurrence, t.done_at, t.created_at, t.updated_at, t.deleted_at,
       c.name AS category_name, c.slug AS category_slug,
       (
         SELECT COALESCE(GROUP_CONCAT(tag.name, ','), '')
         FROM todo_tags tt
         JOIN tags tag ON tag.id = tt.tag_id
         WHERE tt.todo_id = t.id
       ) AS tag_csv
FROM todos t
LEFT JOIN categories c ON c.id = t.category_id`

	conds := []string{}
	args := []any{}

	if filter.Trashed {
		conds = append(conds, "t.deleted_at IS NOT NULL")
	} else {
		conds = append(conds, "t.deleted_at IS NULL")
	}
	if filter.Status != "" {
		conds = append(conds, "t.status = ?")
		args = append(args, filter.Status)
	}
	if filter.Priority != "" {
		conds = append(conds, "t.priority = ?")
		args = append(args, filter.Priority)
	}
	if filter.CategorySlug != "" {
		conds = append(conds, "c.slug = ? COLLATE NOCASE")
		args = append(args, filter.CategorySlug)
	}
	if filter.TagName != "" {
		conds = append(conds, `EXISTS (
			SELECT 1 FROM todo_tags tt
			JOIN tags tg ON tg.id = tt.tag_id
			WHERE tt.todo_id = t.id AND tg.name = ? COLLATE NOCASE
		)`)
		args = append(args, filter.TagName)
	}
	if filter.OnlyOverdue {
		conds = append(conds, "t.due_date < ? AND t.status = 'open' AND t.due_date IS NOT NULL")
		args = append(args, filter.Today)
	}
	if filter.DueBefore != "" {
		conds = append(conds, "t.due_date < ? AND t.due_date IS NOT NULL")
		args = append(args, filter.DueBefore)
	}
	if filter.DueAfter != "" {
		conds = append(conds, "t.due_date >= ? AND t.due_date IS NOT NULL")
		args = append(args, filter.DueAfter)
	}

	q1 := base
	if len(conds) > 0 {
		q1 += "\nWHERE " + strings.Join(conds, " AND ")
	}
	q1 += "\nORDER BY t.due_date ASC NULLS LAST, t.created_at DESC, t.id DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	q1 += fmt.Sprintf("\nLIMIT %d", limit)
	if filter.Offset > 0 {
		q1 += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := q.db.QueryContext(ctx, q1, args...)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()

	var out []ListedTodo
	for rows.Next() {
		var lt ListedTodo
		var tagCSV string
		if err := rows.Scan(
			&lt.Todo.ID, &lt.Todo.Title, &lt.Todo.Description, &lt.Todo.Priority, &lt.Todo.Status,
			&lt.Todo.DueDate, &lt.Todo.CategoryID, &lt.Todo.Notes, &lt.Todo.Recurrence,
			&lt.Todo.DoneAt, &lt.Todo.CreatedAt, &lt.Todo.UpdatedAt, &lt.Todo.DeletedAt,
			&lt.CategoryName, &lt.CategorySlug,
			&tagCSV,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if tagCSV != "" {
			lt.Tags = strings.Split(tagCSV, ",")
		}
		out = append(out, lt)
	}
	return out, rows.Err()
}
