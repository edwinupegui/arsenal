package store

import (
	"context"
	"fmt"
	"strings"
)

// SearchResources runs a full-text search over title/description/notes/tags
// using FTS5 and returns results enriched with category info and aggregated
// tags, matching the shape produced by ListResourcesFiltered. Soft-deleted
// resources are excluded.
//
// Query handling: the user-typed string is split on whitespace, each term is
// stripped of FTS5 syntax characters and wrapped as a prefix ("term"*), then
// joined with implicit AND. An empty query returns an empty result set
// instead of an error so the CLI can render "no matches".
func (q *Queries) SearchResources(ctx context.Context, query string, limit int64) ([]ListedResource, error) {
	match := buildFTSQuery(query)
	if match == "" {
		return []ListedResource{}, nil
	}

	const stmt = `
SELECT r.id, r.title, r.url, r.description, r.type, r.language,
       r.category_id, r.notes, r.favorite, r.created_at, r.updated_at, r.deleted_at,
       c.name AS category_name, c.slug AS category_slug,
       (
         SELECT COALESCE(GROUP_CONCAT(t.name, ','), '')
         FROM resource_tags rt
         JOIN tags t ON t.id = rt.tag_id
         WHERE rt.resource_id = r.id
       ) AS tag_csv
FROM resources_fts f
JOIN resources r ON r.id = f.rowid
LEFT JOIN categories c ON c.id = r.category_id
WHERE resources_fts MATCH ?
  AND r.deleted_at IS NULL
ORDER BY rank
LIMIT ?`

	rows, err := q.db.QueryContext(ctx, stmt, match, limit)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
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
			return nil, fmt.Errorf("fts scan: %w", err)
		}
		if tagCSV != "" {
			lr.Tags = strings.Split(tagCSV, ",")
		}
		out = append(out, lr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fts iterate: %w", err)
	}
	return out, nil
}

// buildFTSQuery converts a user-typed query into a safe FTS5 MATCH expression.
// Empty terms are dropped; FTS5 special characters are stripped from each term;
// each surviving term gets a prefix-match suffix (foo -> "foo"*).
func buildFTSQuery(raw string) string {
	terms := strings.Fields(raw)
	cleaned := make([]string, 0, len(terms))
	for _, t := range terms {
		t = stripFTSSpecials(t)
		if t == "" {
			continue
		}
		cleaned = append(cleaned, fmt.Sprintf("%q*", t))
	}
	return strings.Join(cleaned, " ")
}

// SearchTodos runs a full-text search over title/description/notes/tags using
// FTS5 and returns results enriched with category info and aggregated tags,
// matching the shape produced by ListTodosFiltered. Soft-deleted todos are
// excluded.
func (q *Queries) SearchTodos(ctx context.Context, query string, limit int64) ([]ListedTodo, error) {
	match := buildFTSQuery(query)
	if match == "" {
		return []ListedTodo{}, nil
	}

	const stmt = `
SELECT t.id, t.title, t.description, t.priority, t.status, t.due_date,
       t.category_id, t.notes, t.recurrence, t.done_at, t.created_at, t.updated_at, t.deleted_at,
       c.name AS category_name, c.slug AS category_slug,
       (
         SELECT COALESCE(GROUP_CONCAT(tag.name, ','), '')
         FROM todo_tags tt
         JOIN tags tag ON tag.id = tt.tag_id
         WHERE tt.todo_id = t.id
       ) AS tag_csv
FROM todos_fts f
JOIN todos t ON t.id = f.rowid
LEFT JOIN categories c ON c.id = t.category_id
WHERE todos_fts MATCH ?
  AND t.deleted_at IS NULL
ORDER BY rank
LIMIT ?`

	rows, err := q.db.QueryContext(ctx, stmt, match, limit)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
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
			return nil, fmt.Errorf("fts scan: %w", err)
		}
		if tagCSV != "" {
			lt.Tags = strings.Split(tagCSV, ",")
		}
		out = append(out, lt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fts iterate: %w", err)
	}
	return out, nil
}

// SearchFinance runs a full-text search over notes and account using FTS5
// and returns results enriched with category info and aggregated tags,
// matching the shape produced by ListFinanceFiltered. Soft-deleted
// transactions are excluded.
func (q *Queries) SearchFinance(ctx context.Context, query string, limit int64) ([]ListedFinance, error) {
	match := buildFTSQuery(query)
	if match == "" {
		return []ListedFinance{}, nil
	}

	const stmt = `
SELECT f.id, f.date, f.amount, f.kind, f.account, f.category_id,
       f.notes, f.recurrence, f.currency, f.created_at, f.updated_at, f.deleted_at,
       c.name AS category_name, c.slug AS category_slug,
       (
         SELECT COALESCE(GROUP_CONCAT(tag.name, ','), '')
         FROM finance_tags ft
         JOIN tags tag ON tag.id = ft.tag_id
         WHERE ft.finance_id = f.id
       ) AS tag_csv
FROM finance_fts ft
JOIN finance_transactions f ON f.id = ft.rowid
LEFT JOIN categories c ON c.id = f.category_id
WHERE finance_fts MATCH ?
  AND f.deleted_at IS NULL
ORDER BY rank
LIMIT ?`

	rows, err := q.db.QueryContext(ctx, stmt, match, limit)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
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
			return nil, fmt.Errorf("fts scan: %w", err)
		}
		if tagCSV != "" {
			lf.Tags = strings.Split(tagCSV, ",")
		}
		out = append(out, lf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fts iterate: %w", err)
	}
	return out, nil
}

func stripFTSSpecials(s string) string {
	const specials = `"'()*:^-+`
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(specials, r) {
			return -1
		}
		return r
	}, s)
}
