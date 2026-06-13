-- +goose Up
-- Finance domain (v3.x). Mirrors the todos table shape: amount, kind, soft-delete
-- timestamp, optional category_id FK, and a FTS5 virtual table.

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS finance_transactions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    date        TEXT    NOT NULL,
    amount      REAL    NOT NULL,
    kind        TEXT    NOT NULL CHECK (kind IN ('expense', 'income')),
    account     TEXT    NOT NULL DEFAULT '',
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    notes       TEXT,
    recurrence  TEXT    NOT NULL DEFAULT 'none' CHECK (recurrence IN ('none','daily','weekly','monthly')),
    currency    TEXT    NOT NULL DEFAULT 'USD',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at  TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS finance_tags (
    finance_id INTEGER NOT NULL REFERENCES finance_transactions(id) ON DELETE CASCADE,
    tag_id     INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (finance_id, tag_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_finance_date     ON finance_transactions(date);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_finance_kind     ON finance_transactions(kind);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_finance_deleted  ON finance_transactions(deleted_at);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_finance_category ON finance_transactions(category_id);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_finance_account  ON finance_transactions(account);
-- +goose StatementEnd

-- Auto-bump updated_at on every UPDATE.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_finance_updated_at
AFTER UPDATE ON finance_transactions
FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE finance_transactions
    SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- FTS5 virtual table on notes and account.
-- Note: CREATE VIRTUAL TABLE does not support IF NOT EXISTS in all SQLite builds.
-- +goose StatementBegin
CREATE VIRTUAL TABLE finance_fts USING fts5(
    notes,
    account,
    tokenize='unicode61 remove_diacritics 2'
);
-- +goose StatementEnd

-- Sync triggers for finance_fts.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_finance_fts_insert
AFTER INSERT ON finance_transactions
BEGIN
    INSERT INTO finance_fts(rowid, notes, account)
    VALUES (NEW.id, COALESCE(NEW.notes, ''), COALESCE(NEW.account, ''));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_finance_fts_update
AFTER UPDATE OF notes, account ON finance_transactions
BEGIN
    DELETE FROM finance_fts WHERE rowid = OLD.id;
    INSERT INTO finance_fts(rowid, notes, account)
    VALUES (NEW.id, COALESCE(NEW.notes, ''), COALESCE(NEW.account, ''));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_finance_fts_delete
AFTER DELETE ON finance_transactions
BEGIN
    DELETE FROM finance_fts WHERE rowid = OLD.id;
END;
-- +goose StatementEnd

-- +goose Down
-- Rollback: DROP TRIGGER, TABLE, INDEX in reverse order.
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_finance_fts_delete;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_finance_fts_update;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_finance_fts_insert;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS finance_fts;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_finance_updated_at;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_finance_account;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_finance_category;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_finance_deleted;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_finance_kind;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_finance_date;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS finance_tags;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS finance_transactions;
-- +goose StatementEnd
