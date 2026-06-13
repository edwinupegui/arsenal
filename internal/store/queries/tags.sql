-- name: UpsertTag :one
INSERT INTO tags (name) VALUES (?)
ON CONFLICT(name) DO UPDATE SET name = excluded.name
RETURNING *;

-- name: GetTagByName :one
SELECT * FROM tags WHERE name = ? COLLATE NOCASE LIMIT 1;

-- name: ListTags :many
SELECT t.*, COUNT(rt.resource_id) AS resource_count
FROM tags t
LEFT JOIN resource_tags rt ON rt.tag_id = t.id
GROUP BY t.id
ORDER BY t.name ASC;

-- name: AttachTag :exec
INSERT OR IGNORE INTO resource_tags (resource_id, tag_id) VALUES (?, ?);

-- name: DetachTag :exec
DELETE FROM resource_tags WHERE resource_id = ? AND tag_id = ?;

-- name: DetachAllTagsFromResource :exec
DELETE FROM resource_tags WHERE resource_id = ?;

-- name: ListTagsForResource :many
SELECT t.*
FROM tags t
JOIN resource_tags rt ON rt.tag_id = t.id
WHERE rt.resource_id = ?
ORDER BY t.name ASC;

-- name: ListTagsForTodo :many
SELECT t.*
FROM tags t
JOIN todo_tags tt ON tt.tag_id = t.id
WHERE tt.todo_id = ?
ORDER BY t.name ASC;

-- name: RenameTag :one
UPDATE tags SET name = ? WHERE id = ? RETURNING *;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = ?;

-- name: DeleteOrphanTags :exec
DELETE FROM tags
WHERE id NOT IN (
    SELECT DISTINCT tag_id FROM resource_tags
    UNION
    SELECT DISTINCT tag_id FROM todo_tags
    UNION
    SELECT DISTINCT tag_id FROM finance_tags
);
