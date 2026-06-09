# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Todos lifecycle & status management** — add, edit, mark done, soft-delete, restore, and hard-delete todos.
- **Todos listing & search** — list all todos with filtering by status, priority, and tags; full-text search via SQLite FTS5.
- **Tags support** — assign and filter todos by tags; validates shared domain helpers (`domain.WithTags`) with a second domain.
- **CLI commands** — `arsenal todo add|list|show|edit|done|rm|restore|purge` with flags for priority, tags, recurrence, and JSON output.
- **TUI area-switcher prototype** — `arsenal tui` with functional **Resources** and **Todos** areas; **Today**, **Finance**, and **Calendar** as placeholder areas.
- **Web interface** — 11 new todo routes (`/todos`, `/todos/new`, `/todos/:id`, `/todos/:id/edit`, `/todos/:id/done`, `/todos/:id/open`, `/todos/:id/delete`, etc.) plus sidebar integration across all pages.
- **Recurrence placeholder** — schema and domain support for recurring todos (UI wired, scheduler pending).
- **Database migration** — `20260608000002_todos.sql` creates `todos`, `todo_tags`, and `todo_search` tables with FTS5 indexing.

### Changed

- **Cross-cutting validation** — shared `domain.WithTags` helper now validated by both `resources` and `todos` domains.

### References

- ADR-0002: Sequencing and phase-2 OpenSpec change.

