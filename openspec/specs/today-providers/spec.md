# Spec: today-providers

## Purpose

Concrete `Provider` implementations for v3.0: `TodosProvider` (contributing overdue, due-today, and upcoming-7d sections) and `ResourcesProvider` (contributing a recent-resources section). These providers query existing store methods and map domain rows into the common `Item` shape defined by `today-view`.

## Requirements

### REQ-TP-01: TodosProvider contributes three sections

The system MUST provide a `TodosProvider` that implements the `Provider` interface and returns up to three sections: "overdue" (todos with `due_date < today` and `status = open`), "due-today" (todos with `due_date = today` and `status = open`), and "upcoming" (todos with `due_date` between tomorrow and 7 days from now, `status = open`). Each section's title SHALL be human-readable: "Overdue", "Due Today", "Upcoming".

The `TodosProvider` MUST compute "today" using the user's configured timezone via `internal/today.UserLocation`. When `KeyUserTimezone` is set to a valid IANA timezone, all date boundaries (today, tomorrow, today+7d) SHALL be derived from `time.Now().In(loc)`. When `KeyUserTimezone` is unset or contains an invalid IANA value, the provider SHALL fall back to UTC silently, preserving v3.0 behavior.

### REQ-TP-02: TodosProvider overdue query

The overdue section MUST use `ListTodosDueBefore(today)` or equivalent, filtering to `status = open` and `deleted_at IS NULL`. Items SHALL be sorted by `due_date ASC` (most overdue first), then by `priority DESC`.

### REQ-TP-03: TodosProvider due-today query

The due-today section MUST use `ListTodosDueBetween(today, today)` or equivalent, filtering to `status = open` and `deleted_at IS NULL`. Items SHALL be sorted by `priority DESC`, then by `created_at ASC`.

### REQ-TP-04: TodosProvider upcoming query

The upcoming section MUST use `ListTodosDueBetween(tomorrow, today + 7 days)` or equivalent, filtering to `status = open` and `deleted_at IS NULL`. Items SHALL be sorted by `due_date ASC`, then by `priority DESC`.

### REQ-TP-05: ResourcesProvider contributes one section

The system MUST provide a `ResourcesProvider` that implements the `Provider` interface and returns one section: "recent" with title "Recent Resources". The section MUST contain the 5 most recently created non-trashed resources, queried via `ListResourcesFiltered({Limit: 5})`. Items SHALL be sorted by `created_at DESC`.

### REQ-TP-06: Provider item mapping

Both providers MUST map their domain rows into the common `Item` struct (REQ-TV-07). `TodosProvider` maps: `Domain="todos"`, `Title=todo.Title`, `Subtitle=formatted due_date`, `Priority=todo.Priority`, `Tags=todo.Tags`, `URL="/todos/{id}"`. `ResourcesProvider` maps: `Domain="resources"`, `Title=resource.Title`, `Subtitle=resource.Type`, `Priority=""`, `Tags=resource.Tags`, `URL="/resources/{id}"`.

### REQ-TP-07: FinanceProvider contributes two sections

The system MUST provide a `FinanceProvider` that implements the `Provider` interface and returns up to two sections: "this-month-spending" (title "This Month's Spending" — total expenses + top 3 categories for the current month) and "recent-transactions" (title "Recent Transactions" — 5 most recent non-trashed transactions). The `FinanceProvider` MUST compute month boundaries using the user's configured timezone via `internal/today.UserLocation`, consistent with REQ-TP-01.

The `FinanceProvider` MUST be registered in `today.Service` alongside `TodosProvider` and `ResourcesProvider`.

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

## Scenarios

### Scenario: TodosProvider returns overdue section

- **GIVEN** 3 open todos with `due_date` before today exist
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** a section with key "overdue" and title "Overdue" is returned
- **AND** it contains 3 items sorted by `due_date ASC`

### Scenario: TodosProvider returns due-today section

- **GIVEN** 2 open todos with `due_date = today` exist
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** a section with key "due-today" and title "Due Today" is returned
- **AND** it contains 2 items sorted by `priority DESC`

### Scenario: TodosProvider returns upcoming section

- **GIVEN** 4 open todos with `due_date` between tomorrow and today+7d exist
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** a section with key "upcoming" and title "Upcoming" is returned
- **AND** it contains 4 items sorted by `due_date ASC`

