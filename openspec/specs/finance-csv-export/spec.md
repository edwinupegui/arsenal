# finance-csv-export

## Purpose

CSV export command for finance transactions. Exports per-transaction rows via `arsenal finance export --format csv` to stdout or file. Primary data portability feature for the Finance domain.

## Requirements

### Requirement: CSV format and columns

The system MUST produce a CSV with a header row followed by one data row per transaction. Columns in order: `date`, `kind`, `amount`, `currency`, `account`, `category`, `notes`, `tags`. The file MUST be UTF-8 encoded with comma delimiter. The `tags` column SHALL contain comma-separated tag names within the cell (quoted per RFC 4180).

#### Scenario: Header row is correct

- **WHEN** `arsenal finance export --format csv` is run
- **THEN** the first line is `date,kind,amount,currency,account,category,notes,tags`

#### Scenario: Tags column is comma-separated within quoted cell

- **GIVEN** a transaction with tags `["work", "urgent"]`
- **WHEN** the transaction is exported
- **THEN** the tags column contains `"work,urgent"` (quoted)

#### Scenario: Special characters in notes are escaped

- **GIVEN** a transaction with notes containing commas and quotes: `lunch, "expensive"`
- **WHEN** the transaction is exported
- **THEN** the notes field is properly CSV-escaped per RFC 4180

### Requirement: Output destination

The system MUST write CSV to stdout by default. When `--output <path>` is provided, the system SHALL write to the specified file path instead.

#### Scenario: Default output to stdout

- **WHEN** the user runs `arsenal finance export --format csv`
- **THEN** CSV content is written to stdout

#### Scenario: Output to file with --output

- **WHEN** the user runs `arsenal finance export --format csv --output /tmp/finance.csv`
- **THEN** the file `/tmp/finance.csv` contains the CSV content
- **AND** nothing is written to stdout

### Requirement: Filter flags

The system MUST support filter flags: `--from` (start date), `--to` (end date), `--kind` (expense|income), `--cat` (category slug), `--tag` (tag name). Filters SHALL combine with AND logic.

#### Scenario: Filtered export by date range

- **GIVEN** 10 transactions spanning May and June 2026
- **WHEN** the user runs `arsenal finance export --format csv --from 2026-06-01 --to 2026-06-30`
- **THEN** only June transactions appear in the CSV (plus header row)

#### Scenario: Filtered export by kind

- **WHEN** the user runs `arsenal finance export --format csv --kind expense`
- **THEN** only expense transactions appear in the CSV

### Requirement: Empty export

When no transactions match the filters, the system MUST still output the header row.

#### Scenario: Empty export produces header only

- **GIVEN** zero transactions exist
- **WHEN** the user runs `arsenal finance export --format csv`
- **THEN** output is exactly the header row: `date,kind,amount,currency,account,category,notes,tags`

## Out of Scope

- Monthly summary aggregation, XLSX/JSON export formats, bank-statement import.
