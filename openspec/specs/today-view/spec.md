# Spec: today-view

## Purpose

Core aggregation layer for the "Today" cross-domain view. Defines the `Provider` interface, `Registry`, `Section`/`Item` types, section ordering, density control, refresh contract, graceful degradation, and the `arsenal today` CLI command. This is the architectural spine that turns Arsenal from a domain-specific tool into a daily-driver command center (ADR-0002 Change 4).

## Requirements

### REQ-TV-01: Provider interface

The system MUST define a `Provider` interface in `internal/today/` with two methods: `Name() string` returning a unique provider identifier, and `Sections(ctx context.Context) ([]Section, error)` returning the provider's contributed sections. Any domain that wants to appear in the Today view MUST implement this interface and register with the Registry.

### REQ-TV-02: Registry aggregation

The system MUST provide a `Registry` that holds an ordered list of registered `Provider` implementations. When the Today view is requested, the Registry SHALL call `Sections(ctx)` on each provider, collect results, and produce a single ordered page. The Registry MUST NOT hardcode provider names or section keys — ordering is determined by the fixed section-order table (REQ-TV-03).

### REQ-TV-03: Section ordering

The system MUST render sections in this fixed order: **Overdue → Due Today → Upcoming (next 7 days) → Recent Resources → This Month's Spending → Recent Transactions**. Empty sections SHALL be omitted from the rendered output (no header shown for zero-item sections). This order is not user-configurable in v3.0.

### REQ-TV-04: Section density

Each section MUST display at most **5 items**. When a section contains more than 5 items, the system SHALL truncate the display and render a "show all →" link pointing to the full domain list filtered to that section's criteria (existing route with query params). When a section contains 5 or fewer items, no "show all →" link is shown.

### REQ-TV-05: Refresh contract

The Today view MUST refresh its data only on explicit user action: initial page load, `r` key press in TUI, or area navigation into Today. The system SHALL NOT use background polling, timers, goroutines, or daemon processes for refresh (ADR-0002 "no daemon" constraint).

### REQ-TV-06: Provider error degradation

When a provider's `Sections(ctx)` returns an error, the Registry MUST skip that provider's sections and render a muted error indicator in their place (e.g., "Todos unavailable"). The remaining providers' sections SHALL still render normally. A single provider failure MUST NOT cause the entire Today view to fail.

### REQ-TV-07: Cross-domain Item shape

The system MUST define a common `Item` struct with fields sufficient for both TUI list rendering and web card rendering: `Domain` (string), `ID` (int64), `Title` (string), `Subtitle` (string, optional — due date, category, etc.), `Priority` (string, optional), `Tags` ([]string), and `URL` (string — web link or empty for TUI). Providers MUST map their domain-specific rows into this common shape.

### REQ-TV-08: CLI command

The system MUST provide a top-level `arsenal today` CLI command that renders the Today view as formatted text to stdout. The command SHALL support a `--json` flag that outputs the same data as a JSON array of sections with their items. The CLI command reuses the same Registry and providers as TUI and web.

## Scenarios

### Scenario: Registry collects from two providers

- **GIVEN** a Registry with `TodosProvider` and `ResourcesProvider` registered
- **WHEN** the Today view is requested
- **THEN** the Registry calls `Sections(ctx)` on both providers
- **AND** the result contains sections from both providers in the fixed order (Overdue, Due Today, Upcoming, Recent Resources)

### Scenario: Section ordering is fixed

- **GIVEN** providers return sections with keys "upcoming", "overdue", "due-today", "recent", "this-month-spending", "recent-transactions"
- **WHEN** the Registry produces the ordered page
- **THEN** sections appear in order: Overdue, Due Today, Upcoming, Recent Resources, This Month's Spending, Recent Transactions

### Scenario: Density truncates at 5 items

- **GIVEN** the Overdue section contains 8 items
- **WHEN** the Today view renders
- **THEN** only the first 5 overdue items are displayed
- **AND** a "show all →" link is rendered after the 5th item

### Scenario: No truncation when at or below density

- **GIVEN** the Due Today section contains 3 items
- **WHEN** the Today view renders
- **THEN** all 3 items are displayed
- **AND** no "show all →" link is shown

### Scenario: Manual refresh on r key

- **GIVEN** the user is viewing the Today view in the TUI
- **WHEN** the user presses `r`
- **THEN** the Registry re-fetches sections from all providers
- **AND** the view updates with fresh data

### Scenario: No background refresh

- **GIVEN** the user is viewing the Today view
- **WHEN** 10 minutes pass with no user action
- **THEN** the displayed data remains unchanged (no automatic refresh)

### Scenario: Provider error degrades gracefully

- **GIVEN** `TodosProvider.Sections(ctx)` returns an error (e.g., DB locked)
- **WHEN** the Today view is requested
- **THEN** the todos sections (Overdue, Due Today, Upcoming) are omitted
- **AND** a muted "Todos unavailable" indicator is shown
- **AND** the ResourcesProvider's Recent Resources section still renders normally

### Scenario: Empty sections are omitted

- **GIVEN** there are zero overdue todos, zero due-today todos, 2 upcoming todos, and 3 recent resources
- **WHEN** the Today view renders
- **THEN** only "Upcoming" and "Recent Resources" sections are shown
- **AND** no "Overdue" or "Due Today" headers appear

### Scenario: Finance sections appear after todo and resource sections

- **GIVEN** `TodosProvider`, `ResourcesProvider`, and `FinanceProvider` all return non-empty sections
- **WHEN** the Today view renders
- **THEN** "This Month's Spending" appears after "Recent Resources"
- **AND** "Recent Transactions" appears after "This Month's Spending"

### Scenario: Finance sections omitted when no data

- **GIVEN** zero finance transactions exist
- **WHEN** the Today view renders
- **THEN** "This Month's Spending" and "Recent Transactions" headers do NOT appear
- **AND** the order remains: Overdue, Due Today, Upcoming, Recent Resources

### Scenario: CLI command outputs text

- **GIVEN** the Today view has 2 overdue and 1 due-today item
- **WHEN** the user runs `arsenal today`
- **THEN** formatted text is printed to stdout showing sections and items

### Scenario: CLI command with --json flag

- **GIVEN** the Today view has data
- **WHEN** the user runs `arsenal today --json`
- **THEN** a JSON array of section objects is printed to stdout
- **AND** each section contains its items with domain, id, title, subtitle, priority, tags, and URL fields

## Out of Scope

- Calendar providers (v3.x).
- Pinned items within sections.
- Custom user-defined sections.
- User-configurable section ordering.
- Background polling or daemon-based refresh.
- Timezone handling beyond system-local `date('now')` (deferred to separate ADR).
- Cross-domain search UI (deferred to phase 4).
