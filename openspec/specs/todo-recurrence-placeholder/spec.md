# todo-recurrence-placeholder

## Purpose

Defines the persistence and display of the `recurrence` field on todos. The field is stored as an enum (`none` | `daily` | `weekly` | `monthly`) with default `none`, and is visible in all three surfaces (CLI, TUI, web). **No auto-expansion logic is implemented in v3.0** — when a recurring todo is marked done, no new instance is created. This matches the decision recorded in Q1 (option A) and ADR-0002's "placeholder" framing.

## Requirements

### Requirement: Recurrence field persistence

The system MUST store the `recurrence` field as a `TEXT` column with a `CHECK` constraint limiting values to `'none'`, `'daily'`, `'weekly'`, `'monthly'`. The default value SHALL be `'none'`.

#### Scenario: Set recurrence on create

- **WHEN** a todo is created with `recurrence = 'weekly'`
- **THEN** the row is inserted with `recurrence = 'weekly'`

#### Scenario: Default recurrence is none

- **WHEN** a todo is created without specifying recurrence
- **THEN** the row has `recurrence = 'none'`

#### Scenario: Invalid recurrence rejected

- **WHEN** a todo is created with `recurrence = 'yearly'`
- **THEN** the database CHECK constraint rejects the insert with an error

### Requirement: Recurrence displayed in all surfaces

The system MUST display the recurrence value in list views (CLI, TUI, web), detail/show views, and create/edit forms. The display format SHALL be the raw enum value (e.g., "weekly") — no human-friendly expansion (e.g., "every week") in v3.0.

#### Scenario: Recurrence shown in CLI list

- **WHEN** the user runs `arsenal todo list` and a todo has `recurrence = 'daily'`
- **THEN** the output includes the recurrence value `"daily"` for that todo

#### Scenario: Recurrence shown in TUI list

- **WHEN** the TUI todo list renders a todo with `recurrence = 'monthly'`
- **THEN** the list item displays `"monthly"` as part of the todo metadata

#### Scenario: Recurrence shown in web list and detail

- **WHEN** a todo with `recurrence = 'weekly'` is rendered on the web list or detail page
- **THEN** the recurrence value `"weekly"` is visible

#### Scenario: Recurrence editable in forms

- **WHEN** a user edits a todo and changes recurrence from `none` to `daily`
- **THEN** the update persists and the new value is reflected in all views

### Requirement: No auto-expansion on completion

The system MUST NOT create a new todo instance when a recurring todo is marked done. Marking a recurring todo done SHALL behave identically to marking a non-recurring todo done — only `status` and `done_at` change.

#### Scenario: Marking recurring todo done does not create new instance

- **WHEN** a todo with `recurrence = 'weekly'` and `status = 'open'` is marked done
- **THEN** the todo's status becomes `'done'` and `done_at` is set
- **AND** the total count of todos in the database is unchanged (no new row inserted)
- **AND** no due-date math is performed

#### Scenario: Marking recurring todo done via CLI

- **WHEN** the user runs `arsenal todo done 5` on a weekly recurring todo
- **THEN** the CLI confirms the transition
- **AND** `arsenal todo list` does not show a new instance of that todo

## Out of Scope

- Auto-expansion: creating a new open todo with an advanced `due_date` when a recurring one is completed (deferred to v3.x).
- Due-date math: computing the next occurrence based on recurrence interval and current date.
- Exception handling: skipping specific occurrences (e.g., "not this week").
- RRULE-grade recurrence syntax (deferred to calendar domain in v3.x).
- Recurrence end dates or occurrence counts.
- Background jobs or cron-like scheduling for recurrence expansion.
