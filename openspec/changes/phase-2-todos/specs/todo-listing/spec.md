# todo-listing

## Purpose

Defines filtered, sorted, and paginated list views over the `todos` table. Every surface (CLI `list`, TUI list model, web `/todos`) consumes this same filtering contract. The listing excludes soft-deleted rows by default; a `trashed` flag opts into the trash view.

## Resolved Open Questions

These questions were left open in the proposal and are resolved here:

- **Q4 (listing pagination)**: 50 items per page in web (with "show more" link). TUI is scrollable (no hard page limit). CLI uses `--limit` (default 50) and `--offset` (default 0).
- **Q5 (overdue definition)**: Date-only comparison. A todo is overdue when `due_date < today's date` evaluated at UTC midnight (no time-of-day component). Todos with `due_date IS NULL` are never overdue.
- **Q6 (purge behavior)**: Resolved in `todo-lifecycle` and `todo-cli` specs. Immediate hard-delete, CLI requires `--yes` flag or interactive confirmation, web shows confirmation dialog.

## Requirements

### Requirement: Filter by status

The system MUST support filtering todos by `status` (`open` | `done`). When no status filter is provided, the default SHALL be `open` only (done todos are excluded from the default view).

#### Scenario: List open todos

- **WHEN** listing with no status filter
- **THEN** only todos with `status = 'open'` and `deleted_at IS NULL` are returned

#### Scenario: List done todos

- **WHEN** listing with status filter `done`
- **THEN** only todos with `status = 'done'` and `deleted_at IS NULL` are returned

### Requirement: Filter by priority

The system MUST support filtering by `priority` (`low` | `med` | `high`). This filter is combinable with status and other filters.

#### Scenario: List high-priority open todos

- **WHEN** listing with priority `high` and default status (open)
- **THEN** only open todos with `priority = 'high'` are returned

### Requirement: Filter by category

The system MUST support filtering by category slug (resolved via JOIN on `categories`). A todo with `category_id IS NULL` SHALL NOT match any category filter.

#### Scenario: List by category slug

- **WHEN** listing with category slug `"trabajo"`
- **THEN** only todos whose `category_id` references a category with slug `"trabajo"` are returned

### Requirement: Filter by tag

The system MUST support filtering by tag name (resolved via JOIN on `todo_tags` + `tags`). A todo matches if it has at least one tag with the given name.

#### Scenario: List by tag name

- **WHEN** listing with tag `"urgente"`
- **THEN** only todos that have a tag named `"urgente"` in `todo_tags` are returned

### Requirement: Filter overdue

The system MUST support an `overdue` filter that returns todos where `due_date < today` (date-only, UTC midnight) and `status = 'open'`. Todos with `due_date IS NULL` are excluded.

#### Scenario: List overdue todos

- **WHEN** listing with overdue filter and today is `2026-06-08`
- **THEN** only open todos with `due_date < '2026-06-08'` and `due_date IS NOT NULL` are returned

### Requirement: Filter by due_before

The system MUST support a `due_before` filter accepting an ISO-8601 date. Returns todos where `due_date < given_date`.

#### Scenario: List todos due before a date

- **WHEN** listing with `due_before = '2026-07-01'`
- **THEN** only todos with `due_date < '2026-07-01'` and `due_date IS NOT NULL` are returned

### Requirement: Filter trashed

The system MUST support a `trashed` flag that inverts the soft-delete filter: when `true`, only rows with `deleted_at IS NOT NULL` are returned. When `false` (default), only rows with `deleted_at IS NULL` are returned.

#### Scenario: List trashed todos

- **WHEN** listing with `trashed = true`
- **THEN** only todos with `deleted_at IS NOT NULL` are returned, regardless of status

### Requirement: Default sort order

The system MUST sort results by `due_date ASC NULLS LAST`, then `created_at DESC`, then `id DESC`. This ensures urgent items appear first, then newest items.

#### Scenario: Sort order with mixed due dates

- **WHEN** listing open todos where some have `due_date` and others have `due_date IS NULL`
- **THEN** todos with a due date appear first, sorted ascending by date
- **AND** todos without a due date appear after, sorted by `created_at DESC`

### Requirement: Pagination

The system MUST support `limit` and `offset` parameters. Default limit is 50. Offset defaults to 0.

#### Scenario: Paginate with limit and offset

- **WHEN** listing with `limit = 10` and `offset = 20`
- **THEN** at most 10 rows are returned, skipping the first 20 matching rows

## Out of Scope

- Full-text search (handled by `todo-search`).
- Aggregated views (overdue count, done-today count) — those are sidebar/web concerns computed at the handler level.
- Export to file (deferred; resources export exists but todos export is not in v3.0).
- Server-side push or live refresh of the list.
