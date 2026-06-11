# todo-cli

## Purpose

Defines the Cobra subcommands under `arsenal todo` that expose all todo operations from the command line. This is the primary surface for terminal-first users and the integration point for shell scripts and automation via `--json` output.

## Requirements

### Requirement: Add subcommand

The system MUST provide `arsenal todo add <title>` with flags `--priority` (low|med|high, default med), `--due` (YYYY-MM-DD), `--cat` (category slug), `--tag` (repeatable), `--notes`, `--recurrence` (none|daily|weekly|monthly, default none), and `--desc` (description). Title is required as the first positional argument. Output is text by default, JSON with `--json`.

#### Scenario: Add with all flags

- **WHEN** the user runs `arsenal todo add "pagar luz" --priority high --due 2026-06-10 --tag urgente --tag casa --notes "mensual" --recurrence weekly`
- **THEN** a todo is created with all specified fields
- **AND** the CLI prints the created todo's ID and title to stdout
- **AND** exits with code 0

#### Scenario: Add without title fails

- **WHEN** the user runs `arsenal todo add` with no positional argument
- **THEN** the CLI prints `"Error: title is required"` to stderr and exits non-zero

### Requirement: List subcommand

The system MUST provide `arsenal todo list` with flags `--status` (open|done, default open), `--priority`, `--overdue` (bool), `--cat` (slug), `--tag` (name), `--trashed` (bool), `--due-before` (YYYY-MM-DD), `--limit` (default 50), `--offset` (default 0), and `--json`.

#### Scenario: List filtered by priority and overdue

- **WHEN** the user runs `arsenal todo list --priority high --overdue`
- **THEN** only open, high-priority, overdue todos are printed
- **AND** each line shows ID, title, priority, due date, and tags

#### Scenario: List with JSON output

- **WHEN** the user runs `arsenal todo list --json`
- **THEN** output is a valid JSON array of todo objects

### Requirement: Show subcommand

The system MUST provide `arsenal todo show <id>` that prints full detail of a single todo. With `--json`, outputs a JSON object.

#### Scenario: Show existing todo

- **WHEN** the user runs `arsenal todo show 42` and todo 42 exists
- **THEN** title, description, priority, status, due date, category, tags, notes, recurrence, and timestamps are printed

#### Scenario: Show non-existent ID errors

- **WHEN** the user runs `arsenal todo show 99999` and no such todo exists
- **THEN** the CLI prints `"Error: todo not found"` to stderr and exits non-zero

### Requirement: Done and open subcommands

The system MUST provide `arsenal todo done <id>` and `arsenal todo open <id>` that transition the todo status per `todo-status` spec.

#### Scenario: Done transitions open to done

- **WHEN** the user runs `arsenal todo done 5` on an open todo
- **THEN** the todo's status becomes `done`, `done_at` is set, and the CLI confirms the transition

#### Scenario: Open transitions done to open

- **WHEN** the user runs `arsenal todo open 5` on a done todo
- **THEN** the todo's status becomes `open`, `done_at` is cleared, and the CLI confirms

### Requirement: Rm, restore, and purge subcommands

The system MUST provide `arsenal todo rm <id>` (soft-delete), `arsenal todo restore <id>` (clear soft-delete), and `arsenal todo purge <id>` (hard-delete). Purge MUST require either `--yes` flag or interactive confirmation when stdin is a TTY. In non-interactive mode without `--yes`, purge SHALL fail.

#### Scenario: Rm soft-deletes

- **WHEN** the user runs `arsenal todo rm 5`
- **THEN** the todo is soft-deleted and the CLI confirms

#### Scenario: Restore brings back

- **WHEN** the user runs `arsenal todo restore 5` on a soft-deleted todo
- **THEN** the todo is restored and the CLI confirms

#### Scenario: Purge with --yes flag

- **WHEN** the user runs `arsenal todo purge 5 --yes`
- **THEN** the todo is hard-deleted without prompting

#### Scenario: Purge without --yes in non-interactive mode fails

- **WHEN** the user runs `arsenal todo purge 5` with stdin not a TTY and no `--yes` flag
- **THEN** the CLI prints `"Error: --yes required in non-interactive mode"` and exits non-zero

### Requirement: Edit subcommand

The system MUST provide `arsenal todo edit <id>` with the same flags as `add` (minus the positional title, replaced by `--title`). All provided flags update the corresponding fields; omitted flags leave fields unchanged.

#### Scenario: Edit changes title and priority

- **WHEN** the user runs `arsenal todo edit 5 --title "new title" --priority high`
- **THEN** the todo's title and priority are updated; other fields remain unchanged

### Requirement: Shell completions

The system MUST register completions for `arsenal todo` subcommands and flags (priority values, status values, recurrence values, category slugs from the database).

#### Scenario: Tab-complete priority values

- **WHEN** the user types `arsenal todo add "test" --priority <TAB>`
- **THEN** completions include `low`, `med`, `high`

## Out of Scope

- Interactive TUI launched from CLI (that's `arsenal tui`, a separate command).
- Batch operations (add/list multiple todos in one invocation).
- Piped input for bulk import (deferred).
