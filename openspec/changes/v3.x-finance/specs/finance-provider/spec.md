# finance-provider

## Purpose

`FinanceProvider` implements `today.Provider` to contribute finance sections to the Today view. Provides "This Month's Spending" (summary with total + top 3 categories) and "Recent Transactions" (5 most recent). Registered alongside `TodosProvider` and `ResourcesProvider`.

## Requirements

### Requirement: Provider interface implementation

The system MUST provide `FinanceProvider` implementing `today.Provider` with `Name() = "finance"`. The provider SHALL be registered in `today.Service` alongside existing providers.

#### Scenario: Provider name is finance

- **WHEN** `FinanceProvider.Name()` is called
- **THEN** it returns `"finance"`

#### Scenario: Provider is registered in Today service

- **GIVEN** the Today service is initialized
- **WHEN** providers are listed
- **THEN** `FinanceProvider` appears alongside `TodosProvider` and `ResourcesProvider`

### Requirement: This Month's Spending section

The system MUST return a section with key `"this-month-spending"` and title "This Month's Spending". The section SHALL contain a summary item showing the total expenses for the current month and the top 3 expense categories by amount. The current month SHALL be computed using the user's configured timezone (`KeyUserTimezone`).

#### Scenario: Section shows monthly spending summary

- **GIVEN** 10 expenses totaling $500 exist in June 2026, top categories are food ($200), transport ($100), services ($50)
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** the "this-month-spending" section contains a summary item with total "$500" and top 3 categories

#### Scenario: Section omitted when no expenses in current month

- **GIVEN** zero expenses exist in the current month
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** the "this-month-spending" section is NOT included

### Requirement: Recent Transactions section

The system MUST return a section with key `"recent-transactions"` and title "Recent Transactions". The section SHALL contain the 5 most recent non-trashed transactions sorted by date DESC. Items MUST map to the common `Item` shape with `Domain="finance"`, `Title=account+kind`, `Subtitle=formatted amount+currency`, `Tags=transaction.Tags`, `URL="/finance/{id}"`.

#### Scenario: Section shows 5 most recent transactions

- **GIVEN** 8 non-trashed transactions exist
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** the "recent-transactions" section contains 5 items sorted by date DESC

#### Scenario: Section omitted when no transactions

- **GIVEN** zero non-trashed transactions exist
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** the "recent-transactions" section is NOT included

### Requirement: Graceful error degradation

When `FinanceProvider.Sections(ctx)` returns an error, the provider SHALL return the error to the Registry. The Registry SHALL skip finance sections and render a muted indicator. This follows the same pattern as existing providers (REQ-TV-06).

#### Scenario: Provider error skips finance sections

- **GIVEN** the database is locked when `FinanceProvider.Sections(ctx)` is called
- **WHEN** the Today view is requested
- **THEN** finance sections are omitted
- **AND** a muted "Finance unavailable" indicator is shown
- **AND** all other providers' sections render normally

### Requirement: Timezone-aware month boundaries

The provider MUST compute "this month" using `KeyUserTimezone`. When the timezone is unset or invalid, the provider SHALL fall back to UTC.

#### Scenario: Sections respect configured timezone

- **GIVEN** `KeyUserTimezone` is `"America/Argentina/Buenos_Aires"` (UTC−3)
- **AND** current UTC time is 2026-07-01 02:00 (2026-06-30 23:00 local)
- **WHEN** `FinanceProvider.Sections(ctx)` is called
- **THEN** "this month" refers to June 2026 (local time)

## Out of Scope

- Budget alerts, overdue bills, multi-currency aggregation in Today.
