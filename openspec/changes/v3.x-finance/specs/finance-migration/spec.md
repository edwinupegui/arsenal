# finance-migration

## Purpose

Schema migration for the Finance domain. Creates `finance_transactions`, `finance_tags`, `finance_fts` (FTS5 virtual table), sync triggers, and indices. Forward-only migration per ADR-0001.

## Requirements

### Requirement: finance_transactions table

The migration `20260613000000_finance.sql` MUST create the `finance_transactions` table with columns: `id` (INTEGER PRIMARY KEY AUTOINCREMENT), `date` (TEXT NOT NULL), `amount` (REAL NOT NULL), `kind` (TEXT NOT NULL, CHECK: `expense`|`income`), `account` (TEXT NOT NULL DEFAULT ''), `category_id` (INTEGER, FK → `categories.id`, nullable), `notes` (TEXT), `recurrence` (TEXT NOT NULL DEFAULT 'none', CHECK: `none`|`daily`|`weekly`|`monthly`), `currency` (TEXT NOT NULL DEFAULT 'USD'), `created_at` (TEXT NOT NULL DEFAULT strftime), `updated_at` (TEXT NOT NULL DEFAULT strftime), `deleted_at` (TEXT).

#### Scenario: Table created on fresh install

- **WHEN** the migration runs on a fresh database
- **THEN** `finance_transactions` exists with all specified columns and constraints

#### Scenario: CHECK constraint rejects invalid kind

- **WHEN** an INSERT attempts `kind = 'transfer'`
- **THEN** SQLite rejects the row with a CHECK constraint failure

### Requirement: finance_tags junction table

The migration MUST create `finance_tags` with columns: `finance_id` (INTEGER NOT NULL, FK → `finance_transactions.id`), `tag_id` (INTEGER NOT NULL, FK → `tags.id`), `PRIMARY KEY (finance_id, tag_id)`.

#### Scenario: Junction table links transactions and tags

- **WHEN** a row is inserted into `finance_tags` with valid `finance_id` and `tag_id`
- **THEN** the row exists linking the transaction to the tag

### Requirement: finance_fts FTS5 virtual table

The migration MUST create `finance_fts` as an FTS5 virtual table on columns `notes` and `account`. Sync triggers MUST fire AFTER INSERT, AFTER UPDATE, and AFTER DELETE on `finance_transactions` to keep the FTS index consistent.

#### Scenario: FTS5 index syncs on insert

- **WHEN** a transaction is inserted with notes `"lunch meeting"`
- **THEN** a search for `"lunch"` in `finance_fts` returns the transaction

#### Scenario: FTS5 index syncs on update

- **WHEN** a transaction's notes are updated from `"lunch"` to `"dinner"`
- **THEN** searching `"lunch"` no longer returns the transaction
- **AND** searching `"dinner"` returns it

#### Scenario: FTS5 index syncs on delete

- **WHEN** a transaction is deleted
- **THEN** the corresponding FTS entry is removed

### Requirement: Indices

The migration MUST create indices: `idx_finance_date` on `date`, `idx_finance_kind` on `kind`, `idx_finance_deleted` on `deleted_at`, `idx_finance_category` on `category_id`.

#### Scenario: Indices exist after migration

- **WHEN** the migration completes
- **THEN** all four indices exist and are queryable

### Requirement: IF NOT EXISTS safety

All CREATE TABLE, CREATE INDEX, and CREATE TRIGGER statements MUST use `IF NOT EXISTS` guards so re-runs are safe.

#### Scenario: Re-running migration is safe

- **WHEN** the migration is run twice on the same database
- **THEN** no errors occur and the schema is unchanged after the second run

### Requirement: Forward-only

The migration MUST NOT include a DOWN migration. Rollback instructions MAY be included as SQL comments.

#### Scenario: No down migration exists

- **WHEN** inspecting the migration file
- **THEN** no `-- goose down` section contains destructive DROP statements

## Out of Scope

- Down migration, data migration from external sources, schema changes to existing tables.
