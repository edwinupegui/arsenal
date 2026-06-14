# Delta for today-view

## MODIFIED Requirements

### REQ-TV-03: Section ordering

The system MUST render sections in this fixed order: **Overdue → Due Today → Upcoming (next 7 days) → Recent Resources → This Month's Spending → Recent Transactions → Today's Events → Upcoming Events**. Empty sections SHALL be omitted from the rendered output (no header shown for zero-item sections). This order is not user-configurable in v3.x.
(Previously: Section order ended at Recent Transactions with no calendar sections.)

The `sectionOrder` map in `internal/today/sections.go` MUST include `"events-today": 7` and `"events-upcoming": 8`.

#### Scenario: Section ordering is fixed with calendar sections last

- **GIVEN** providers return sections with keys "upcoming", "overdue", "due-today", "recent", "this-month-spending", "recent-transactions", "events-today", "events-upcoming"
- **WHEN** the Registry produces the ordered page
- **THEN** sections appear in order: Overdue, Due Today, Upcoming, Recent Resources, This Month's Spending, Recent Transactions, Today's Events, Upcoming Events

#### Scenario: Calendar sections appear after finance sections

- **GIVEN** `FinanceProvider` and `CalendarProvider` both return non-empty sections
- **WHEN** the Today view renders
- **THEN** "Today's Events" appears after "Recent Transactions"
- **AND** "Upcoming Events" appears after "Today's Events"

#### Scenario: Calendar sections omitted when no calendar data

- **GIVEN** zero calendar events exist for today or the next 7 days
- **WHEN** the Today view renders
- **THEN** "Today's Events" and "Upcoming Events" headers do NOT appear
- **AND** the order remains: Overdue, Due Today, Upcoming, Recent Resources, This Month's Spending, Recent Transactions

#### Scenario: Finance sections omitted when no finance data

- **GIVEN** zero finance transactions exist but calendar events exist
- **WHEN** the Today view renders
- **THEN** "This Month's Spending" and "Recent Transactions" headers do NOT appear
- **AND** "Today's Events" and "Upcoming Events" appear in their correct positions

## ADDED Requirements

### REQ-TV-09: showAllURLFor includes calendar sections

The system MUST extend `showAllURLFor` in `internal/today/today.go` with two cases: `"events-today"` → `"/calendar?from={today}&to={today}"` and `"events-upcoming"` → `"/calendar?from={tomorrow}&to={today_plus_7}"`. When a calendar section exceeds 5 items, the density truncation (REQ-TV-04) MUST render a "show all →" link using these URLs.

#### Scenario: events-today show-all link points to filtered calendar list

- **GIVEN** the "events-today" section contains 8 events
- **WHEN** the Today view renders
- **THEN** only the first 5 events are displayed
- **AND** a "show all →" link is rendered pointing to `/calendar?from={today}&to={today}`

#### Scenario: events-upcoming show-all link points to filtered calendar list

- **GIVEN** the "events-upcoming" section contains 8 events
- **WHEN** the Today view renders
- **THEN** only the first 5 events are displayed
- **AND** a "show all →" link is rendered pointing to `/calendar?from={tomorrow}&to={today_plus_7}`

#### Scenario: No show-all link when section has 5 or fewer items

- **GIVEN** the "events-today" section contains 3 events
- **WHEN** the Today view renders
- **THEN** all 3 events are displayed and no "show all →" link appears
