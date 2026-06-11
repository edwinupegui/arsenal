# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Today cross-domain view** — `internal/today/` package with `Provider` interface, `Registry`, `Service`, and two concrete providers (`TodosProvider`, `ResourcesProvider`). Aggregates overdue todos, due-today todos, upcoming todos, and recent resources into a single unified view.
- **TUI Today area** — `areaToday` is no longer a placeholder; renders real aggregated data with `r` key refresh and `n` key to open an inline new-todo form (title field, default `med` priority, enter saves, esc cancels). Default landing surface changed from `areaResources` to `areaToday`.
- **Web Today route** — `GET /today` renders the Today view with sectioned cards, density truncation (5 items per section), and "show all" links. Sidebar includes a "Today" entry with overdue count badge that updates via `hx-swap-oob` after mark-done/open actions.
- **CLI `arsenal today`** — terminal surface for the cross-domain Today view; renders overdue / due-today / upcoming todos and recent resources as a tab-aligned table (with show-all links) or as JSON via `--json`. Completes spec REQ-TV-08.
- **Todos lifecycle & status management** — add, edit, mark done, soft-delete, restore, and hard-delete todos.
- **Todos listing & search** — list all todos with filtering by status, priority, and tags; full-text search via SQLite FTS5.
- **Tags support** — assign and filter todos by tags; validates shared domain helpers (`domain.WithTags`) with a second domain.
- **CLI commands** — `arsenal todo add|list|show|edit|done|rm|restore|purge` with flags for priority, tags, recurrence, and JSON output.
- **TUI area-switcher prototype** — `arsenal tui` with functional **Resources** and **Todos** areas; **Today**, **Finance**, and **Calendar** as placeholder areas.
- **Web interface** — 11 new todo routes (`/todos`, `/todos/new`, `/todos/:id`, `/todos/:id/edit`, `/todos/:id/done`, `/todos/:id/open`, `/todos/:id/delete`, etc.) plus sidebar integration across all pages.
- **Recurrence placeholder** — schema and domain support for recurring todos (UI wired, scheduler pending).
- **Database migration** — `20260608000002_todos.sql` creates `todos`, `todo_tags`, and `todo_search` tables with FTS5 indexing.

### Changed

- **KeyLandingSurface enum values** — expanded from `["tui", "web"]` to `["today", "resources"]`. **Backward incompatible**: users with old config values (`tui` or `web`) will fall back to the default `today`. No data migration required; config store validates on read.
- **Cross-cutting validation** — shared `domain.WithTags` helper now validated by both `resources` and `todos` domains.
- **Provider registry pattern** — validates ADR-0002 Change 4: cross-domain aggregation via independent providers with graceful degradation.

### Known limitations

- **Timezone handling** — `date('now')` in SQLite is UTC. Due-today comparison may be off by hours for non-UTC users. ADR-0003 accepted; implementation deferred to a follow-up.

### References

- ADR-0002: Sequencing and phase-3 OpenSpec change.

