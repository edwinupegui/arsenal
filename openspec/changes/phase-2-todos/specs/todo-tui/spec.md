# todo-tui

## Purpose

Defines the TUI sub-area for todos and the area-switcher prototype that wires all five v3.0 areas (Today, Resources, Todos, Finance, Calendar) as placeholders. Only Resources and Todos are functional; the other three show `(coming soon)`. This validates the TUI architecture before phase 3 builds the Today view on top of it (ADR-0002 Change 6).

## Resolved Open Questions

These questions were left open in the proposal and are resolved here with concrete defaults. They are noted as **user-tunable** — the user may override during `sdd-design` or after seeing the prototype.

- **Q2 (default landing surface)**: Default is `areaResources` for now. This avoids breaking existing flows. The `areaToday` becomes the default in phase 3 when Today is functional.
- **Q3 (area-switching keybindings)**: `Tab` cycles forward, `Shift+Tab` cycles backward, `1`-`5` direct jump to Today/Resources/Todos/Finance/Calendar respectively. On-screen hints show the available keys in the status bar.

## Requirements

### Requirement: Area enum and state

The system MUST add a `currentArea` enum to `internal/tui/app.go` with values: `areaToday`, `areaResources`, `areaTodos`, `areaFinance`, `areaCalendar`, `areaTrash`. The default area on launch SHALL be `areaResources` (Q2 resolution). The `areaTrash` is a special area accessible from any area via a dedicated key, not part of the Tab cycle.

#### Scenario: Default area on launch

- **WHEN** the TUI starts
- **THEN** the rendered area is Resources (existing behavior preserved)

### Requirement: Area switching with Tab and Shift+Tab

The system MUST cycle through the five main areas (Today → Resources → Todos → Finance → Calendar → Today...) on `Tab` (forward) and `Shift+Tab` (backward). The status bar MUST display the current area name and available keybinding hints.

#### Scenario: Tab cycles forward

- **WHEN** the current area is Resources and the user presses `Tab`
- **THEN** the area switches to Todos
- **AND** the status bar updates to show "Todos"

#### Scenario: Shift+Tab cycles backward

- **WHEN** the current area is Todos and the user presses `Shift+Tab`
- **THEN** the area switches to Resources

#### Scenario: Tab wraps around

- **WHEN** the current area is Calendar and the user presses `Tab`
- **THEN** the area switches to Today (wraps to first)

### Requirement: Direct jump with number keys

The system MUST support `1`-`5` keys for direct area jump: `1`=Today, `2`=Resources, `3`=Todos, `4`=Finance, `5`=Calendar.

#### Scenario: Jump to Todos with key 3

- **WHEN** the user presses `3` from any area
- **THEN** the area switches to Todos immediately

### Requirement: Placeholder areas

The system MUST render `(coming soon)` for areas that are not yet functional: Today, Finance, and Calendar. The placeholder MUST include the area name and a message indicating the feature is planned for a future phase.

#### Scenario: Today area shows coming soon

- **WHEN** the user switches to the Today area
- **THEN** the view displays "Today (coming soon — phase 3)"

#### Scenario: Finance area shows coming soon

- **WHEN** the user switches to the Finance area
- **THEN** the view displays "Finance (coming soon — v3.x)"

### Requirement: Todo area sub-model

The system MUST provide a `internal/tui/todos.go` sub-model that renders the todo list with the same filtering capabilities as `todo-listing`. The list is scrollable (no hard page limit). Keybindings within the todo area: `x` marks done/open, `d` soft-deletes, `r` restores (in trash view), `enter` opens detail/edit, `n` opens new-todo form.

#### Scenario: Todo area shows list

- **WHEN** the user switches to the Todos area
- **THEN** a scrollable list of open todos is displayed, sorted per `todo-listing` default sort

#### Scenario: Mark done with x

- **WHEN** the user presses `x` on a selected open todo
- **THEN** the todo transitions to `done` per `todo-status` spec
- **AND** the list refreshes to reflect the change

#### Scenario: Soft-delete with d

- **WHEN** the user presses `d` on a selected todo
- **THEN** the todo is soft-deleted per `todo-lifecycle` spec
- **AND** the list refreshes

#### Scenario: Restore with r in trash view

- **WHEN** the user is in the trash view and presses `r` on a selected trashed todo
- **THEN** the todo is restored per `todo-lifecycle` spec

#### Scenario: Edit with enter

- **WHEN** the user presses `enter` on a selected todo
- **THEN** the detail view opens showing full todo information

### Requirement: Status bar shows current area

The system MUST display the current area name in the status bar at all times, along with keybinding hints for area switching (`Tab`/`Shift+Tab`/`1-5`).

#### Scenario: Status bar reflects area switch

- **WHEN** the user switches from Resources to Todos
- **THEN** the status bar changes from "Resources" to "Todos" with updated keybinding hints

## Out of Scope

- Today view aggregation (phase 3; this phase only wires the placeholder).
- Finance and Calendar functional areas (v3.x).
- Trash as a Tab-cycle area (it's a special toggle, like in the current resources TUI).
- Mouse support for area switching.
- Persistent area preference across TUI sessions (could use `configstore` in v3.x).
