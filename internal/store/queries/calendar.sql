-- name: CreateCalendarEvent :one
INSERT INTO calendar_events (
    title, description, start_at, end_at, all_day, location, category_id, notes, recurrence
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetCalendarEvent :one
SELECT * FROM calendar_events WHERE id = ? LIMIT 1;

-- name: ListCalendarEvents :many
SELECT * FROM calendar_events
WHERE deleted_at IS NULL
ORDER BY start_at ASC, created_at DESC, id DESC
LIMIT ? OFFSET ?;

-- name: ListTrashedCalendarEvents :many
SELECT * FROM calendar_events
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id DESC;

-- name: UpdateCalendarEvent :one
UPDATE calendar_events
SET title = ?, description = ?, start_at = ?, end_at = ?, all_day = ?,
    location = ?, category_id = ?, notes = ?, recurrence = ?
WHERE id = ?
RETURNING *;

-- name: SoftDeleteCalendarEvent :exec
UPDATE calendar_events
SET deleted_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND deleted_at IS NULL;

-- name: RestoreCalendarEvent :exec
UPDATE calendar_events
SET deleted_at = NULL
WHERE id = ? AND deleted_at IS NOT NULL;

-- name: PurgeCalendarEvent :exec
DELETE FROM calendar_events WHERE id = ?;

-- name: CountCalendarEvents :one
SELECT COUNT(*) FROM calendar_events WHERE deleted_at IS NULL;

-- name: ListEventsToday :many
-- Events whose start_at falls within [dayStart, dayEnd]. The provider passes
-- bounds formatted for the all_day branch (date) or timed branch (datetime).
SELECT * FROM calendar_events
WHERE deleted_at IS NULL
  AND start_at >= ? AND start_at <= ?
ORDER BY start_at ASC, id ASC;

-- name: ListEventsUpcoming :many
-- Events whose start_at falls within (dayEnd, weekEnd].
SELECT * FROM calendar_events
WHERE deleted_at IS NULL
  AND start_at > ? AND start_at <= ?
ORDER BY start_at ASC, id ASC;

-- name: ListTagsForCalendar :many
SELECT t.*
FROM tags t
JOIN calendar_tags ct ON ct.tag_id = t.id
WHERE ct.event_id = ?
ORDER BY t.name ASC;

-- name: AttachTagToCalendar :exec
INSERT OR IGNORE INTO calendar_tags (event_id, tag_id) VALUES (?, ?);

-- name: DetachAllTagsFromCalendar :exec
DELETE FROM calendar_tags WHERE event_id = ?;

-- ListCalendarFiltered is hand-written in list.go (dynamic WHERE).
-- SearchCalendar is hand-written in search.go (FTS5 MATCH not parseable by sqlc).
