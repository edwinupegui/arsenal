# Spec: today-tui

## Purpose

Wires the TUI `areaToday` placeholder to real Today view rendering, adds context-aware keybindings (`r` refresh, `n` new todo), updates the status bar, and changes the default TUI landing surface from `areaResources` to `areaToday` with `KeyLandingSurface` config override.

## Requirements

### REQ-TT-01: Wire areaToday to updateToday

The system MUST route `areaToday` in `App.Update()` to an `updateToday()` method that renders the Today view using the Registry. The placeholder text "Today (coming soon — phase 3)" MUST be replaced with the real Today view output: section headers, items, density truncation, and "show all →" indicators.

### REQ-TT-02: r key refreshes Today data

When the current area is `areaToday`, the system MUST bind the `r` key to a full Today view refresh — re-fetching sections from all providers via the Registry and re-rendering. The `r` keybinding MUST NOT conflict with the restore keybinding in the trash view (trash view takes precedence when active).

### REQ-TT-03: n key opens new-todo form

When the current area is `areaToday`, the system MUST bind the `n` key to open the new-todo creation form (same behavior as `n` in `areaTodos`). This provides a quick-add shortcut from the Today dashboard.

### REQ-TT-04: Status bar context-aware hints

The status bar MUST display context-aware keybinding hints when `areaToday` is active: `r` refresh, `n` new todo, `Tab`/`Shift+Tab` area switch, `1-5` direct jump. These hints replace the hints shown for other areas.

### REQ-TT-05: Default landing surface is areaToday

The TUI MUST open to `areaToday` by default on launch, replacing the previous `areaResources` default. This signals that Arsenal is a daily-driver command center, not just a bookmark manager.

### REQ-TT-06: KeyLandingSurface config override

The system MUST read `KeyLandingSurface` from `configstore` on launch. Valid values are `today` (default) and `resources`. If the value is `resources`, the TUI SHALL open to `areaResources` instead of `areaToday`. If the value is missing or invalid, the system SHALL fall back to `today`.

## Scenarios

### Scenario: Today area renders real data

- **GIVEN** the Registry has providers registered with data
- **WHEN** the user switches to the Today area
- **THEN** sections with headers and items are rendered (replacing the placeholder)

### Scenario: r key refreshes Today view

- **GIVEN** the user is in the Today area viewing stale data
- **WHEN** the user presses `r`
- **THEN** the Registry re-fetches all provider sections
- **AND** the view re-renders with fresh data

### Scenario: n key opens new-todo form from Today

- **GIVEN** the user is in the Today area
- **WHEN** the user presses `n`
- **THEN** the new-todo creation form opens

### Scenario: Default landing is Today

- **GIVEN** `KeyLandingSurface` is not set or is set to `today`
- **WHEN** the TUI starts
- **THEN** the rendered area is Today

### Scenario: KeyLandingSurface=resources overrides default

- **GIVEN** `KeyLandingSurface` is set to `resources`
- **WHEN** the TUI starts
- **THEN** the rendered area is Resources (legacy behavior preserved)

### Scenario: Invalid KeyLandingSurface falls back to today

- **GIVEN** `KeyLandingSurface` is set to an invalid value (e.g., `"finance"`)
- **WHEN** the TUI starts
- **THEN** the rendered area is Today (fallback to default)

### Scenario: Status bar shows Today hints

- **GIVEN** the user is in the Today area
- **WHEN** the status bar renders
- **THEN** it displays "Today" with hints: `r` refresh, `n` new todo, `Tab`/`1-5` switch

## Out of Scope

- Inline editing of todos from the Today view (navigate to Todos area for full CRUD).
- Mouse or scroll-wheel support in the Today view.
- Persistent scroll position across refreshes.
- Today view detail drill-down (items are display-only in v3.0).
