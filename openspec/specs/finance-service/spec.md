# finance-service

## Purpose

Core domain service for Finance. Defines the `Transaction` type, lifecycle operations (Create, Get, Update, SoftDelete, Restore, Purge, List), tag attachment via `domain.WithTags`, and the `Attacher` implementation. All three surfaces (CLI, TUI, web) route through this service.

## Requirements

### Requirement: Transaction domain type

The system MUST define a `Transaction` struct with fields: `ID` (int64), `Date` (string, ISO-8601 YYYY-MM-DD), `Amount` (float64), `Kind` (string, CHECK: `expense`|`income`), `Account` (string, free-text, no FK), `CategoryID` (int64, nullable FK → `categories`), `Notes` (string, nullable), `Recurrence` (string, CHECK: `none`|`daily`|`weekly`|`monthly`, default `none`), `Currency` (string, denormalized from `KeyCurrency` at create time), `CreatedAt`, `UpdatedAt`, `DeletedAt` (nullable timestamp). Tags are attached via the `finance_tags` junction table.

#### Scenario: Create expense transaction

- **GIVEN** `KeyCurrency` is set to `"USD"`
- **WHEN** a transaction is created with date `"2026-06-13"`, amount `42.50`, kind `"expense"`, account `"checking"`, category `"food"`, notes `"lunch"`, recurrence `"none"`, tags `["work"]`
- **THEN** a row is inserted with `currency="USD"`, `deleted_at IS NULL`
- **AND** one row exists in `finance_tags` linking to tag `"work"`

#### Scenario: Create income transaction

- **WHEN** a transaction is created with kind `"income"`, amount `3000.00`, account `"salary"`
- **THEN** the row is inserted with kind `"income"` and `deleted_at IS NULL`

#### Scenario: Reject invalid kind

- **WHEN** a transaction is created with kind `"transfer"`
- **THEN** the operation fails with a validation error and no row is inserted

#### Scenario: Reject invalid recurrence

- **WHEN** a transaction is created with recurrence `"yearly"`
- **THEN** the operation fails with a validation error and no row is inserted

### Requirement: Update transaction

The system MUST replace all mutable fields (date, amount, kind, account, category_id, notes, recurrence) and re-attach tags via `domain.WithTags` with `pruneOrphans=true`. The `updated_at` column SHALL be auto-bumped by the database trigger.

#### Scenario: Update changes amount and tags

- **WHEN** a transaction with amount `10.00` and tags `["a"]` is updated to amount `20.00` and tags `["b"]`
- **THEN** the row's amount is `20.00` and `finance_tags` contains only `["b"]`

#### Scenario: Update non-existent transaction fails

- **WHEN** an update targets an ID that does not exist
- **THEN** the operation returns an error and no rows are modified

### Requirement: Soft-delete, restore, purge

The system MUST support soft-delete (set `deleted_at`), restore (clear `deleted_at`), and purge (hard-delete row + cascade `finance_tags`). Soft-delete is idempotent (`WHERE deleted_at IS NULL`). Restore is idempotent (`WHERE deleted_at IS NOT NULL`).

#### Scenario: Soft-delete sets deleted_at

- **WHEN** an active transaction is soft-deleted
- **THEN** `deleted_at` is set to a non-NULL timestamp
- **AND** the transaction no longer appears in default listings

#### Scenario: Restore clears deleted_at

- **WHEN** a soft-deleted transaction is restored
- **THEN** `deleted_at` is `NULL` and the transaction reappears in default listings

#### Scenario: Purge hard-deletes row and tags

- **WHEN** a transaction is purged
- **THEN** the row is removed from `finance_transactions`, `finance_tags`, and `finance_fts`

### Requirement: List with filter

The system MUST provide a `List` method accepting filters: date range (`from`, `to`), `kind`, `category_id`, `tag` name, `trashed` (bool). Results SHALL be sorted by `date DESC`, then `created_at DESC`.

#### Scenario: List filtered by date range and kind

- **GIVEN** 5 expenses and 3 incomes exist in June 2026
- **WHEN** List is called with `from="2026-06-01"`, `to="2026-06-30"`, `kind="expense"`
- **THEN** exactly 5 expense transactions are returned sorted by date DESC

#### Scenario: List trashed

- **WHEN** List is called with `trashed=true`
- **THEN** only soft-deleted transactions are returned

### Requirement: Attacher for domain.WithTags

The system MUST provide `internal/finance/attacher.go` implementing the `domain.Attacher` interface, mirroring `todos/attacher.go`. The attacher SHALL manage `finance_tags` junction rows.

#### Scenario: Attacher creates junction rows

- **WHEN** `WithTags` is called with tag names `["urgent", "casa"]` for transaction ID 5
- **THEN** two rows exist in `finance_tags` linking transaction 5 to the corresponding tag IDs

## Out of Scope

- Multi-currency conversion, budget tracking, double-entry bookkeeping.
- Recurrence auto-expansion (metadata-only placeholder).
- Bulk operations.
