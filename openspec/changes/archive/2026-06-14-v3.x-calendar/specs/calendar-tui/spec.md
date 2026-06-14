# calendar-tui

## Purpose

TUI sub-area for Calendar, replacing the `areaCalendar` placeholder in `internal/tui/app.go` with a functional sub-model. Provides an event list, detail view, CRUD keybindings, and status bar integration following the `areaTodos` and `areaFinance` patterns.

## Requirements

### Requirement: Calendar sub-model replaces placeholder

The system MUST replace the `areaCalendar` placeholder in `internal/tui/app.go` with a `calendarModel` sub-model. The `Update()` method SHALL route `areaCalendar` messages to `updateCalendar()`. The `View()` method SHALL render the event list or detail view. Key `5` and Tab/Shift-Tab navigation SHALL reach the Calendar area.

#### Scenario: Calendar area renders event list

- **WHEN** the user switches to the Calendar area
- **THEN** a scrollable list of events is displayed, sorted by `start_at ASC` (earliest first)

#### Scenario: Placeholder message no longer appears

- **WHEN** the user switches to the Calendar area
- **THEN** the "Calendar (coming soon — v3.x)" message is NOT displayed

#### Scenario: Key 5 activates Calendar area

- **WHEN** the user presses `5` in any area
- **THEN** the Calendar area is activated

### Requirement: Keybindings

The system MUST support these keybindings within the Calendar area: `n` (new event), `e` (edit selected), `d` (soft-delete selected), `r` (restore selected, in trashed view), `x` (purge selected, requires confirmation), `j`/`k` (navigate down/up), `enter` (detail view), `Tab` (area switch).

#### Scenario: Navigate with j/k

- **WHEN** the user presses `j` in the calendar list
- **THEN** the selection moves to the next event

#### Scenario: Navigate up with k

- **WHEN** the user presses `k` in the calendar list
- **THEN** the selection moves to the previous event

#### Scenario: Create with n

- **WHEN** the user presses `n` in the Calendar area
- **THEN** a new-event form is displayed

#### Scenario: Soft-delete with d

- **WHEN** the user presses `d` on a selected event
- **THEN** the event is soft-deleted and the list refreshes without the deleted event

#### Scenario: Restore with r in trashed view

- **WHEN** the user is viewing trashed events and presses `r`
- **THEN** the selected event is restored and the list refreshes

#### Scenario: Purge with x requires confirmation

- **WHEN** the user presses `x` on a selected trashed event
- **THEN** a confirmation prompt is shown before hard-deleting

#### Scenario: Enter opens detail view

- **WHEN** the user presses `enter` on a selected event
- **THEN** the detail view is rendered for that event

### Requirement: Status bar context hints

The system MUST display "Calendar" as the area name in the status bar with keybinding hints: `n` new, `e` edit, `d` delete, `Tab` switch.

#### Scenario: Status bar shows calendar hints

- **WHEN** the user is in the Calendar area
- **THEN** the status bar shows "Calendar" and relevant key hints (`n`, `e`, `d`, `Tab`)

### Requirement: Detail view on Enter

The system MUST render a detail view showing all event fields (title, description, start_at, end_at, all_day, location, category, tags, notes, recurrence, timestamps) when `enter` is pressed on a selected event. All-day events MUST display start_at as date-only. Timed events MUST display start_at and end_at in human-readable form.

#### Scenario: Detail view displays all-day event correctly

- **GIVEN** an all-day event with `start_at="2026-06-15"` and `all_day=true`
- **WHEN** the user presses `enter` on the event
- **THEN** the detail view shows `"All day"` and `"2026-06-15"` (no time component)

#### Scenario: Detail view displays timed event with end_at

- **GIVEN** a timed event with `start_at="2026-06-15T09:00:00"` and `end_at="2026-06-15T09:30:00"`
- **WHEN** the user presses `enter` on the event
- **THEN** the detail view shows `09:00` and `09:30` alongside the date

#### Scenario: Detail view displays open-ended event

- **GIVEN** a timed event with `end_at = NULL`
- **WHEN** the user presses `enter` on the event
- **THEN** the detail view does NOT display an end time or shows `"—"` in its place

#### Scenario: Detail view shows tags

- **GIVEN** an event with tags `["work", "weekly"]`
- **WHEN** the user presses `enter` on the event
- **THEN** the detail view shows both tags

### Requirement: List item formatting

Each event in the list view MUST display: title, date/time indicator (all-day badge or time range), and optionally location when non-empty.

#### Scenario: All-day event shows date badge

- **GIVEN** an all-day event with `start_at="2026-06-15"`
- **WHEN** the Calendar area is viewed
- **THEN** the list item shows `"All day"` alongside the date

#### Scenario: Timed event shows time range

- **GIVEN** a timed event with `start_at="2026-06-15T09:00:00"` and `end_at="2026-06-15T09:30:00"`
- **WHEN** the Calendar area is viewed
- **THEN** the list item shows `"09:00–09:30"`

## Out of Scope

- Mouse support, persistent area preference, inline editing in list view, calendar grid view.
