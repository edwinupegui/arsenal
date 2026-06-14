# finance-tui

## Purpose

TUI sub-area for Finance, replacing the `areaFinance` placeholder with a functional sub-model. Provides transaction list, detail view, CRUD keybindings, and status bar integration following the `areaTodos` pattern.

## Requirements

### Requirement: Finance sub-model replaces placeholder

The system MUST replace the `areaFinance` placeholder in `internal/tui/app.go` with a `financeModel` sub-model. The `Update()` method SHALL route `areaFinance` to `updateFinance()`. The `View()` method SHALL render the finance list or detail view.

#### Scenario: Finance area renders transaction list

- **WHEN** the user switches to the Finance area
- **THEN** a scrollable list of transactions is displayed, sorted by date DESC (most recent first)

#### Scenario: Placeholder message no longer appears

- **WHEN** the user switches to the Finance area
- **THEN** the "Finance (coming soon — v3.x)" message is NOT displayed

### Requirement: Keybindings

The system MUST support these keybindings within the Finance area: `n` (new transaction), `e` (edit selected), `d` (soft-delete selected), `r` (restore selected, in trashed view), `x` (purge selected), `j`/`k` (navigate down/up), `enter` (detail view), `Tab` (area switch).

#### Scenario: Navigate with j/k

- **WHEN** the user presses `j` in the finance list
- **THEN** the selection moves to the next transaction

#### Scenario: Create with n

- **WHEN** the user presses `n` in the Finance area
- **THEN** a new-transaction form is displayed

#### Scenario: Soft-delete with d

- **WHEN** the user presses `d` on a selected transaction
- **THEN** the transaction is soft-deleted and the list refreshes

#### Scenario: Restore with r in trashed view

- **WHEN** the user is viewing trashed transactions and presses `r`
- **THEN** the selected transaction is restored and the list refreshes

#### Scenario: Purge with x

- **WHEN** the user presses `x` on a selected trashed transaction
- **THEN** the transaction is hard-deleted after confirmation

### Requirement: Status bar context hints

The system MUST display "Finance" as the area name in the status bar with keybinding hints: `n` new, `e` edit, `d` delete, `Tab` switch.

#### Scenario: Status bar shows finance hints

- **WHEN** the user is in the Finance area
- **THEN** the status bar shows "Finance" and relevant key hints

### Requirement: Detail view on Enter

The system MUST render a detail view showing all transaction fields (date, amount, kind, account, category, tags, notes, recurrence, currency, timestamps) when `enter` is pressed on a selected transaction.

#### Scenario: Enter opens detail

- **WHEN** the user presses `enter` on a selected expense transaction
- **THEN** the detail view displays all fields including tags and currency

## Out of Scope

- Mouse support, persistent area preference, inline editing in list view.
