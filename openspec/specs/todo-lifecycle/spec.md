# todo-lifecycle

## Purpose

Defines the create, read, update, soft-delete, restore, and purge operations on the `todos` table. This is the foundational capability every other todo capability depends on — status transitions, listing, search, tags, and all three surfaces (CLI, TUI, web) route through this lifecycle.

## Requirements

### Requirement: Create todo

The system MUST create a todo with a title validated by `domain.ValidateTitle`, an optional description, priority (`low` | `med` | `high`, default `med`), optional due date (ISO-8601 `YYYY-MM-DD`), optional category FK, optional notes, and recurrence (`none` | `daily` | `weekly` | `monthly`, default `none`). On success the system SHALL return the created row with status `open`, `done_at = NULL`, and `deleted_at = NULL`. Tags MUST be attached via `domain.WithTags` with `pruneOrphans=false` on create (no orphans possible for a new row).

#### Scenario: Create with all fields

- **WHEN** a todo is created with title `"pagar luz"`, priority `high`, due_date `2026-06-10`, recurrence `weekly`, notes `"mensual"`, and tags `["urgente", "casa"]`
- **THEN** a row is inserted with status `open`, priority `high`, due_date `2026-06-10`, recurrence `weekly`, `done_at IS NULL`, `deleted_at IS NULL`
- **AND** two tag rows exist in `todo_tags` linking to `urgente` and `casa`

#### Scenario: Create with defaults

- **WHEN** a todo is created with only title `"leer ADR"`
- **THEN** priority is `med`, recurrence is `none`, description is `NULL`, due_date is `NULL`, category_id is `NULL`

#### Scenario: Reject empty title

- **WHEN** a todo is created with title `""` or `"   "`
- **THEN** the operation fails with error `"title is required"` and no row is inserted

#### Scenario: Reject title exceeding max length

- **WHEN** a todo is created with a title longer than `domain.MaxTitleLength` (500 chars)
- **THEN** the operation fails with error `"title exceeds 500 chars"` and no row is inserted

### Requirement: Update todo

The system MUST replace all mutable fields of an existing todo (title, description, priority, due_date, category_id, notes, recurrence) and re-attach tags via `domain.WithTags` with `pruneOrphans=true`. The title MUST pass `domain.ValidateTitle`. The `updated_at` column SHALL be auto-bumped by the database trigger.

#### Scenario: Update changes priority

- **WHEN** a todo with priority `low` is updated to priority `high`
- **THEN** the row's priority is `high` and `updated_at` is greater than the previous value

#### Scenario: Update with new tag list prunes orphans

- **WHEN** a todo with tags `["a", "b"]` is updated to tags `["c"]`
- **THEN** `todo_tags` contains only `["c"]` for that todo
- **AND** `domain.WithTags` is called with `pruneOrphans=true`, so orphan tag rows (if no other owner references them) are deleted from the `tags` table

#### Scenario: Update non-existent todo fails

- **WHEN** an update targets a todo ID that does not exist
- **THEN** the operation returns an error and no rows are modified

### Requirement: Soft-delete todo

The system MUST set `deleted_at` to the current UTC timestamp when soft-deleting. A todo that is already soft-deleted SHALL NOT have its `deleted_at` overwritten (idempotent guard: `WHERE deleted_at IS NULL`).

#### Scenario: Soft-delete an active todo

- **WHEN** a todo with `deleted_at IS NULL` is soft-deleted
- **THEN** `deleted_at` is set to a non-NULL ISO-8601 timestamp
- **AND** the todo no longer appears in default (non-trashed) listings

#### Scenario: Soft-delete an already-deleted todo is a no-op

- **WHEN** a todo with `deleted_at IS NOT NULL` is soft-deleted again
- **THEN** `deleted_at` is unchanged and no error is returned

### Requirement: Restore todo

The system MUST clear `deleted_at` to `NULL` when restoring. A todo that is not soft-deleted SHALL NOT be affected (guard: `WHERE deleted_at IS NOT NULL`).

#### Scenario: Restore a soft-deleted todo

- **WHEN** a todo with `deleted_at IS NOT NULL` is restored
- **THEN** `deleted_at` is set to `NULL`
- **AND** the todo reappears in default listings

#### Scenario: Restore an active todo is a no-op

- **WHEN** a todo with `deleted_at IS NULL` is restored
- **THEN** no columns change and no error is returned

### Requirement: Purge todo

The system MUST hard-delete the todo row from the database. This is irreversible. Cascade deletes remove associated `todo_tags` rows. The FTS5 sync trigger removes the `todos_fts` entry.

#### Scenario: Purge after soft-delete

- **WHEN** a soft-deleted todo is purged
- **THEN** the row is removed from `todos`, `todo_tags`, and `todos_fts`
- **AND** a subsequent read by ID returns not-found

#### Scenario: Purge an active todo

- **WHEN** an active (non-deleted) todo is purged
- **THEN** the row is hard-deleted regardless of `deleted_at` state

## Out of Scope

- Auto-expansion of recurring todos on completion (deferred to v3.x, see `todo-recurrence-placeholder`).
- Cascade soft-delete (todos are independent; no parent-child relationships in v3.0).
- Audit trail or history log of field changes.
- Bulk operations (create/update/delete multiple todos in one call).
