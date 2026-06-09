# todo-web

## Purpose

Defines the web routes, templates, and sidebar integration for todos. The web surface provides HTMX-powered CRUD with card-based list views, inline status transitions, and sidebar counts. Templates reuse the existing `form.html` pattern with a `kind=todo` variant.

## Requirements

### Requirement: List route

The system MUST serve `GET /todos` rendering the todo list page with filtering (status, priority, overdue, category, tag, trashed) and pagination (50 per page, Q4 resolution). The page uses the template `internal/web/templates/todos.html`.

#### Scenario: List page renders open todos

- **WHEN** a user visits `GET /todos`
- **THEN** the page renders a card-based list of open todos, sorted per `todo-listing` default sort
- **AND** filter controls are visible (status dropdown, priority dropdown, overdue checkbox, category dropdown, tag input)

#### Scenario: List page with filter query params

- **WHEN** a user visits `GET /todos?status=done&priority=high`
- **THEN** only done, high-priority todos are rendered

### Requirement: Create routes

The system MUST serve `GET /todos/new` (render create form) and `POST /todos` (process creation). The form reuses `form.html` with `kind=todo`, providing fields for title, description, priority, due date, category, tags, notes, and recurrence.

#### Scenario: Create form renders

- **WHEN** a user visits `GET /todos/new`
- **THEN** an empty form is rendered with all todo fields

#### Scenario: Create form validates and creates

- **WHEN** a user submits `POST /todos` with valid data
- **THEN** the todo is created and the user is redirected to `GET /todos` with a success flash message

#### Scenario: Create form rejects invalid input

- **WHEN** a user submits `POST /todos` with an empty title
- **THEN** the form re-renders with an error message on the title field

### Requirement: Show and edit routes

The system MUST serve `GET /todos/{id}` (detail view) and `GET /todos/{id}/edit` (edit form) and `POST /todos/{id}` (process update).

#### Scenario: Show renders todo detail

- **WHEN** a user visits `GET /todos/42`
- **THEN** the full todo detail is rendered including title, description, priority, status, due date, category, tags, notes, recurrence, and timestamps

#### Scenario: Edit form updates todo

- **WHEN** a user submits `POST /todos/42` with updated fields
- **THEN** the todo is updated and the user is redirected to `GET /todos/42`

### Requirement: Status transition routes

The system MUST serve `POST /todos/{id}/done` and `POST /todos/{id}/open` for HTMX-powered status transitions. These endpoints return HTML fragments that replace the todo card in-place.

#### Scenario: Mark done via HTMX

- **WHEN** a user clicks the "Done" button on a todo card (triggers `POST /todos/42/done`)
- **THEN** the todo transitions to `done` per `todo-status` spec
- **AND** the response HTML fragment updates the card to show done styling and an "Open" button

#### Scenario: Mark open via HTMX

- **WHEN** a user clicks the "Open" button on a done todo card (triggers `POST /todos/42/open`)
- **THEN** the todo transitions to `open` and the card updates in-place

### Requirement: Delete, restore, and purge routes

The system MUST serve `POST /todos/{id}/delete` (soft-delete), `POST /todos/{id}/restore` (restore), and `POST /todos/{id}/purge` (hard-delete). Soft-delete and restore use HTMX to update cards in-place. Purge shows a confirmation dialog before executing.

#### Scenario: Soft-delete removes card via HTMX

- **WHEN** a user clicks "Delete" on a todo card (triggers `POST /todos/42/delete`)
- **THEN** the todo is soft-deleted and the card is removed from the list via HTMX

#### Scenario: Restore brings card back

- **WHEN** a user clicks "Restore" on a trashed todo (triggers `POST /todos/42/restore`)
- **THEN** the todo is restored and reappears in the default list

#### Scenario: Purge button only shown when trashed

- **WHEN** viewing a trashed todo
- **THEN** a "Purge permanently" button is visible
- **AND** clicking it shows a confirmation dialog before executing the hard-delete

### Requirement: Sidebar integration

The system MUST add a "Todos" link to the web sidebar (in `layout.html`) with counts: open count, done-today count, and overdue count. Counts are computed at the handler level, not in the template.

#### Scenario: Sidebar shows todo counts

- **WHEN** any page is rendered
- **THEN** the sidebar includes a "Todos" link with badge counts (e.g., "Todos (12 open, 3 overdue)")

#### Scenario: Sidebar counts update after action

- **WHEN** a user marks a todo done via HTMX
- **AND** the sidebar is included in the HTMX response fragment
- **THEN** the open count decrements and done-today count increments

### Requirement: View model

The system MUST define a `todoVM` struct mirroring the `resourceVM` pattern, mapping store rows to template-friendly structs with formatted dates, resolved tag names, and category slugs.

#### Scenario: View model resolves tags

- **WHEN** a todo with tags `["urgente", "casa"]` is rendered
- **THEN** the `todoVM.Tags` field contains `["casa", "urgente"]` (sorted, normalized)

<!-- AMEND: phase-3-today -->

### Requirement: Sidebar Today entry and ordering

The system MUST add a "Today" link to the web sidebar before all domain entries. The sidebar order SHALL be: Today, Resources, Todos, Categories, Tags, Trash. The "Today" link SHALL display an overdue count badge (from `CountOverdueTodos`); the badge is hidden when the count is zero. After HTMX actions that change overdue counts (e.g., mark-done), the sidebar badge SHALL update via `hx-swap-oob` without a full page reload.
(Previously: Sidebar listed only Resources, Todos, Categories, Tags, Trash with no Today entry.)

#### Scenario: Sidebar shows Today link with overdue badge

- **WHEN** any page is rendered and 3 overdue open todos exist
- **THEN** the sidebar includes a "Today" link with badge "3" appearing before "Resources"

#### Scenario: Sidebar badge updates via hx-swap-oob after mark-done

- **WHEN** a user marks an overdue todo done via HTMX from any page
- **AND** the sidebar is included in the HTMX response via `hx-swap-oob`
- **THEN** the Today overdue badge decrements (e.g., from "3" to "2")

#### Scenario: Sidebar Today badge hidden when zero

- **WHEN** any page is rendered and zero overdue open todos exist
- **THEN** the sidebar includes a "Today" link with no badge

## Out of Scope

- Real-time updates via WebSocket (HTMX polling or manual refresh only).
- Mobile-specific responsive layouts (base responsive CSS is sufficient).
- Todo export from the web UI.
- Drag-and-drop reordering of todos.
- Rich text editing for description/notes (plain text only).
