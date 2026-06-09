-- +goose Up
-- Todos (v3.phase2). Mirrors the resources table shape: title, soft-delete
-- timestamp, an optional category_id FK, and a FTS5 virtual table that the
-- upcoming Today view and the cross-domain search will lean on.

-- +goose StatementBegin
CREATE TABLE todos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    description TEXT,
    priority    TEXT    NOT NULL DEFAULT 'med' CHECK (priority IN ('low','med','high')),
    status      TEXT    NOT NULL DEFAULT 'open' CHECK (status IN ('open','done')),
    due_date    TEXT,                               -- ISO-8601 YYYY-MM-DD; NULL = no due date
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    notes       TEXT,
    recurrence  TEXT    NOT NULL DEFAULT 'none' CHECK (recurrence IN ('none','daily','weekly','monthly')),
    done_at     TEXT,                               -- set when status flips to 'done'
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at  TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE todo_tags (
    todo_id INTEGER NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
    PRIMARY KEY (todo_id, tag_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_todos_due      ON todos(due_date);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_todos_status   ON todos(status);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_todos_deleted  ON todos(deleted_at);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_todos_priority ON todos(priority);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_todo_tags_tag  ON todo_tags(tag_id);
-- +goose StatementEnd

-- Auto-bump updated_at on every UPDATE, just like resources.
-- +goose StatementBegin
CREATE TRIGGER trg_todos_updated_at
AFTER UPDATE ON todos
FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE todos
    SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- FTS5 virtual table for todos. Same shape as resources_fts: title,
-- description, notes, tags. Stays in sync via triggers below.
-- +goose StatementBegin
CREATE VIRTUAL TABLE todos_fts USING fts5(
    title,
    description,
    notes,
    tags,
    tokenize='unicode61 remove_diacritics 2'
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE VIEW v_todo_tags AS
SELECT
    t.id AS todo_id,
    COALESCE(GROUP_CONCAT(g.name, ' '), '') AS tags
FROM todos t
LEFT JOIN todo_tags tt ON tt.todo_id = t.id
LEFT JOIN tags g       ON g.id        = tt.tag_id
GROUP BY t.id;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_todos_fts_insert
AFTER INSERT ON todos
BEGIN
    INSERT INTO todos_fts(rowid, title, description, notes, tags)
    VALUES (NEW.id,
            COALESCE(NEW.title, ''),
            COALESCE(NEW.description, ''),
            COALESCE(NEW.notes, ''),
            '');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_todos_fts_update
AFTER UPDATE OF title, description, notes ON todos
BEGIN
    DELETE FROM todos_fts WHERE rowid = OLD.id;
    INSERT INTO todos_fts(rowid, title, description, notes, tags)
    VALUES (NEW.id,
            COALESCE(NEW.title, ''),
            COALESCE(NEW.description, ''),
            COALESCE(NEW.notes, ''),
            (SELECT tags FROM v_todo_tags WHERE todo_id = NEW.id));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_todos_fts_delete
AFTER DELETE ON todos
BEGIN
    DELETE FROM todos_fts WHERE rowid = OLD.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_todo_tags_fts_insert
AFTER INSERT ON todo_tags
BEGIN
    UPDATE todos_fts
    SET tags = (SELECT tags FROM v_todo_tags WHERE todo_id = NEW.todo_id)
    WHERE rowid = NEW.todo_id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_todo_tags_fts_delete
AFTER DELETE ON todo_tags
BEGIN
    UPDATE todos_fts
    SET tags = (SELECT tags FROM v_todo_tags WHERE todo_id = OLD.todo_id)
    WHERE rowid = OLD.todo_id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_todo_tags_fts_delete;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_todo_tags_fts_insert;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_todos_fts_delete;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_todos_fts_update;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_todos_fts_insert;
-- +goose StatementEnd
-- +goose StatementBegin
DROP VIEW IF EXISTS v_todo_tags;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS todos_fts;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_todos_updated_at;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_todo_tags_tag;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_todos_priority;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_todos_deleted;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_todos_status;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_todos_due;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS todo_tags;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS todos;
-- +goose StatementEnd
