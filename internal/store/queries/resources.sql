-- name: CreateResource :one
INSERT INTO resources (
    title, url, description, type, language, category_id, notes, favorite
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: CreateResourceWithTimestamps :one
INSERT INTO resources (
    title, url, description, type, language, category_id, notes, favorite,
    created_at, updated_at, deleted_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetResource :one
SELECT * FROM resources WHERE id = ? LIMIT 1;

-- name: GetResourceByURL :one
SELECT * FROM resources WHERE url = ? LIMIT 1;

-- name: ListResources :many
SELECT * FROM resources
WHERE deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT  ?
OFFSET ?;

-- name: ListTrashedResources :many
SELECT * FROM resources
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id DESC;

-- name: UpdateResource :one
UPDATE resources
SET title       = ?,
    url         = ?,
    description = ?,
    type        = ?,
    language    = ?,
    category_id = ?,
    notes       = ?
WHERE id = ?
RETURNING *;

-- name: SoftDeleteResource :exec
UPDATE resources
SET deleted_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND deleted_at IS NULL;

-- name: RestoreResource :exec
UPDATE resources
SET deleted_at = NULL
WHERE id = ? AND deleted_at IS NOT NULL;

-- name: PurgeResource :exec
DELETE FROM resources WHERE id = ?;

-- name: SetFavorite :exec
UPDATE resources SET favorite = ? WHERE id = ?;

-- name: CountResources :one
SELECT COUNT(*) FROM resources WHERE deleted_at IS NULL;
