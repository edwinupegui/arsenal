-- name: CreateTodo :one
INSERT INTO todos (
    title, description, priority, status, due_date, category_id, notes, recurrence
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetTodo :one
SELECT * FROM todos WHERE id = ? LIMIT 1;

-- name: ListTodos :many
SELECT * FROM todos
WHERE deleted_at IS NULL
ORDER BY due_date ASC NULLS LAST, created_at DESC, id DESC
LIMIT ?
OFFSET ?;

-- name: ListTrashedTodos :many
SELECT * FROM todos
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id DESC;

-- name: UpdateTodo :one
UPDATE todos
SET title       = ?,
    description = ?,
    priority    = ?,
    status      = ?,
    due_date    = ?,
    category_id = ?,
    notes       = ?,
    recurrence  = ?
WHERE id = ?
RETURNING *;

-- name: SoftDeleteTodo :exec
UPDATE todos
SET deleted_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND deleted_at IS NULL;

-- name: RestoreTodo :exec
UPDATE todos
SET deleted_at = NULL
WHERE id = ? AND deleted_at IS NOT NULL;

-- name: PurgeTodo :exec
DELETE FROM todos WHERE id = ?;

-- name: MarkTodoDone :exec
UPDATE todos
SET status = 'done', done_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status = 'open';

-- name: MarkTodoOpen :exec
UPDATE todos
SET status = 'open', done_at = NULL
WHERE id = ? AND status = 'done';

-- name: ListTodosByStatus :many
SELECT * FROM todos
WHERE status = ? AND deleted_at IS NULL
ORDER BY due_date ASC NULLS LAST, created_at DESC, id DESC;

-- name: ListTodosDueBefore :many
SELECT * FROM todos
WHERE due_date < ? AND status = 'open' AND due_date IS NOT NULL AND deleted_at IS NULL
ORDER BY due_date ASC;

-- name: ListTodosDueBetween :many
SELECT * FROM todos
WHERE due_date BETWEEN ? AND ? AND status = 'open' AND due_date IS NOT NULL AND deleted_at IS NULL
ORDER BY due_date ASC;

-- name: CountOpenTodos :one
SELECT COUNT(*) FROM todos WHERE status = 'open' AND deleted_at IS NULL;

-- name: ListTodosBasic :many
SELECT t.*, c.name AS category_name, c.slug AS category_slug,
       (
         SELECT COALESCE(GROUP_CONCAT(tag.name, ','), '')
         FROM todo_tags tt
         JOIN tags tag ON tag.id = tt.tag_id
         WHERE tt.todo_id = t.id
       ) AS tag_csv
FROM todos t
LEFT JOIN categories c ON c.id = t.category_id
WHERE (COALESCE(sqlc.narg('trashed'), 0) = 0 AND t.deleted_at IS NULL)
   OR (COALESCE(sqlc.narg('trashed'), 0) = 1 AND t.deleted_at IS NOT NULL)
ORDER BY t.due_date ASC NULLS LAST, t.created_at DESC, t.id DESC
LIMIT sqlc.narg('limit') OFFSET sqlc.narg('offset');

-- SearchTodos is hand-written in search.go (FTS5 MATCH not parseable by sqlc)

-- name: DetachAllTagsFromTodo :exec
DELETE FROM todo_tags WHERE todo_id = ?;

-- name: AttachTagToTodo :exec
INSERT OR IGNORE INTO todo_tags (todo_id, tag_id) VALUES (?, ?);
