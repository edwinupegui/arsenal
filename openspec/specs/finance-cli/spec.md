# finance-cli

## Purpose

Cobra subcommands under `arsenal finance` exposing all finance operations from the command line. Primary surface for terminal-first users and shell automation via `--json` output.

## Requirements

### Requirement: Parent command and subcommand tree

The system MUST provide `arsenal finance` with subcommands: `add`, `list`, `show`, `edit`, `rm`, `restore`, `purge`, `export`. The parent command SHALL print help when invoked without a subcommand.

#### Scenario: Finance help lists subcommands

- **WHEN** the user runs `arsenal finance`
- **THEN** available subcommands are printed: add, list, show, edit, rm, restore, purge, export

### Requirement: Add subcommand

The system MUST provide `arsenal finance add` with flags: `--date` (YYYY-MM-DD, default today), `--amount` (float, required), `--kind` (expense|income, required), `--account` (string), `--cat` (category slug), `--tag` (repeatable), `--notes`, `--recurrence` (none|daily|weekly|monthly, default none). Output is text by default, JSON with `--json`.

#### Scenario: Add expense with all flags

- **WHEN** the user runs `arsenal finance add --date 2026-06-13 --amount 42.50 --kind expense --account checking --cat food --tag work --notes "lunch"`
- **THEN** a transaction is created with all specified fields
- **AND** the CLI prints the created transaction's ID to stdout

#### Scenario: Add without required amount fails

- **WHEN** the user runs `arsenal finance add --kind expense`
- **THEN** the CLI prints an error about missing `--amount` and exits non-zero

#### Scenario: Add with invalid kind fails

- **WHEN** the user runs `arsenal finance add --amount 10 --kind transfer`
- **THEN** the CLI prints `"Error: kind must be expense or income"` and exits non-zero

### Requirement: List subcommand

The system MUST provide `arsenal finance list` with flags: `--from`, `--to` (date range), `--kind`, `--cat`, `--tag`, `--trashed` (bool), `--json`.

#### Scenario: List with date range filter

- **WHEN** the user runs `arsenal finance list --from 2026-06-01 --to 2026-06-30`
- **THEN** only transactions within June 2026 are printed

#### Scenario: List with JSON output

- **WHEN** the user runs `arsenal finance list --json`
- **THEN** output is a valid JSON array of transaction objects

### Requirement: Show, edit, rm, restore, purge subcommands

The system MUST provide `show <id>`, `edit <id>` (same flags as add plus `--title`), `rm <id>` (soft-delete), `restore <id>`, and `purge <id>` (requires `--yes` or TTY confirmation).

#### Scenario: Show existing transaction

- **WHEN** the user runs `arsenal finance show 5`
- **THEN** full transaction detail is printed including all fields and tags

#### Scenario: Purge without --yes in non-interactive mode fails

- **WHEN** the user runs `arsenal finance purge 5` with stdin not a TTY and no `--yes`
- **THEN** the CLI prints `"Error: --yes required in non-interactive mode"` and exits non-zero

### Requirement: Shell completions

The system MUST register completions for `arsenal finance` subcommands and flag values (kind, recurrence, category slugs).

#### Scenario: Tab-complete kind values

- **WHEN** the user types `arsenal finance add --kind <TAB>`
- **THEN** completions include `expense` and `income`

## Out of Scope

- Interactive TUI from CLI, piped bulk import.
