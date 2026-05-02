-- +goose Up
-- +goose StatementBegin
CREATE VIRTUAL TABLE resources_fts USING fts5(
    title,
    description,
    notes,
    tags,
    tokenize='unicode61 remove_diacritics 2'
);
-- +goose StatementEnd

-- Helper view: aggregated tags per resource (for FTS payload)
-- +goose StatementBegin
CREATE VIEW v_resource_tags AS
SELECT
    r.id AS resource_id,
    COALESCE(GROUP_CONCAT(t.name, ' '), '') AS tags
FROM resources r
LEFT JOIN resource_tags rt ON rt.resource_id = r.id
LEFT JOIN tags t           ON t.id          = rt.tag_id
GROUP BY r.id;
-- +goose StatementEnd

-- Insert: index a new resource with empty tags (tags arrive via resource_tags triggers)
-- +goose StatementBegin
CREATE TRIGGER trg_resources_fts_insert
AFTER INSERT ON resources
BEGIN
    INSERT INTO resources_fts(rowid, title, description, notes, tags)
    VALUES (NEW.id,
            COALESCE(NEW.title, ''),
            COALESCE(NEW.description, ''),
            COALESCE(NEW.notes, ''),
            '');
END;
-- +goose StatementEnd

-- Update of textual columns: replace row in FTS preserving current aggregated tags
-- +goose StatementBegin
CREATE TRIGGER trg_resources_fts_update
AFTER UPDATE OF title, description, notes ON resources
BEGIN
    DELETE FROM resources_fts WHERE rowid = OLD.id;
    INSERT INTO resources_fts(rowid, title, description, notes, tags)
    VALUES (NEW.id,
            COALESCE(NEW.title, ''),
            COALESCE(NEW.description, ''),
            COALESCE(NEW.notes, ''),
            (SELECT tags FROM v_resource_tags WHERE resource_id = NEW.id));
END;
-- +goose StatementEnd

-- Delete: drop from FTS
-- +goose StatementBegin
CREATE TRIGGER trg_resources_fts_delete
AFTER DELETE ON resources
BEGIN
    DELETE FROM resources_fts WHERE rowid = OLD.id;
END;
-- +goose StatementEnd

-- Tag link change: refresh tags column for the affected resource
-- +goose StatementBegin
CREATE TRIGGER trg_resource_tags_fts_insert
AFTER INSERT ON resource_tags
BEGIN
    UPDATE resources_fts
    SET tags = (SELECT tags FROM v_resource_tags WHERE resource_id = NEW.resource_id)
    WHERE rowid = NEW.resource_id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_resource_tags_fts_delete
AFTER DELETE ON resource_tags
BEGIN
    UPDATE resources_fts
    SET tags = (SELECT tags FROM v_resource_tags WHERE resource_id = OLD.resource_id)
    WHERE rowid = OLD.resource_id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_resource_tags_fts_delete;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_resource_tags_fts_insert;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_resources_fts_delete;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_resources_fts_update;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_resources_fts_insert;
-- +goose StatementEnd
-- +goose StatementBegin
DROP VIEW IF EXISTS v_resource_tags;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS resources_fts;
-- +goose StatementEnd
