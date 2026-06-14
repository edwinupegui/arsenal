# calendar-ical-export

## Purpose

iCal (.ics) export for calendar events. Produces RFC 5545-compliant VCALENDAR output via `arsenal calendar export --format ical` to stdout or `--output <path>`. Events only (VEVENT); no VTODO. Implemented with Go stdlib only (no external libraries).

## Requirements

### Requirement: VCALENDAR envelope

The system MUST wrap all VEVENT blocks in a VCALENDAR envelope. The output MUST begin with `BEGIN:VCALENDAR` and end with `END:VCALENDAR`. The envelope MUST include `VERSION:2.0` and `PRODID:-//Arsenal//Calendar//EN`.

#### Scenario: Output starts and ends with VCALENDAR envelope

- **WHEN** `arsenal calendar export --format ical` is run
- **THEN** the first line is `BEGIN:VCALENDAR`
- **AND** the last line is `END:VCALENDAR`

#### Scenario: VCALENDAR envelope includes VERSION and PRODID

- **WHEN** the export is run
- **THEN** the output contains `VERSION:2.0`
- **AND** the output contains `PRODID:-//Arsenal//Calendar//EN`

### Requirement: VEVENT blocks

The system MUST produce one VEVENT block per exported event. Each VEVENT MUST include: `UID` (derived from event ID, e.g. `{id}@arsenal`), `SUMMARY` (event title), `DTSTART` (formatted from `start_at`), `DTSTAMP` (export time in UTC). `DESCRIPTION` MUST be included when non-empty. `LOCATION` MUST be included when non-empty. `DTEND` MUST be included when `end_at` is non-NULL. `RRULE` MUST be included when `recurrence != "none"`.

#### Scenario: VEVENT contains required fields

- **GIVEN** an event with id=5, title="Team standup", `start_at="2026-06-15T09:00:00"`, `end_at="2026-06-15T09:30:00"`, `recurrence="none"`
- **WHEN** the event is exported
- **THEN** the VEVENT block contains `UID:5@arsenal`, `SUMMARY:Team standup`, `DTSTART:20260615T090000`, `DTEND:20260615T093000`

#### Scenario: VEVENT omits DTEND when end_at is NULL

- **GIVEN** an event with `end_at = NULL`
- **WHEN** the event is exported
- **THEN** the VEVENT block does NOT contain a DTEND line

#### Scenario: VEVENT includes DESCRIPTION when non-empty

- **GIVEN** an event with `description = "Daily sync"`
- **WHEN** the event is exported
- **THEN** the VEVENT block contains `DESCRIPTION:Daily sync`

#### Scenario: VEVENT omits DESCRIPTION when empty

- **GIVEN** an event with `description = ""`
- **WHEN** the event is exported
- **THEN** the VEVENT block does NOT contain a DESCRIPTION line

#### Scenario: VEVENT includes LOCATION when non-empty

- **GIVEN** an event with `location = "Conference Room A"`
- **WHEN** the event is exported
- **THEN** the VEVENT block contains `LOCATION:Conference Room A`

### Requirement: All-day event DATE value type

When `all_day = true`, the system MUST use `DATE` value type for DTSTART (format `YYYYMMDD`, no time component). DTEND, when present for an all-day event, MUST also use `DATE` value type.

#### Scenario: All-day event DTSTART uses DATE type

- **GIVEN** an event with `all_day=true` and `start_at="2026-06-15"`
- **WHEN** the event is exported
- **THEN** the VEVENT block contains `DTSTART;VALUE=DATE:20260615`
- **AND** no time component appears in the DTSTART value

#### Scenario: All-day event DTEND uses DATE type

- **GIVEN** an all-day event with `end_at="2026-06-16"`
- **WHEN** the event is exported
- **THEN** the VEVENT block contains `DTEND;VALUE=DATE:20260616`

### Requirement: RRULE from recurrence enum

The system MUST map the `recurrence` field to an iCal RRULE as follows: `daily` → `RRULE:FREQ=DAILY`, `weekly` → `RRULE:FREQ=WEEKLY`, `monthly` → `RRULE:FREQ=MONTHLY`, `yearly` → `RRULE:FREQ=YEARLY`. When `recurrence = "none"`, no RRULE line is included.

#### Scenario: Daily recurrence maps to FREQ=DAILY

- **GIVEN** an event with `recurrence = "daily"`
- **WHEN** the event is exported
- **THEN** the VEVENT block contains `RRULE:FREQ=DAILY`

#### Scenario: Weekly recurrence maps to FREQ=WEEKLY

- **GIVEN** an event with `recurrence = "weekly"`
- **WHEN** the event is exported
- **THEN** the VEVENT block contains `RRULE:FREQ=WEEKLY`

#### Scenario: Monthly recurrence maps to FREQ=MONTHLY

- **GIVEN** an event with `recurrence = "monthly"`
- **WHEN** the event is exported
- **THEN** the VEVENT block contains `RRULE:FREQ=MONTHLY`

#### Scenario: Yearly recurrence maps to FREQ=YEARLY

- **GIVEN** an event with `recurrence = "yearly"`
- **WHEN** the event is exported
- **THEN** the VEVENT block contains `RRULE:FREQ=YEARLY`

#### Scenario: No recurrence produces no RRULE

- **GIVEN** an event with `recurrence = "none"`
- **WHEN** the event is exported
- **THEN** the VEVENT block does NOT contain an RRULE line

### Requirement: Output destination

The system MUST write iCal output to stdout by default. When `--output <path>` is provided, the system SHALL write to the specified file path instead.

#### Scenario: Default output to stdout

- **WHEN** the user runs `arsenal calendar export --format ical`
- **THEN** iCal content is written to stdout

#### Scenario: Output to file with --output

- **WHEN** the user runs `arsenal calendar export --format ical --output /tmp/events.ics`
- **THEN** the file `/tmp/events.ics` contains the iCal content
- **AND** nothing is written to stdout

### Requirement: Filter support

The export MUST support `--from` and `--to` date range filters applied to `start_at`. Filters combine with the same logic as `calendar-service` List.

#### Scenario: Export filtered by date range

- **GIVEN** 10 events spanning June and July 2026
- **WHEN** the user runs `arsenal calendar export --format ical --from 2026-06-01 --to 2026-06-30`
- **THEN** only June events appear as VEVENT blocks in the output

### Requirement: Empty export

When no events match the filter, the system MUST still produce a valid VCALENDAR envelope with no VEVENT blocks.

#### Scenario: Empty export produces valid envelope only

- **GIVEN** zero events exist
- **WHEN** the user runs `arsenal calendar export --format ical`
- **THEN** the output is exactly `BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:...\nEND:VCALENDAR`

### Requirement: Stdlib-only implementation

The iCal export MUST be implemented using Go standard library only. No third-party iCal formatting libraries SHALL be introduced.

#### Scenario: No external ical dependency in go.mod

- **WHEN** `go.mod` is inspected after implementation
- **THEN** no ical-specific third-party module is present

## Out of Scope

- VTODO export, calendar import/parsing, VALARM (reminders), ATTENDEE, ORGANIZER fields, time zone VTIMEZONE components.
