# calendar-provider

## Purpose

`CalendarProvider` implements `today.Provider` to contribute calendar sections to the Today view. Provides "Today's Events" (events whose `start_at` falls within the current day in the user's timezone) and "Upcoming Events" (events within today+1 to today+7). Both sections are omitted when empty. Registered in `today.Service` alongside existing providers.

## Requirements

### Requirement: Provider interface implementation

The system MUST provide `CalendarProvider` implementing `today.Provider` with `Name() = "calendar"`. The provider SHALL be registered in `today.Service` via `todaySvc.Register(providers.NewCalendarProvider(db))` in both `internal/web/handlers.go::newHandlers()` and `internal/tui/app.go::New()`.

#### Scenario: Provider name is calendar

- **WHEN** `CalendarProvider.Name()` is called
- **THEN** it returns `"calendar"`

#### Scenario: Provider is registered in Today service

- **GIVEN** the Today service is initialized
- **WHEN** providers are listed
- **THEN** `CalendarProvider` appears alongside `TodosProvider`, `ResourcesProvider`, and `FinanceProvider`

### Requirement: Today's Events section

The system MUST return a section with key `"events-today"` and title `"Today's Events"`. The section SHALL contain all non-trashed events where `start_at` falls within the current day in the user's configured timezone (`KeyUserTimezone`). All-day events with `start_at = today` (date-only) MUST be included. Timed events MUST be included when their `start_at` datetime falls within the boundaries of today (00:00:00 to 23:59:59 local time). The section MUST be omitted when empty.

#### Scenario: Today's Events includes timed event starting today

- **GIVEN** `KeyUserTimezone` is `"America/Bogota"` (UTC−5) and local date is `2026-06-15`
- **AND** a timed event with `start_at="2026-06-15T09:00:00"` exists
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the "events-today" section contains that event

#### Scenario: Today's Events includes all-day event for today

- **GIVEN** local date is `2026-06-15`
- **AND** an all-day event with `start_at="2026-06-15"` and `all_day=true` exists
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the "events-today" section contains that event

#### Scenario: Today's Events excludes events on other days

- **GIVEN** local date is `2026-06-15`
- **AND** an event with `start_at="2026-06-16T09:00:00"` exists
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the "events-today" section does NOT contain that event

#### Scenario: Today's Events is omitted when no events today

- **GIVEN** no events have `start_at` on the current local date
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the "events-today" section is NOT included in the returned slice

#### Scenario: Today's Events excludes soft-deleted events

- **GIVEN** a soft-deleted event with `start_at` today
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the deleted event does NOT appear in "events-today"

### Requirement: Upcoming Events section

The system MUST return a section with key `"events-upcoming"` and title `"Upcoming Events"`. The section SHALL contain all non-trashed events where `start_at` falls within today+1 to today+7 (inclusive) in the user's configured timezone. All-day and timed events follow the same inclusion rules as the "events-today" section but scoped to the upcoming window. The section MUST be omitted when empty.

#### Scenario: Upcoming Events includes events in the next 7 days

- **GIVEN** local date is `2026-06-15`
- **AND** events exist with `start_at` on `2026-06-16`, `2026-06-20`, and `2026-06-22`
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the "events-upcoming" section contains the events for Jun 16 and Jun 20
- **AND** the event for Jun 22 (beyond today+7) is NOT included

#### Scenario: Upcoming Events boundary is today+7

- **GIVEN** local date is `2026-06-15`
- **AND** an event with `start_at="2026-06-22"` (exactly today+7) exists
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the "events-upcoming" section contains that event

#### Scenario: Upcoming Events is omitted when no upcoming events

- **GIVEN** no events exist within today+1 to today+7
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the "events-upcoming" section is NOT included

### Requirement: Item mapping

Items in both sections MUST map to the common `Item` shape with: `Domain="calendar"`, `Title=event.Title`, `Subtitle` (formatted as `"HH:MM–HH:MM"` for timed events with end_at, `"HH:MM"` for timed events without end_at, `"All day"` for all-day events, optionally appended with `" · {location}"` when location is non-empty), `Tags=event.Tags`, `URL="/calendar/{id}"`.

#### Scenario: Timed event with end_at maps subtitle as time range

- **GIVEN** an event with `start_at="2026-06-15T09:00:00"`, `end_at="2026-06-15T10:30:00"`, `all_day=false`, `location=""`
- **WHEN** the event is mapped to an `Item`
- **THEN** `Subtitle = "09:00–10:30"`

#### Scenario: Timed event without end_at maps subtitle as start time only

- **GIVEN** an event with `start_at="2026-06-15T09:00:00"`, `end_at=NULL`, `all_day=false`
- **WHEN** the event is mapped to an `Item`
- **THEN** `Subtitle = "09:00"`

#### Scenario: All-day event maps subtitle as "All day"

- **GIVEN** an event with `all_day=true` and no location
- **WHEN** the event is mapped to an `Item`
- **THEN** `Subtitle = "All day"`

#### Scenario: Location appended to subtitle when non-empty

- **GIVEN** an event with `start_at="2026-06-15T09:00:00"`, `end_at="2026-06-15T10:00:00"`, `location="Office"`
- **WHEN** the event is mapped to an `Item`
- **THEN** `Subtitle = "09:00–10:00 · Office"`

#### Scenario: Item URL points to calendar detail route

- **GIVEN** an event with `id = 7`
- **WHEN** the event is mapped to an `Item`
- **THEN** `URL = "/calendar/7"`

### Requirement: Timezone-aware day boundaries

The provider MUST compute "today" and the upcoming window using `today.UserLocation(ctx, db)`. When `KeyUserTimezone` is unset or contains an invalid IANA value, the provider SHALL fall back to UTC silently.

#### Scenario: Provider respects configured timezone

- **GIVEN** `KeyUserTimezone` is `"America/Bogota"` (UTC−5)
- **AND** current UTC time is 2026-06-16 03:00 (2026-06-15 22:00 local)
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** "today" is `2026-06-15` (local) and events matching that date appear in "events-today"

#### Scenario: Provider falls back to UTC when timezone invalid

- **GIVEN** `KeyUserTimezone` is set to `"Invalid/Zone"`
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the provider uses UTC to determine day boundaries and does not return an error

### Requirement: Graceful error degradation

When `CalendarProvider.Sections(ctx)` returns an error, the provider SHALL return the error to the Registry. The Registry SHALL skip calendar sections and render a muted indicator. This follows REQ-TV-06.

#### Scenario: Provider error skips calendar sections

- **GIVEN** the database is locked when `CalendarProvider.Sections(ctx)` is called
- **WHEN** the Today view is requested
- **THEN** calendar sections are omitted
- **AND** a muted "Calendar unavailable" indicator is shown
- **AND** all other providers' sections render normally

## Out of Scope

- Recurrence expansion in the provider, multi-day event spanning, cross-domain event aggregation (todos due dates), real-time push updates.
