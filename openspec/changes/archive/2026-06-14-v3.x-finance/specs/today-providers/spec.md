# Delta for today-providers

## ADDED Requirements

### REQ-TP-07: FinanceProvider contributes two sections

The system MUST provide a `FinanceProvider` that implements the `Provider` interface and returns up to two sections: "this-month-spending" (title "This Month's Spending" — total expenses + top 3 categories for the current month) and "recent-transactions" (title "Recent Transactions" — 5 most recent non-trashed transactions). The `FinanceProvider` MUST compute month boundaries using the user's configured timezone via `internal/today.UserLocation`, consistent with REQ-TP-01.

The `FinanceProvider` MUST be registered in `today.Service` alongside `TodosProvider` and `ResourcesProvider`.

#### Scenario: FinanceProvider returns this-month-spending section

- **GIVEN** 5 expenses exist in the current month totaling $300
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** a section with key "this-month-spending" and title "This Month's Spending" is returned
- **AND** it contains a summary item showing total "$300" and top 3 categories

#### Scenario: FinanceProvider returns recent-transactions section

- **GIVEN** 8 non-trashed transactions exist
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** a section with key "recent-transactions" and title "Recent Transactions" is returned
- **AND** it contains 5 items sorted by date DESC

#### Scenario: FinanceProvider omits empty sections

- **GIVEN** zero transactions exist
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** the returned slice is empty (no finance sections)

#### Scenario: FinanceProvider error skips sections gracefully

- **GIVEN** the database is locked when `FinanceProvider.Sections(ctx)` is called
- **WHEN** the Today view is requested
- **THEN** finance sections are omitted
- **AND** a muted "Finance unavailable" indicator is shown
- **AND** all other providers' sections render normally

#### Scenario: FinanceProvider respects user timezone

- **GIVEN** `KeyUserTimezone` is `"America/Argentina/Buenos_Aires"` (UTC−3)
- **AND** current UTC time is 2026-07-01 02:00 (2026-06-30 23:00 local)
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** "this month" refers to June 2026 (local time, not UTC July)

#### Scenario: Item mapping for finance transaction

- **GIVEN** a transaction with ID 10, account "checking", kind "expense", amount 42.50, currency "USD", tags ["food"]
- **WHEN** the transaction is mapped to an `Item`
- **THEN** `Domain="finance"`, `Title="checking (expense)"`, `Subtitle="$42.50"`, `Tags=["food"]`, `URL="/finance/10"`
