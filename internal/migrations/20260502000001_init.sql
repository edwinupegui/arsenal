-- +goose Up
-- +goose StatementBegin
CREATE TABLE categories (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    slug       TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    name       TEXT    NOT NULL,
    icon       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE resources (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    url         TEXT    NOT NULL UNIQUE,
    description TEXT,
    type        TEXT    NOT NULL CHECK (type IN (
                    'video','article','tool','repo','course',
                    'podcast','newsletter','community','book','other'
                )),
    language    TEXT    NOT NULL DEFAULT 'OTHER' CHECK (language IN (
                    'ES','EN','PT','OTHER'
                )),
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    notes       TEXT,
    favorite    INTEGER NOT NULL DEFAULT 0 CHECK (favorite IN (0,1)),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at  TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE tags (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL UNIQUE COLLATE NOCASE
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE resource_tags (
    resource_id INTEGER NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    tag_id      INTEGER NOT NULL REFERENCES tags(id)      ON DELETE CASCADE,
    PRIMARY KEY (resource_id, tag_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_resources_deleted  ON resources(deleted_at);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_resources_type     ON resources(type);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_resources_lang     ON resources(language);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_resources_category ON resources(category_id);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_resources_favorite ON resources(favorite);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_resource_tags_tag  ON resource_tags(tag_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_resources_updated_at
AFTER UPDATE ON resources
FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE resources
    SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_resources_updated_at;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_resource_tags_tag;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_resources_favorite;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_resources_category;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_resources_lang;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_resources_type;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_resources_deleted;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS resource_tags;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS tags;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS resources;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS categories;
-- +goose StatementEnd
