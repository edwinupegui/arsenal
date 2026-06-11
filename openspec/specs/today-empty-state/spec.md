# Spec: today-empty-state

## Purpose

Friendly empty-state rendering for the Today view when no items exist across all sections, and per-section empty handling with actionable shortcut hints. Ensures first-run and zero-data experiences guide users toward productive next actions.

## Requirements

### REQ-ES-01: Global empty state

When ALL sections across ALL providers return zero items, the system MUST render a friendly message instead of a blank view. The message SHALL be: "Nothing on your plate today." followed by actionable shortcut hints. This applies to both TUI and web surfaces.

### REQ-ES-02: Per-section empty handling

Individual sections with zero items MUST be omitted entirely from the rendered output — no section header, no placeholder text. The Today view only shows headers for sections that contain at least one item. This is distinct from the global empty state (REQ-ES-01), which triggers only when ALL sections are empty.

### REQ-ES-03: Shortcut hints in empty state

The global empty state message MUST include shortcut hints appropriate to the current surface. In TUI: `n` to add a todo, `2` to browse resources. In web: "Add a todo" link to `/todos/new` and "Browse resources" link to `/resources`. The hints SHALL be visually distinct from the message text (e.g., keyboard-style badges in TUI, anchor links in web).

## Scenarios

### Scenario: Global empty state in TUI

- **GIVEN** no overdue, due-today, or upcoming todos exist
- **AND** no resources exist
- **WHEN** the user views the Today area in the TUI
- **THEN** the message "Nothing on your plate today." is displayed
- **AND** shortcut hints `n` to add a todo and `2` to browse resources are shown

### Scenario: Global empty state in web

- **GIVEN** no overdue, due-today, or upcoming todos exist
- **AND** no resources exist
- **WHEN** the user visits `/today`
- **THEN** the message "Nothing on your plate today." is displayed
- **AND** links to "Add a todo" (`/todos/new`) and "Browse resources" (`/resources`) are rendered

### Scenario: Partial data skips empty sections

- **GIVEN** 2 upcoming todos exist but zero overdue and zero due-today todos
- **AND** 3 recent resources exist
- **WHEN** the Today view renders
- **THEN** only "Upcoming" and "Recent Resources" sections are shown
- **AND** no "Overdue" or "Due Today" headers appear
- **AND** the global empty state message is NOT shown

### Scenario: Empty state after marking last todo done

- **GIVEN** the Today view shows 1 due-today todo
- **WHEN** the user marks that todo as done
- **AND** refreshes the Today view
- **THEN** the global empty state message is displayed (assuming no other items exist)

## Out of Scope

- Onboarding wizard or quick-start guide (the empty state is sufficient for v3.0).
- Animated or illustrated empty states.
- Per-section empty messages (e.g., "No overdue items!" under the Overdue header).
