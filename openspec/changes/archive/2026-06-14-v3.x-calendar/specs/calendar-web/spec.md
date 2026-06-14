# calendar-web

## Purpose

Web routes, templates, and sidebar integration for Calendar. Provides HTMX-powered CRUD with card-based list views, inline lifecycle actions, sidebar count badge, and an empty state. Follows the `finance-web` pattern. Inserted between Finance and Trash in the sidebar.

## Requirements

### Requirement: List and create routes

The system MUST serve `GET /calendar` (list with filters: date range, all-day, recurrence, category, tag, trashed) and `GET /calendar/new` + `POST /calendar` (create form and processing).

#### Scenario: List page renders events

- **WHEN** a user visits `GET /calendar`
- **THEN** a card-based list of events is rendered sorted by `start_at ASC`
- **AND** filter controls are visible (date range, recurrence, tag)

#### Scenario: Create form validates required fields

- **WHEN** a user submits `POST /calendar` without a title
- **THEN** the form is re-rendered with a validation error and no event is created

#### Scenario: Create form creates event and redirects

- **WHEN** a user submits `POST /calendar` with valid data (title, start_at)
- **THEN** the event is created and the user is redirected to `GET /calendar`

#### Scenario: Create form accepts all-day events

- **WHEN** a user submits `POST /calendar` with `all_day=true` and `start_at` as a date-only value
- **THEN** the event is created with `all_day=true` and `start_at` stored as `YYYY-MM-DD`

### Requirement: Detail and edit routes

The system MUST serve `GET /calendar/:id` (detail), `GET /calendar/:id/edit` (edit form), `POST /calendar/:id` (update processing).

#### Scenario: Show renders event detail

- **WHEN** a user visits `GET /calendar/3`
- **THEN** full event detail is rendered including title, start_at, end_at, all_day indicator, location, tags, recurrence, notes

#### Scenario: All-day event detail omits time

- **GIVEN** an all-day event with `all_day=true` and `start_at="2026-06-15"`
- **WHEN** a user visits `GET /calendar/{id}`
- **THEN** the detail page shows `"All day"` and the date `2026-06-15` without a time component

#### Scenario: Edit form updates event

- **WHEN** a user submits `POST /calendar/3` with an updated start_at
- **THEN** the event is updated and the user is redirected to `GET /calendar/3`

#### Scenario: Show non-existent event returns 404

- **WHEN** a user visits `GET /calendar/9999` and no event exists with that ID
- **THEN** the response status is 404

### Requirement: Lifecycle routes with HTMX

The system MUST serve `POST /calendar/:id/delete` (soft-delete), `POST /calendar/:id/restore` (restore), `POST /calendar/:id/purge` (hard-delete with confirmation). These endpoints SHALL return HTMX fragments for in-place card updates.

#### Scenario: Soft-delete removes card via HTMX

- **WHEN** a user clicks "Delete" on an event card
- **THEN** the event is soft-deleted and the card is removed from the list via HTMX

#### Scenario: Restore brings card back

- **WHEN** a user clicks "Restore" on a trashed event
- **THEN** the event is restored and reappears in the default list

#### Scenario: Purge shows confirmation

- **WHEN** viewing a trashed event
- **THEN** a "Purge permanently" button is visible with a confirmation dialog before hard-delete

### Requirement: Sidebar entry with count badge

The system MUST add a "Calendar" link to the web sidebar in `layout.html` positioned between "Finance" and "Trash". The link SHALL include a count badge reflecting the number of non-trashed events. The badge SHALL be hidden when count is zero.

#### Scenario: Sidebar shows calendar count badge

- **WHEN** any page is rendered and 8 non-trashed events exist
- **THEN** the sidebar includes "Calendar" with badge "8"

#### Scenario: Sidebar badge hidden when zero events

- **WHEN** any page is rendered and zero events exist
- **THEN** the sidebar includes "Calendar" with no visible badge

#### Scenario: Sidebar entry is positioned between Finance and Trash

- **WHEN** any page is rendered
- **THEN** the "Calendar" sidebar link appears after "Finance" and before "Trash"

### Requirement: commonPage includes CalendarCount

The system MUST extend `commonPage()` in `internal/web/handlers.go` to include a `CalendarCount` field (count of non-trashed events). All layout renders SHALL have access to this field for the sidebar badge.

#### Scenario: CalendarCount is available on every page

- **GIVEN** 5 non-trashed events exist
- **WHEN** any web page is rendered
- **THEN** the template receives `CalendarCount = 5`

### Requirement: Empty state

The system MUST render an empty state message when no events exist, with a link to the create form.

#### Scenario: Empty state shown on first visit

- **WHEN** a user visits `GET /calendar` and no events exist
- **THEN** an empty state message is rendered with an "Add event" link

## Out of Scope

- Real-time WebSocket updates, drag-and-drop calendar grid view, rich text notes, iCal subscribe endpoint.
