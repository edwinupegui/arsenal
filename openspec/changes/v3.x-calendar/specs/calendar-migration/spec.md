# calendar-migration

## Purpose

Schema migration for the Calendar domain. Creates `calendar_events`, `calendar_tags`, `calendar_fts` (FTS5 virtual table), FTS sync triggers, and indices. Forward-only migration per ADR-0001.

## Requirements

### Requirement: calendar_events table

The migration MUST create the `calendar_events` table with columns: `id` (INTEGER PRIMARY KEY AUTOINCREMENT), `title` (TEXT NOT NULL), `description` (TEXT), `start_at` (TEXT NOT NULL — format `YYYY-MM-DDTHH:MM:SS` for timed events, `YYYY-MM-DD` for all-day events), `end_at` (TEXT, nullable — NULL means open-ended; maps to iCal DTEND), `all_day` (INTEGER NOT NULL DEFAULT 0, CHECK: 0 or 1), `location` (TEXT), `category_id` (INTEGER, FK → `categories.id`, nullable), `notes` (TEXT), `recurrence` (TEXT NOT NULL DEFAULT 'none', CHECK: `none`|`daily`|`weekly`|`monthly`|`yearly`), `created_at` (TEXT NOT NULL DEFAULT strftime), `updated_at` (TEXT NOT NULL DEFAULT strftime), `deleted_at` (TEXT).

#### Scenario: Table created on fresh install

- **WHEN** the migration runs on a fresh database
- **THEN** `calendar_events` exists with all specified columns and constraints

#### Scenario: CHECK constraint rejects invalid recurrence

- **WHEN** an INSERT attempts `recurrence = 'biweekly'`
- **THEN** SQLite rejects the row with a CHECK constraint failure

#### Scenario: CHECK constraint rejects invalid all_day

- **WHEN** an INSERT attempts `all_day = 2`
- **THEN** SQLite rejects the row with a CHECK constraint failure

#### Scenario: Nullable end_at allows open-ended events

- **WHEN** an INSERT with `end_at = NULL` is executed
- **THEN** the row is stored successfully with `end_at IS NULL`

#### Scenario: All-day event stores date-only start_at

- **WHEN** an INSERT with `all_day = 1` and `start_at = '2026-06-15'` is executed
- **THEN** the row is stored with `start_at = '2026-06-15'`

### Requirement: calendar_tags junction table

The migration MUST create `calendar_tags` with columns: `calendar_id` (INTEGER NOT NULL, FK → `calendar_events.id`), `tag_id` (INTEGER NOT NULL, FK → `tags.id`), `PRIMARY KEY (calendar_id, tag_id)`.

#### Scenario: Junction table links events and tags

- **WHEN** a row is inserted into `calendar_tags` with valid `calendar_id` and `tag_id`
- **THEN** the row exists linking the event to the tag

#### Scenario: Junction table rejects duplicate pairs

- **WHEN** a duplicate `(calendar_id, tag_id)` pair is inserted
- **THEN** SQLite rejects the row with a PRIMARY KEY constraint failure

### Requirement: calendar_fts FTS5 virtual table

The migration MUST create `calendar_fts` as an FTS5 virtual table on columns `title`, `description`, and `location`. Sync triggers MUST fire AFTER INSERT, AFTER UPDATE, and AFTER DELETE on `calendar_events` to keep the FTS index consistent. The FTS virtual table MUST NOT use `IF NOT EXISTS` (SQLite limitation, same as finance).

#### Scenario: FTS5 index syncs on insert

- **WHEN** an event is inserted with title `"team standup"` and location `"office"`
- **THEN** a search for `"standup"` in `calendar_fts` returns the event
- **AND** a search for `"office"` in `calendar_fts` returns the event

#### Scenario: FTS5 index syncs on update

- **WHEN** an event's title is updated from `"standup"` to `"retrospective"`
- **THEN** searching `"standup"` no longer returns the event
- **AND** searching `"retrospective"` returns it

#### Scenario: FTS5 index syncs on delete

- **WHEN** an event is deleted
- **THEN** the corresponding FTS entry is removed

#### Scenario: FTS5 searches location column

- **WHEN** an event with `location = "Conference Room A"` is inserted
- **THEN** a search for `"Conference"` in `calendar_fts` returns the event

### Requirement: Indices

The migration MUST create indices: `idx_calendar_start` on `start_at`, `idx_calendar_deleted` on `deleted_at`, `idx_calendar_category` on `category_id`.

#### Scenario: Indices exist after migration

- **WHEN** the migration completes
- **THEN** all three indices exist and are queryable

### Requirement: IF NOT EXISTS safety

All CREATE TABLE and CREATE TRIGGER statements MUST use `IF NOT EXISTS` guards so re-runs are safe. The FTS5 `CREATE VIRTUAL TABLE` MUST NOT use `IF NOT EXISTS` (unsupported by SQLite).

#### Scenario: Re-running table/trigger statements is safe

- **WHEN** the migration's CREATE TABLE and CREATE TRIGGER statements are executed twice
- **THEN** no errors occur on the second run

### Requirement: Forward-only

The migration MUST NOT include a DOWN migration. Rollback instructions MAY be included as SQL comments. The migration file MUST include a comment documenting the single-timezone storage assumption for `start_at`.

#### Scenario: No down migration exists

- **WHEN** inspecting the migration file
- **THEN** no destructive DROP statements exist outside of SQL comments

#### Scenario: Timezone assumption is documented

- **WHEN** inspecting the migration file
- **THEN** a comment states that `start_at` is stored as local time without tz offset and that changing `KeyUserTimezone` reinterprets historical values

## Out of Scope

- Down migration, data migration from external sources, recurrence expansion, multi-timezone schema changes.
