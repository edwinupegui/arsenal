-- name: CreateCategory :one
INSERT INTO categories (slug, name, icon, sort_order)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = ? LIMIT 1;

-- name: GetCategoryBySlug :one
SELECT * FROM categories WHERE slug = ? LIMIT 1;

-- name: ListCategories :many
SELECT * FROM categories ORDER BY sort_order ASC, name ASC;

-- name: ListCategoriesWithCounts :many
SELECT
    c.id, c.slug, c.name, c.icon, c.sort_order,
    COUNT(r.id) AS resource_count
FROM categories c
LEFT JOIN resources r
    ON r.category_id = c.id AND r.deleted_at IS NULL
GROUP BY c.id
ORDER BY c.sort_order ASC, c.name ASC;

-- name: UpdateCategory :one
UPDATE categories
SET slug = ?, name = ?, icon = ?, sort_order = ?
WHERE id = ?
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = ?;
