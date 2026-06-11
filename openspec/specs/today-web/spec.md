# Spec: today-web

## Purpose

Web route, sidebar integration, overdue count badge, "show all →" navigation links, and HTMX partial refresh for the Today view. Isolates Today data fetching from `commonPage()` so non-Today pages remain lightweight.

## Requirements

### REQ-TW-01: /today route

The system MUST serve `GET /today` rendering the Today view page using template `internal/web/templates/today.html`. The page displays all non-empty sections from the Registry with items, density truncation, and "show all →" links. The route MUST be registered in the web router alongside existing domain routes.

### REQ-TW-02: Sidebar Today entry with overdue badge

The system MUST add a "Today" link to the web sidebar in `layout.html`. The link SHALL display an overdue count badge (number of overdue open todos), computed via `CountOverdueTodos`. The badge MUST be visible on all pages, not just `/today`. When the overdue count is zero, the badge SHALL be hidden (no "0" badge).

### REQ-TW-03: show-all links use existing routes

Each section's "show all →" link MUST point to an existing domain route with query parameters: Overdue → `/todos?status=open&overdue=true`, Due Today → `/todos?status=open&due=today`, Upcoming → `/todos?status=open&due=upcoming`, Recent Resources → `/resources`. No new routes are created for "show all" navigation.

### REQ-TW-04: hx-swap-oob section refresh

The system MUST support HTMX out-of-band section refresh. Each section in the Today template SHALL have a unique `id` attribute. A refresh action (e.g., marking a todo done from the Today view) SHALL return an `hx-swap-oob` response that updates the affected section and the sidebar badge without a full page reload.

### REQ-TW-05: Sidebar ordering

The sidebar MUST list navigation entries in this order: **Today, Resources, Todos**, Categories, Tags, Trash. "Today" appears first, before all domain entries.

### REQ-TW-06: commonPage data isolation

The Today view's data fetching (full section aggregation) MUST be performed by the `/today` handler, not by `commonPage()`. The `commonPage()` function SHALL only compute the lightweight overdue count for the sidebar badge (reusing existing `CountOverdueTodos`). Non-Today pages MUST NOT incur the cost of full Today aggregation.

## Scenarios

### Scenario: Today page renders all sections

- **GIVEN** overdue, due-today, upcoming todos and recent resources exist
- **WHEN** a user visits `GET /today`
- **THEN** the page renders four sections with items, each truncated to 5 items max

### Scenario: Sidebar shows Today link with overdue badge

- **GIVEN** 3 overdue open todos exist
- **WHEN** any page is rendered
- **THEN** the sidebar includes a "Today" link with badge showing "3"

### Scenario: Sidebar hides badge when zero overdue

- **GIVEN** zero overdue open todos exist
- **WHEN** any page is rendered
- **THEN** the sidebar includes a "Today" link with no badge

### Scenario: show-all link navigates to filtered todo list

- **GIVEN** the Overdue section shows 5 of 8 items with a "show all →" link
- **WHEN** the user clicks "show all →" on the Overdue section
- **THEN** the browser navigates to `/todos?status=open&overdue=true`

### Scenario: show-all for Recent Resources navigates to resources

- **GIVEN** the Recent Resources section shows a "show all →" link
- **WHEN** the user clicks "show all →" on Recent Resources
- **THEN** the browser navigates to `/resources`

### Scenario: Mark-done from Today refreshes section and badge

- **GIVEN** the user is on `/today` with 2 overdue items and sidebar badge "2"
- **WHEN** the user marks one overdue todo as done via HTMX
- **THEN** the overdue section updates via `hx-swap-oob` to show 1 item
- **AND** the sidebar badge updates to "1" via `hx-swap-oob`

### Scenario: Non-Today pages do not fetch full aggregation

- **GIVEN** a user visits `GET /resources`
- **WHEN** the page renders
- **THEN** `commonPage()` computes only the overdue count for the sidebar badge
- **AND** full Today section aggregation is NOT performed

## Out of Scope

- Mobile-specific responsive layout for the Today page.
- Real-time updates via WebSocket or SSE.
- Today page search or inline filtering.
- Drag-and-drop reordering of Today items.
- Exporting Today view data from the web UI.
