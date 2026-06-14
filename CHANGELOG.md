# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Calendar domain end-to-end** — new `internal/calendar/` package (domain types, validators, service) backed by `calendar_events`, `calendar_tags`, and a `calendar_fts` FTS5 index over title/description/location. Supports timed and all-day events with start/end, location, category, tags, notes, and recurrence (`none/daily/weekly/monthly/yearly`); full lifecycle (create, edit, soft-delete, restore, purge) with cross-domain orphan-tag cleanup.
- **Calendar CLI** — `arsenal calendar add|list|show|edit|rm|restore|purge|export` with start/end, `--all-day`, location/category/tag/notes/recurrence flags, `--json` output, and a non-interactive purge guard.
- **Calendar iCal export** — `arsenal calendar export --format ical [--output path]` writes RFC 5545 (VCALENDAR/VEVENT, all-day and timed events, RRULE from the recurrence enum, text escaping, and 75-octet line folding).
- **Calendar TUI area** — `areaCalendar` is no longer a placeholder; renders an event list, detail view, and trash with keybindings for new/edit/delete/restore/purge and navigation.
- **Calendar web surface** — 9 routes (`/calendar`, `/calendar/new`, `/calendar/:id`, `/calendar/:id/edit`, `/calendar/:id/delete`, `/calendar/:id/restore`, `/calendar/:id/purge`, etc.) with HTMX card swaps, an empty state, and a sidebar entry with a count badge.
- **Calendar in Today view** — new `CalendarProvider` adds "today's events" and "upcoming events" (next 7 days) sections to the cross-domain Today view, timezone-aware and omitted when empty.
- **Database migration** — `20260614000000_calendar.sql` creates `calendar_events`, `calendar_tags`, and `calendar_fts` with CHECK constraints, indices, and FTS sync triggers.
- **Finance domain end-to-end** — new `internal/finance/` package (domain types, validators, service) backed by `finance_transactions`, `finance_tags`, and a `finance_fts` FTS5 index. Supports expense/income transactions with category, account, tags, notes, and recurrence; full lifecycle (create, edit, soft-delete, restore, purge) with cross-domain orphan-tag cleanup.
- **Finance CLI** — `arsenal finance add|list|show|edit|rm|restore|purge|export` with date/amount/kind/account/category/tag/notes/recurrence flags, `--json` output, and a non-interactive purge guard.
- **Finance CSV export** — `arsenal finance export --format csv [--output path]` writes RFC 4180 CSV (tags in a quoted, comma-separated cell), honors filter flags, and emits a header-only file when there is no data.
- **Finance TUI area** — `areaFinance` is no longer a placeholder; renders a transaction list, detail view, and trash with keybindings for new/edit/delete/restore/purge and navigation.
- **Finance web surface** — 9 routes (`/finance`, `/finance/new`, `/finance/:id`, `/finance/:id/edit`, `/finance/:id/delete`, `/finance/:id/restore`, `/finance/:id/purge`, etc.) with HTMX card swaps, an empty state, and a sidebar entry with a count badge.
- **Finance in Today view** — new `FinanceProvider` adds "this-month spending" (total + top categories) and "recent transactions" sections to the cross-domain Today view, timezone-aware and omitted when empty.
- **Database migration** — `20260613000000_finance.sql` creates `finance_transactions`, `finance_tags`, and `finance_fts` with CHECK constraints, indices, and FTS sync triggers.
- **Today cross-domain view** — `internal/today/` package with `Provider` interface, `Registry`, `Service`, and two concrete providers (`TodosProvider`, `ResourcesProvider`). Aggregates overdue todos, due-today todos, upcoming todos, and recent resources into a single unified view.
- **TUI Today area** — `areaToday` is no longer a placeholder; renders real aggregated data with `r` key refresh and `n` key to open an inline new-todo form (title field, default `med` priority, enter saves, esc cancels). Default landing surface changed from `areaResources` to `areaToday`.
- **Web Today route** — `GET /today` renders the Today view with sectioned cards, density truncation (5 items per section), and "show all" links. Sidebar includes a "Today" entry with overdue count badge that updates via `hx-swap-oob` after mark-done/open actions.
- **CLI `arsenal today`** — terminal surface for the cross-domain Today view; renders overdue / due-today / upcoming todos and recent resources as a tab-aligned table (with show-all links) or as JSON via `--json`. Completes spec REQ-TV-08.
- **Todos lifecycle & status management** — add, edit, mark done, soft-delete, restore, and hard-delete todos.
- **Todos listing & search** — list all todos with filtering by status, priority, and tags; full-text search via SQLite FTS5.
- **Tags support** — assign and filter todos by tags; validates shared domain helpers (`domain.WithTags`) with a second domain.
- **CLI commands** — `arsenal todo add|list|show|edit|done|rm|restore|purge` with flags for priority, tags, recurrence, and JSON output.
- **TUI area-switcher prototype** — `arsenal tui` with all functional areas: **Resources**, **Todos**, **Today**, **Finance**, and **Calendar**.
- **Web interface** — 11 new todo routes (`/todos`, `/todos/new`, `/todos/:id`, `/todos/:id/edit`, `/todos/:id/done`, `/todos/:id/open`, `/todos/:id/delete`, etc.) plus sidebar integration across all pages.
- **Recurrence placeholder** — schema and domain support for recurring todos (UI wired, scheduler pending).
- **Database migration** — `20260608000002_todos.sql` creates `todos`, `todo_tags`, and `todo_search` tables with FTS5 indexing.

### Changed

- **KeyLandingSurface enum values** — expanded from `["tui", "web"]` to `["today", "resources"]`. **Backward incompatible**: users with old config values (`tui` or `web`) will fall back to the default `today`. No data migration required; config store validates on read.
- **Cross-cutting validation** — shared `domain.WithTags` helper now validated by both `resources` and `todos` domains.
- **Provider registry pattern** — validates ADR-0002 Change 4: cross-domain aggregation via independent providers with graceful degradation.

### Fixed

- **Timezone handling** — date-based comparisons now respect user-configured timezone (`KeyUserTimezone`), defaulting to UTC for backwards compatibility.

### References

- ADR-0002: Sequencing and phase-3 OpenSpec change.

