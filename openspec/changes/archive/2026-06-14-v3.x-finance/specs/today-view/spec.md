# Delta for today-view

## MODIFIED Requirements

### REQ-TV-03: Section ordering

The system MUST render sections in this fixed order: **Overdue → Due Today → Upcoming (next 7 days) → Recent Resources → This Month's Spending → Recent Transactions**. Empty sections SHALL be omitted from the rendered output (no header shown for zero-item sections). This order is not user-configurable in v3.0.
(Previously: Section order was Overdue → Due Today → Upcoming → Recent Resources with no finance sections.)

#### Scenario: Section ordering is fixed

- **GIVEN** providers return sections with keys "upcoming", "overdue", "due-today", "recent", "this-month-spending", "recent-transactions"
- **WHEN** the Registry produces the ordered page
- **THEN** sections appear in order: Overdue, Due Today, Upcoming, Recent Resources, This Month's Spending, Recent Transactions

#### Scenario: Finance sections appear after todo and resource sections

- **GIVEN** `TodosProvider`, `ResourcesProvider`, and `FinanceProvider` all return non-empty sections
- **WHEN** the Today view renders
- **THEN** "This Month's Spending" appears after "Recent Resources"
- **AND** "Recent Transactions" appears after "This Month's Spending"

#### Scenario: Finance sections omitted when no data

- **GIVEN** zero finance transactions exist
- **WHEN** the Today view renders
- **THEN** "This Month's Spending" and "Recent Transactions" headers do NOT appear
- **AND** the order remains: Overdue, Due Today, Upcoming, Recent Resources
