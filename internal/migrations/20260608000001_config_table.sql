-- +goose Up
-- +goose StatementBegin
-- Generic key-value configuration table, shared across domains.
-- Keys are free-form (e.g. "currency", "landing_surface", "active_domains").
-- Values are stored as text; callers parse/validate before use.
CREATE TABLE arsenal_config (
    k TEXT PRIMARY KEY,
    v TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS arsenal_config;
-- +goose StatementEnd