### Scenario: TodosProvider omits empty sections

- **GIVEN** no open todos have `due_date = today`
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** the "due-today" section is NOT included in the returned slice

### Scenario: TodosProvider excludes done and deleted todos

- **GIVEN** 1 done todo and 1 soft-deleted todo are overdue
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** neither the done nor the deleted todo appears in the overdue section

### Scenario: ResourcesProvider returns recent section

- **GIVEN** 8 non-trashed resources exist
- **WHEN** `ResourcesProvider.Sections(ctx)` is called
- **THEN** a section with key "recent" and title "Recent Resources" is returned
- **AND** it contains the 5 most recently created resources

### Scenario: ResourcesProvider omits section when no resources

- **GIVEN** zero non-trashed resources exist
- **WHEN** `ResourcesProvider.Sections(ctx)` is called
- **THEN** the returned slice is empty (no "recent" section)

### Scenario: Item mapping for todo includes URL

- **GIVEN** a todo with ID 42, title "pay rent", due 2026-06-09, priority high, tags ["urgent"]
- **WHEN** the todo is mapped to an `Item`
- **THEN** `Domain="todos"`, `Title="pay rent"`, `Subtitle="2026-06-09"`, `Priority="high"`, `Tags=["urgent"]`, `URL="/todos/42"`

### Scenario: TodosProvider uses configured timezone for day boundaries

- **GIVEN** `KeyUserTimezone` is set to `America/Argentina/Buenos_Aires` (UTC−3)
- **AND** current UTC time is 2026-06-12 02:00 (2026-06-11 23:00 local)
- **AND** an open todo has `due_date = 2026-06-11`
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** the todo appears in the "due-today" section (local today is 2026-06-11)

### Scenario: TodosProvider defaults to UTC when timezone unset

- **GIVEN** `KeyUserTimezone` is unset (default `"UTC"`)
- **AND** current UTC time is 2026-06-12 02:00
- **AND** an open todo has `due_date = 2026-06-11`
- **WHEN** `TodosProvider.Sections(ctx)` is called
- **THEN** the todo appears in the "overdue" section (UTC today is 2026-06-12)

### Scenario: FinanceProvider returns this-month-spending section

- **GIVEN** 5 expenses exist in the current month totaling $300
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** a section with key "this-month-spending" and title "This Month's Spending" is returned
- **AND** it contains a summary item showing total "$300" and top 3 categories

### Scenario: FinanceProvider returns recent-transactions section

- **GIVEN** 8 non-trashed transactions exist
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** a section with key "recent-transactions" and title "Recent Transactions" is returned
- **AND** it contains 5 items sorted by date DESC

### Scenario: FinanceProvider omits empty sections

- **GIVEN** zero transactions exist
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** the returned slice is empty (no finance sections)

### Scenario: FinanceProvider error skips sections gracefully

- **GIVEN** the database is locked when `FinanceProvider.Sections(ctx)` is called
- **WHEN** the Today view is requested
- **THEN** finance sections are omitted
- **AND** a muted "Finance unavailable" indicator is shown
- **AND** all other providers' sections render normally

### Scenario: FinanceProvider respects user timezone

- **GIVEN** `KeyUserTimezone` is `"America/Argentina/Buenos_Aires"` (UTC−3)
- **AND** current UTC time is 2026-07-01 02:00 (2026-06-30 23:00 local)
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** "this month" refers to June 2026 (local time, not UTC July)

### Scenario: Item mapping for finance transaction

- **GIVEN** a transaction with ID 10, account "checking", kind "expense", amount 42.50, currency "USD", tags ["food"]
- **WHEN** the transaction is mapped to an `Item`
- **THEN** `Domain="finance"`, `Title="checking (expense)"`, `Subtitle="$42.50"`, `Tags=["food"]`, `URL="/finance/10"`

### Scenario: Item mapping for finance transaction

- **GIVEN** a transaction with ID 10, account "checking", kind "expense", amount 42.50, currency "USD", tags ["food"]
- **WHEN** the transaction is mapped to an `Item`
- **THEN** `Domain="finance"`, `Title="checking (expense)"`, `Subtitle="$42.50"`, `Tags=["food"]`, `URL="/finance/10"`

## Out of Scope

- Recurring todo auto-expansion (v3.x).
- Provider-level caching or memoization.
- Custom sort orders within sections.
