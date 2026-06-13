# Delta: timezone-v3.0.1 → today-providers

## MODIFIED Requirements

### REQ-TP-01: TodosProvider contributes three sections

The system MUST provide a `TodosProvider` that implements the `Provider` interface and returns up to three sections: "overdue" (todos with `due_date < today` and `status = open`), "due-today" (todos with `due_date = today` and `status = open`), and "upcoming" (todos with `due_date` between tomorrow and 7 days from now, `status = open`). Each section's title SHALL be human-readable: "Overdue", "Due Today", "Upcoming".

The `TodosProvider` MUST compute "today" using the user's configured timezone via `internal/today.UserLocation`. When `KeyUserTimezone` is set to a valid IANA timezone, all date boundaries (today, tomorrow, today+7d) SHALL be derived from `time.Now().In(loc)`. When `KeyUserTimezone` is unset or contains an invalid IANA value, the provider SHALL fall back to UTC silently, preserving v3.0 behavior.
(Previously: "today" was computed exclusively via `time.Now().UTC()` with no timezone configuration.)

#### Scenario: TodosProvider returns overdue section

- **GIVEN** 3 open todos with `due_date` before today exist
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** a section with key "overdue" and title "Overdue" is returned
- **AND** it contains 3 items sorted by `due_date ASC`

#### Scenario: TodosProvider returns due-today section

- **GIVEN** 2 open todos with `due_date = today` exist
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** a section with key "due-today" and title "Due Today" is returned
- **AND** it contains 2 items sorted by `priority DESC`

#### Scenario: TodosProvider returns upcoming section

- **GIVEN** 4 open todos with `due_date` between tomorrow and today+7d exist
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** a section with key "upcoming" and title "Upcoming" is returned
- **AND** it contains 4 items sorted by `due_date ASC`

#### Scenario: TodosProvider omits empty sections

- **GIVEN** no open todos have `due_date = today`
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** the "due-today" section is NOT included in the returned slice

#### Scenario: TodosProvider excludes done and deleted todos

- **GIVEN** 1 done todo and 1 soft-deleted todo are overdue
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** neither the done nor the deleted todo appears in the overdue section

#### Scenario: TodosProvider uses configured timezone for day boundaries

- **GIVEN** `KeyUserTimezone` is set to `America/Argentina/Buenos_Aires` (UTC−3)
- **AND** current UTC time is 2026-06-12 02:00 (2026-06-11 23:00 local)
- **AND** an open todo has `due_date = 2026-06-11`
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** the todo appears in the "due-today" section (local today is 2026-06-11)

#### Scenario: TodosProvider defaults to UTC when timezone unset

- **GIVEN** `KeyUserTimezone` is unset (default `"UTC"`)
- **AND** current UTC time is 2026-06-12 02:00
- **AND** an open todo has `due_date = 2026-06-11`
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** the todo appears in the "overdue" section (UTC today is 2026-06-12)
