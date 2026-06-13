# Delta: timezone-v3.0.1 → todo-web

## MODIFIED Requirements

### Requirement: List route

The system MUST serve `GET /todos` rendering the todo list page with filtering (status, priority, overdue, category, tag, trashed) and pagination (50 per page, Q4 resolution). The page uses the template `internal/web/templates/todos.html`. The overdue filter MUST compute "today" using `internal/today.UserLocation`, consistent with the service layer.
(Previously: Overdue filter computed "today" via `time.Now().UTC()` with no timezone configuration.)

#### Scenario: List page renders open todos

- **GIVEN** open todos exist
- **WHEN** a user visits `GET /todos`
- **THEN** the page renders a card-based list of open todos, sorted per `todo-listing` default sort
- **AND** filter controls are visible (status dropdown, priority dropdown, overdue checkbox, category dropdown, tag input)

#### Scenario: List page with filter query params

- **WHEN** a user visits `GET /todos?status=done&priority=high`
- **THEN** only done, high-priority todos are rendered

#### Scenario: List overdue filter respects user timezone

- **GIVEN** `KeyUserTimezone` is set to `America/Argentina/Buenos_Aires` (UTC−3)
- **AND** current UTC time is 2026-06-12 02:00 (2026-06-11 23:00 local)
- **AND** an open todo has `due_date = 2026-06-11`
- **WHEN** a user visits `GET /todos?overdue=true`
- **THEN** the todo is NOT included (due-today in local timezone, not overdue)

### Requirement: Sidebar integration

The system MUST add a "Todos" link to the web sidebar (in `layout.html`) with counts: open count, done-today count, and overdue count. Counts are computed at the handler level, not in the template. The overdue count MUST be computed using `internal/today.UserLocation`.
(Previously: Overdue count computed via `time.Now().UTC()` with no timezone configuration.)

#### Scenario: Sidebar shows todo counts

- **WHEN** any page is rendered
- **THEN** the sidebar includes a "Todos" link with badge counts (e.g., "Todos (12 open, 3 overdue)")

#### Scenario: Sidebar counts update after action

- **WHEN** a user marks a todo done via HTMX
- **AND** the sidebar is included in the HTMX response fragment
- **THEN** the open count decrements and done-today count increments

#### Scenario: Sidebar overdue count respects user timezone

- **GIVEN** `KeyUserTimezone` is set to `America/Argentina/Buenos_Aires` (UTC−3)
- **AND** current UTC time is 2026-06-12 02:00 (2026-06-11 23:00 local)
- **AND** 2 open todos have `due_date = 2026-06-11`
- **WHEN** any page is rendered
- **THEN** the sidebar overdue count is 0 (due-today in local timezone, not overdue)

### Requirement: Sidebar Today entry and ordering

The system MUST add a "Today" link to the web sidebar before all domain entries. The sidebar order SHALL be: Today, Resources, Todos, Categories, Tags, Trash. The "Today" link SHALL display an overdue count badge (from `CountOverdueTodos`); the badge is hidden when the count is zero. After HTMX actions that change overdue counts (e.g., mark-done), the sidebar badge SHALL update via `hx-swap-oob` without a full page reload. The overdue count MUST be computed using `internal/today.UserLocation`.
(Previously: Sidebar listed only Resources, Todos, Categories, Tags, Trash with no Today entry. Overdue count used UTC.)

#### Scenario: Sidebar shows Today link with overdue badge

- **GIVEN** 3 overdue open todos exist
- **WHEN** any page is rendered
- **THEN** the sidebar includes a "Today" link with badge "3" appearing before "Resources"

#### Scenario: Sidebar badge updates via hx-swap-oob after mark-done

- **WHEN** a user marks an overdue todo done via HTMX from any page
- **AND** the sidebar is included in the HTMX response via `hx-swap-oob`
- **THEN** the Today overdue badge decrements (e.g., from "3" to "2")

#### Scenario: Sidebar Today badge hidden when zero

- **GIVEN** zero overdue open todos exist
- **WHEN** any page is rendered
- **THEN** the sidebar includes a "Today" link with no badge

#### Scenario: Sidebar Today badge respects user timezone

- **GIVEN** `KeyUserTimezone` is set to `America/Argentina/Buenos_Aires` (UTC−3)
- **AND** current UTC time is 2026-06-12 02:00 (2026-06-11 23:00 local)
- **AND** 2 open todos have `due_date = 2026-06-11`
- **WHEN** any page is rendered
- **THEN** the Today badge is hidden (due-today in local timezone, zero overdue)
