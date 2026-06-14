# Delta for today-providers

## ADDED Requirements

### REQ-TP-08: CalendarProvider contributes two sections

The system MUST provide a `CalendarProvider` that implements the `Provider` interface and returns up to two sections: "events-today" (title "Today's Events" — non-trashed events whose `start_at` falls within the current local day) and "events-upcoming" (title "Upcoming Events" — non-trashed events whose `start_at` falls within today+1 to today+7). The `CalendarProvider` MUST compute day boundaries using the user's configured timezone via `internal/today.UserLocation`, consistent with REQ-TP-01.

The `CalendarProvider` MUST be registered in `today.Service` alongside `TodosProvider`, `ResourcesProvider`, and `FinanceProvider`.

Section keys: `"events-today"` (order 7) and `"events-upcoming"` (order 8).

#### Scenario: CalendarProvider returns events-today section

- **GIVEN** local date is `2026-06-15` and 3 non-trashed events have `start_at` on that date
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** a section with key `"events-today"` and title `"Today's Events"` is returned
- **AND** it contains the 3 events

#### Scenario: CalendarProvider returns events-upcoming section

- **GIVEN** local date is `2026-06-15` and 4 non-trashed events have `start_at` between `2026-06-16` and `2026-06-22`
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** a section with key `"events-upcoming"` and title `"Upcoming Events"` is returned
- **AND** it contains the 4 events

#### Scenario: CalendarProvider omits empty sections

- **GIVEN** no events exist for today or the next 7 days
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** the returned slice is empty (no calendar sections)

#### Scenario: CalendarProvider omits events-today when none today but upcoming exist

- **GIVEN** no events exist for today but 2 events exist within today+1..today+7
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** only the "events-upcoming" section is returned (no "events-today" section)

#### Scenario: CalendarProvider error skips sections gracefully

- **GIVEN** the database is locked when `CalendarProvider.Sections(ctx)` is called
- **WHEN** the Today view is requested
- **THEN** calendar sections are omitted
- **AND** a muted "Calendar unavailable" indicator is shown
- **AND** all other providers' sections render normally

#### Scenario: CalendarProvider respects user timezone

- **GIVEN** `KeyUserTimezone` is `"America/Bogota"` (UTC−5)
- **AND** current UTC time is 2026-06-16 03:00 (2026-06-15 22:00 local)
- **WHEN** `CalendarProvider.Sections(ctx)` is called
- **THEN** "today" is `2026-06-15` (local date) and matching events appear in "events-today"

#### Scenario: Item mapping for calendar event

- **GIVEN** an event with `id=7`, `title="Team standup"`, `start_at="2026-06-15T09:00:00"`, `end_at="2026-06-15T09:30:00"`, `all_day=false`, `location="Office"`, `tags=["work"]`
- **WHEN** the event is mapped to an `Item`
- **THEN** `Domain="calendar"`, `Title="Team standup"`, `Subtitle="09:00–09:30 · Office"`, `Tags=["work"]`, `URL="/calendar/7"`

#### Scenario: All-day event item mapping

- **GIVEN** an all-day event with `id=8`, `title="Company holiday"`, `all_day=true`, `start_at="2026-06-15"`, `location=""`
- **WHEN** the event is mapped to an `Item`
- **THEN** `Subtitle="All day"`
