# calendar-service

## Purpose

Core domain service for Calendar. Defines the `Event` type, `Recurrence` enum, lifecycle operations (Create, Get, Update, SoftDelete, Restore, Purge, List, Export), tag attachment via `domain.WithTags`, and the `Attacher` implementation. All three surfaces (CLI, TUI, web) route through this service.

## Requirements

### Requirement: Event domain type

The system MUST define an `Event` struct with fields: `ID` (int64), `Title` (string), `Description` (string, nullable), `StartAt` (string — `YYYY-MM-DDTHH:MM:SS` for timed events, `YYYY-MM-DD` when `AllDay = true`), `EndAt` (string, nullable — NULL means open-ended), `AllDay` (bool), `Location` (string, nullable), `CategoryID` (int64, nullable FK → `categories`), `Notes` (string, nullable), `Recurrence` (string, CHECK: `none`|`daily`|`weekly`|`monthly`|`yearly`, default `none`), `CreatedAt`, `UpdatedAt`, `DeletedAt` (nullable timestamp). Tags are attached via the `calendar_tags` junction table. `StartAt` is stored as local time without timezone offset per ADR-0003.

#### Scenario: Create timed event

- **WHEN** an event is created with `title="Team standup"`, `start_at="2026-06-15T09:00:00"`, `end_at="2026-06-15T09:30:00"`, `all_day=false`, `location="Office"`, `recurrence="none"`, `tags=["work"]`
- **THEN** a row is inserted with `deleted_at IS NULL`
- **AND** one row exists in `calendar_tags` linking to tag `"work"`

#### Scenario: Create all-day event

- **WHEN** an event is created with `all_day=true` and `start_at="2026-06-15"`
- **THEN** `start_at` is stored as `"2026-06-15"` (date-only, no time component)
- **AND** `all_day = 1` in the stored row

#### Scenario: Create open-ended event with NULL end_at

- **WHEN** an event is created with `end_at` omitted or explicitly NULL
- **THEN** the row is inserted with `end_at IS NULL`

#### Scenario: Reject invalid recurrence

- **WHEN** an event is created with `recurrence = "biweekly"`
- **THEN** the operation fails with a validation error and no row is inserted

#### Scenario: Reject invalid all_day and start_at combination

- **WHEN** an event is created with `all_day=true` and `start_at="2026-06-15T09:00:00"` (datetime instead of date-only)
- **THEN** the operation fails with a validation error

### Requirement: Update event

The system MUST replace all mutable fields (title, description, start_at, end_at, all_day, location, category_id, notes, recurrence) and re-attach tags via `domain.WithTags` with `pruneOrphans=true`. The `updated_at` column SHALL be auto-bumped by the database trigger.

#### Scenario: Update changes start_at and end_at

- **WHEN** an event with `start_at="2026-06-15T09:00:00"` is updated to `start_at="2026-06-16T10:00:00"` and `end_at="2026-06-16T11:00:00"`
- **THEN** the stored row reflects the new values

#### Scenario: Update clears end_at to NULL

- **WHEN** an event with `end_at="2026-06-15T10:00:00"` is updated with `end_at=NULL`
- **THEN** `end_at IS NULL` in the stored row

#### Scenario: Update changes tags

- **WHEN** an event with tags `["work"]` is updated to tags `["personal"]`
- **THEN** `calendar_tags` contains only `["personal"]` for that event

#### Scenario: Update non-existent event fails

- **WHEN** an update targets an ID that does not exist
- **THEN** the operation returns an error and no rows are modified

### Requirement: Soft-delete, restore, purge

The system MUST support soft-delete (set `deleted_at`), restore (clear `deleted_at`), and purge (hard-delete row + cascade `calendar_tags`). Soft-delete is idempotent (`WHERE deleted_at IS NULL`). Restore is idempotent (`WHERE deleted_at IS NOT NULL`).

#### Scenario: Soft-delete sets deleted_at

- **WHEN** an active event is soft-deleted
- **THEN** `deleted_at` is set to a non-NULL timestamp
- **AND** the event no longer appears in default listings

#### Scenario: Restore clears deleted_at

- **WHEN** a soft-deleted event is restored
- **THEN** `deleted_at` is `NULL` and the event reappears in default listings

#### Scenario: Purge hard-deletes row and removes from FTS

- **WHEN** an event is purged
- **THEN** the row is removed from `calendar_events`, `calendar_tags`, and `calendar_fts`

#### Scenario: Soft-delete on already-deleted event is idempotent

- **WHEN** a soft-deleted event is soft-deleted again
- **THEN** no error occurs and `deleted_at` remains set

### Requirement: List with filter

The system MUST provide a `List` method accepting filters: date range (`from`, `to` on `start_at`), `all_day` (bool, optional), `recurrence`, `category_id`, `tag` name, `trashed` (bool). Results SHALL be sorted by `start_at ASC`, then `created_at ASC`.

#### Scenario: List filtered by date range

- **GIVEN** 5 events in June 2026 and 3 in July 2026
- **WHEN** List is called with `from="2026-06-01"`, `to="2026-06-30"`
- **THEN** exactly 5 events are returned sorted by `start_at ASC`

#### Scenario: List all-day events only

- **WHEN** List is called with `all_day=true`
- **THEN** only events with `all_day = 1` are returned

#### Scenario: List trashed

- **WHEN** List is called with `trashed=true`
- **THEN** only soft-deleted events are returned

#### Scenario: List by tag

- **WHEN** List is called with `tag="work"`
- **THEN** only events tagged `"work"` are returned

### Requirement: Export

The system MUST provide an `Export` method that accepts the same filters as `List` and returns all matching non-trashed events (no density limit). The result is used as input to the iCal export surface.

#### Scenario: Export returns all matching events without truncation

- **GIVEN** 50 non-trashed events exist matching the filter
- **WHEN** `Export` is called with matching filters
- **THEN** all 50 events are returned

#### Scenario: Export excludes trashed events

- **GIVEN** 10 active and 5 soft-deleted events exist
- **WHEN** `Export` is called with no `trashed` filter
- **THEN** only the 10 active events are returned

### Requirement: Attacher for domain.WithTags

The system MUST provide `internal/calendar/attacher.go` implementing the `domain.Attacher` interface, mirroring `finance/attacher.go`. The attacher SHALL manage `calendar_tags` junction rows.

#### Scenario: Attacher creates junction rows

- **WHEN** `WithTags` is called with tag names `["meeting", "weekly"]` for event ID 3
- **THEN** two rows exist in `calendar_tags` linking event 3 to the corresponding tag IDs

### Requirement: Timezone storage assumption

`StartAt` MUST be stored as a local-time string without timezone offset, consistent with ADR-0003 (single-timezone assumption). The service MUST document that changing `KeyUserTimezone` reinterprets historical `start_at` values without migration.

#### Scenario: start_at stored without tz offset

- **WHEN** an event is created with `start_at="2026-06-15T09:00:00"`
- **THEN** the value stored is exactly `"2026-06-15T09:00:00"` with no offset suffix

## Out of Scope

- Multi-timezone storage, recurrence auto-expansion, recurring event instance generation, bulk operations, attendee management.
