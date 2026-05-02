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
