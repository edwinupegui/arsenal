-- name: CreateFinanceTransaction :one
INSERT INTO finance_transactions (
    date, amount, kind, account, category_id, notes, recurrence, currency
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetFinanceTransaction :one
SELECT * FROM finance_transactions WHERE id = ? LIMIT 1;

-- name: ListFinanceTransactions :many
SELECT * FROM finance_transactions
WHERE deleted_at IS NULL
ORDER BY date DESC, created_at DESC, id DESC
LIMIT ? OFFSET ?;

-- name: ListTrashedFinanceTransactions :many
SELECT * FROM finance_transactions
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id DESC;

-- name: UpdateFinanceTransaction :one
UPDATE finance_transactions
SET date        = ?,
    amount      = ?,
    kind        = ?,
    account     = ?,
    category_id = ?,
    notes       = ?,
    recurrence  = ?
WHERE id = ?
RETURNING *;

-- name: SoftDeleteFinanceTransaction :exec
UPDATE finance_transactions
SET deleted_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND deleted_at IS NULL;

-- name: RestoreFinanceTransaction :exec
UPDATE finance_transactions
SET deleted_at = NULL
WHERE id = ? AND deleted_at IS NOT NULL;

-- name: PurgeFinanceTransaction :exec
DELETE FROM finance_transactions WHERE id = ?;

-- name: CountFinanceTransactions :one
SELECT COUNT(*) FROM finance_transactions WHERE deleted_at IS NULL;

-- name: ListFinanceByMonth :many
SELECT * FROM finance_transactions
WHERE kind = 'expense'
  AND deleted_at IS NULL
  AND date >= ? AND date <= ?
ORDER BY date DESC, created_at DESC;

-- name: TopCategoriesByMonth :many
SELECT
    c.name AS category_name,
    COALESCE(SUM(f.amount), 0) AS total
FROM finance_transactions f
LEFT JOIN categories c ON c.id = f.category_id
WHERE f.kind = 'expense'
  AND f.deleted_at IS NULL
  AND f.date >= ? AND f.date <= ?
GROUP BY c.id
ORDER BY total DESC
LIMIT ?;

-- name: ListTagsForFinance :many
SELECT t.*
FROM tags t
JOIN finance_tags ft ON ft.tag_id = t.id
WHERE ft.finance_id = ?
ORDER BY t.name ASC;

-- name: AttachTagToFinance :exec
INSERT OR IGNORE INTO finance_tags (finance_id, tag_id) VALUES (?, ?);

-- name: DetachAllTagsFromFinance :exec
DELETE FROM finance_tags WHERE finance_id = ?;

-- ListFinanceFiltered is hand-written in list.go (dynamic WHERE).

-- SearchFinance is hand-written in search.go (FTS5 MATCH not parseable by sqlc).
