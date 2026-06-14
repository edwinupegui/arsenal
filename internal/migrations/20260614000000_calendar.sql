-- +goose Up
-- Calendar domain (v3.x). Mirrors the finance table shape with calendar-specific
-- fields: start_at (datetime or date-only when all_day), nullable end_at,
-- all_day flag, and location. recurrence is metadata-only (no expansion in v3.x).
-- TIMEZONE NOTE (ADR-0003): start_at/end_at are stored as local-time strings
-- without a timezone offset, consistent with the single-system-timezone model.
-- Changing user_timezone reinterprets historical start_at/end_at values.

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS calendar_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    description TEXT,
    start_at    TEXT    NOT NULL,            -- 'YYYY-MM-DDTHH:MM:SS' (timed) or 'YYYY-MM-DD' (all_day=1)
    end_at      TEXT,                        -- nullable; NULL = open-ended; maps to iCal DTEND
    all_day     INTEGER NOT NULL DEFAULT 0 CHECK (all_day IN (0, 1)),
    location    TEXT    NOT NULL DEFAULT '',
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    notes       TEXT,
    recurrence  TEXT    NOT NULL DEFAULT 'none' CHECK (recurrence IN ('none','daily','weekly','monthly','yearly')),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at  TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS calendar_tags (
    event_id INTEGER NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    tag_id   INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (event_id, tag_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_calendar_start    ON calendar_events(start_at);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_calendar_deleted  ON calendar_events(deleted_at);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_calendar_category ON calendar_events(category_id);
-- +goose StatementEnd

-- Auto-bump updated_at on every UPDATE.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_calendar_updated_at
AFTER UPDATE ON calendar_events
FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE calendar_events
    SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- FTS5 virtual table on title, description, location.
-- Note: CREATE VIRTUAL TABLE does not support IF NOT EXISTS.
-- +goose StatementBegin
CREATE VIRTUAL TABLE calendar_fts USING fts5(
    title,
    description,
    location,
    tokenize='unicode61 remove_diacritics 2'
);
-- +goose StatementEnd

-- Sync triggers for calendar_fts.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_calendar_fts_insert
AFTER INSERT ON calendar_events
BEGIN
    INSERT INTO calendar_fts(rowid, title, description, location)
    VALUES (NEW.id, NEW.title, COALESCE(NEW.description, ''), COALESCE(NEW.location, ''));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_calendar_fts_update
AFTER UPDATE OF title, description, location ON calendar_events
BEGIN
    DELETE FROM calendar_fts WHERE rowid = OLD.id;
    INSERT INTO calendar_fts(rowid, title, description, location)
    VALUES (NEW.id, NEW.title, COALESCE(NEW.description, ''), COALESCE(NEW.location, ''));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_calendar_fts_delete
AFTER DELETE ON calendar_events
BEGIN
    DELETE FROM calendar_fts WHERE rowid = OLD.id;
END;
-- +goose StatementEnd

-- +goose Down
-- Rollback: DROP TRIGGER, TABLE, INDEX in reverse order.
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_calendar_fts_delete;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_calendar_fts_update;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_calendar_fts_insert;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS calendar_fts;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_calendar_updated_at;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_calendar_category;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_calendar_deleted;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_calendar_start;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS calendar_tags;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS calendar_events;
-- +goose StatementEnd
