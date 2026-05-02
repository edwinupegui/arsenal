package store

import (
	"context"
	"fmt"
	"strings"
)

// SearchResources runs a full-text search over title/description/notes/tags using FTS5.
// The query is sanitized: each whitespace-separated term is wrapped in quotes,
// suffixed with `*` so it matches as a prefix, and joined with implicit AND.
// Soft-deleted resources are excluded.
func (q *Queries) SearchResources(ctx context.Context, query string, limit int64) ([]Resource, error) {
	match := buildFTSQuery(query)
	if match == "" {
		return []Resource{}, nil
	}

	const stmt = `
SELECT r.id, r.title, r.url, r.description, r.type, r.language,
       r.category_id, r.notes, r.favorite, r.created_at, r.updated_at, r.deleted_at
FROM resources_fts f
JOIN resources r ON r.id = f.rowid
WHERE resources_fts MATCH ?
  AND r.deleted_at IS NULL
ORDER BY rank
LIMIT ?`

	rows, err := q.db.QueryContext(ctx, stmt, match, limit)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer rows.Close()

	var out []Resource
	for rows.Next() {
		var r Resource
		if err := rows.Scan(
			&r.ID, &r.Title, &r.Url, &r.Description, &r.Type, &r.Language,
			&r.CategoryID, &r.Notes, &r.Favorite, &r.CreatedAt, &r.UpdatedAt, &r.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("fts scan: %w", err)
		}
		out = append(out, r)
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

func stripFTSSpecials(s string) string {
	const specials = `"'()*:^-+`
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(specials, r) {
			return -1
		}
		return r
	}, s)
}
