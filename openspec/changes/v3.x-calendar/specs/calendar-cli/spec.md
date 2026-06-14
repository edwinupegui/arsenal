# calendar-cli

## Purpose

Cobra subcommands under `arsenal calendar` exposing all calendar operations from the command line. Primary surface for terminal-first users and shell automation via `--json` output.

## Requirements

### Requirement: Parent command and subcommand tree

The system MUST provide `arsenal calendar` with subcommands: `add`, `list`, `show`, `edit`, `rm`, `restore`, `purge`, `export`. The parent command SHALL print help when invoked without a subcommand. `newCalendarCmd()` MUST be registered in `internal/cli/root.go`.

#### Scenario: Calendar help lists subcommands

- **WHEN** the user runs `arsenal calendar`
- **THEN** available subcommands are printed: add, list, show, edit, rm, restore, purge, export

### Requirement: Add subcommand

The system MUST provide `arsenal calendar add` with flags: `--title` (string, required), `--start` (datetime or date, required), `--end` (datetime, optional), `--all-day` (bool, default false), `--location` (string, optional), `--cat` (category slug, optional), `--tag` (repeatable, optional), `--description` (string, optional), `--notes` (string, optional), `--recurrence` (none|daily|weekly|monthly|yearly, default none). Output is text by default, JSON with `--json`.

#### Scenario: Add timed event with all flags

- **WHEN** the user runs `arsenal calendar add --title "Team standup" --start "2026-06-15T09:00:00" --end "2026-06-15T09:30:00" --location "Office" --tag work --recurrence daily`
- **THEN** an event is created with all specified fields
- **AND** the CLI prints the created event's ID to stdout

#### Scenario: Add all-day event

- **WHEN** the user runs `arsenal calendar add --title "Company holiday" --start "2026-06-15" --all-day`
- **THEN** an event is created with `all_day=true` and `start_at="2026-06-15"`

#### Scenario: Add without required title fails

- **WHEN** the user runs `arsenal calendar add --start "2026-06-15T09:00:00"`
- **THEN** the CLI prints an error about missing `--title` and exits non-zero

#### Scenario: Add without required start fails

- **WHEN** the user runs `arsenal calendar add --title "Meeting"`
- **THEN** the CLI prints an error about missing `--start` and exits non-zero

#### Scenario: Add with invalid recurrence fails

- **WHEN** the user runs `arsenal calendar add --title "X" --start "2026-06-15T09:00:00" --recurrence biweekly`
- **THEN** the CLI prints an error about invalid recurrence and exits non-zero

### Requirement: List subcommand

The system MUST provide `arsenal calendar list` with flags: `--from`, `--to` (date range on `start_at`), `--all-day` (bool), `--recurrence`, `--cat`, `--tag`, `--trashed` (bool), `--json`.

#### Scenario: List with date range filter

- **WHEN** the user runs `arsenal calendar list --from 2026-06-01 --to 2026-06-30`
- **THEN** only events with `start_at` within June 2026 are printed

#### Scenario: List with JSON output

- **WHEN** the user runs `arsenal calendar list --json`
- **THEN** output is a valid JSON array of event objects

#### Scenario: List trashed events

- **WHEN** the user runs `arsenal calendar list --trashed`
- **THEN** only soft-deleted events are printed

### Requirement: Show, edit, rm, restore, purge subcommands

The system MUST provide `show <id>` (full event detail), `edit <id>` (same flags as add), `rm <id>` (soft-delete), `restore <id>`, and `purge <id>` (requires `--yes` or TTY confirmation).

#### Scenario: Show existing event

- **WHEN** the user runs `arsenal calendar show 3`
- **THEN** full event detail is printed including all fields, tags, start_at, end_at, all_day, location

#### Scenario: Show non-existent event fails

- **WHEN** the user runs `arsenal calendar show 9999` and no event with ID 9999 exists
- **THEN** the CLI prints an error and exits non-zero

#### Scenario: Purge without --yes in non-interactive mode fails

- **WHEN** the user runs `arsenal calendar purge 3` with stdin not a TTY and no `--yes`
- **THEN** the CLI prints `"Error: --yes required in non-interactive mode"` and exits non-zero

#### Scenario: Rm soft-deletes event

- **WHEN** the user runs `arsenal calendar rm 3`
- **THEN** the event's `deleted_at` is set and the event no longer appears in default list

### Requirement: Export subcommand

The system MUST provide `arsenal calendar export` with flags: `--format` (ical, required), `--from`, `--to` (date range), `--output` (file path, optional — stdout if omitted). The `--format ical` value is the only supported format in v3.x.

#### Scenario: Export to stdout in ical format

- **WHEN** the user runs `arsenal calendar export --format ical`
- **THEN** iCal content is written to stdout beginning with `BEGIN:VCALENDAR`

#### Scenario: Export with unsupported format fails

- **WHEN** the user runs `arsenal calendar export --format csv`
- **THEN** the CLI prints an error about unsupported format and exits non-zero

#### Scenario: Export with --output writes to file

- **WHEN** the user runs `arsenal calendar export --format ical --output /tmp/events.ics`
- **THEN** the file `/tmp/events.ics` contains the iCal content
- **AND** nothing is written to stdout

### Requirement: Shell completions

The system MUST register completions for `arsenal calendar` subcommands and flag values (recurrence values, category slugs) in `internal/cli/completion.go`.

#### Scenario: Tab-complete recurrence values

- **WHEN** the user types `arsenal calendar add --recurrence <TAB>`
- **THEN** completions include `none`, `daily`, `weekly`, `monthly`, `yearly`

#### Scenario: Tab-complete subcommands

- **WHEN** the user types `arsenal calendar <TAB>`
- **THEN** completions include `add`, `list`, `show`, `edit`, `rm`, `restore`, `purge`, `export`

## Out of Scope

- Interactive TUI from CLI, piped bulk import, CSV export format.
