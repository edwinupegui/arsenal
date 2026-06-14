# finance-web

## Purpose

Web routes, templates, and sidebar integration for Finance. Provides HTMX-powered CRUD with card-based list views, inline lifecycle actions, sidebar count badge, and an empty state. Follows the `todo-web` pattern.

## Requirements

### Requirement: List and create routes

The system MUST serve `GET /finance` (list with filters: date range, kind, category, tag, trashed) and `GET /finance/new` + `POST /finance` (create form and processing).

#### Scenario: List page renders transactions

- **WHEN** a user visits `GET /finance`
- **THEN** a card-based list of transactions is rendered sorted by date DESC
- **AND** filter controls are visible

#### Scenario: Create form validates and creates

- **WHEN** a user submits `POST /finance` with valid data
- **THEN** the transaction is created and the user is redirected to `GET /finance`

### Requirement: Detail and edit routes

The system MUST serve `GET /finance/:id` (detail), `GET /finance/:id/edit` (edit form), `POST /finance/:id` (update processing).

#### Scenario: Show renders transaction detail

- **WHEN** a user visits `GET /finance/5`
- **THEN** full transaction detail is rendered including all fields and tags

#### Scenario: Edit form updates transaction

- **WHEN** a user submits `POST /finance/5` with updated amount
- **THEN** the transaction is updated and the user is redirected to `GET /finance/5`

### Requirement: Lifecycle routes with HTMX

The system MUST serve `POST /finance/:id/delete` (soft-delete), `POST /finance/:id/restore` (restore), `POST /finance/:id/purge` (hard-delete with confirmation). These endpoints SHALL return HTMX fragments for in-place card updates.

#### Scenario: Soft-delete removes card via HTMX

- **WHEN** a user clicks "Delete" on a transaction card
- **THEN** the transaction is soft-deleted and the card is removed from the list via HTMX

#### Scenario: Restore brings card back

- **WHEN** a user clicks "Restore" on a trashed transaction
- **THEN** the transaction is restored and reappears in the default list

#### Scenario: Purge shows confirmation

- **WHEN** viewing a trashed transaction
- **THEN** a "Purge permanently" button is visible with a confirmation dialog

### Requirement: Sidebar entry with count badge

The system MUST add a "Finance" link to the web sidebar in `layout.html` with a transaction count badge. The badge SHALL reflect the count of non-trashed transactions. The badge is hidden when count is zero.

#### Scenario: Sidebar shows finance count badge

- **WHEN** any page is rendered and 12 non-trashed transactions exist
- **THEN** the sidebar includes "Finance" with badge "12"

#### Scenario: Sidebar badge hidden when zero

- **WHEN** any page is rendered and zero transactions exist
- **THEN** the sidebar includes "Finance" with no badge

### Requirement: Empty state

The system MUST render an empty state message when no transactions exist, with a link to the create form.

#### Scenario: Empty state shown on first visit

- **WHEN** a user visits `GET /finance` and no transactions exist
- **THEN** an empty state message is rendered with a "Add transaction" link

## Out of Scope

- Real-time WebSocket updates, drag-and-drop, rich text notes.
