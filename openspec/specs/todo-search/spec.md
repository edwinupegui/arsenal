# todo-search

## Purpose

Defines full-text search over the `todos_fts` FTS5 virtual table. This capability provides per-domain search that produces results in a shape compatible with the cross-domain UNION ALL strategy defined in ADR-0002 Change 5. The UNION ALL query itself is NOT built in v3.0 — that ships in phase 3 when the Today view aggregates results.

## Requirements

### Requirement: Prefix search with sanitization

The system MUST accept a user query string, sanitize it via `store.stripFTSSpecials` (or an extracted shared helper in `store/fts.go`), convert it to an FTS5 prefix expression via `store.buildFTSQuery`, and execute a MATCH against `todos_fts`. The search MUST NOT crash on special characters (`*`, `"`, `(`, `)`, `+`, `-`).

#### Scenario: Search by title prefix

- **WHEN** searching for `"pag"`
- **THEN** todos with titles starting with or containing `"pag"` are returned, ordered by FTS5 rank

#### Scenario: Search matches description

- **WHEN** searching for `"invoice"` and a todo has description `"monthly invoice payment"`
- **THEN** that todo is included in results

#### Scenario: Search matches notes

- **WHEN** searching for `"rutina"` and a todo has notes `"rutina de mañana"`
- **THEN** that todo is included in results

#### Scenario: Search matches tag names

- **WHEN** searching for `"urgente"` and a todo has tag `"urgente"` (indexed via `v_todo_tags` trigger)
- **THEN** that todo is included in results

#### Scenario: Special characters do not crash

- **WHEN** searching for `"c++"`, `"foo*bar"`, or `"(test)"`
- **THEN** the query is sanitized and executed without error
- **AND** results are returned (possibly empty) rather than a SQL error

### Requirement: Result shape for cross-domain union

The system MUST return search results in a standardized shape: `{domain: "todo", id, title, snippet, score}`. This shape matches the cross-domain union contract from ADR-0002 Change 5 so that phase 3 can UNION ALL across `resources_fts` and `todos_fts` without reshaping.

#### Scenario: Result shape contains domain tag

- **WHEN** a search returns a matching todo
- **THEN** the result includes `domain = "todo"`, the todo's `id`, `title`, a `snippet` from the matched FTS5 column, and a `score` from FTS5 rank

### Requirement: Exclude soft-deleted todos

The system MUST exclude todos where `deleted_at IS NOT NULL` from search results.

#### Scenario: Search excludes trashed todos

- **WHEN** searching for a term that matches a soft-deleted todo
- **THEN** the soft-deleted todo is NOT included in results

## Out of Scope

- The cross-domain UNION ALL query itself (phase 3, Today view).
- Search UI (handled per-surface in `todo-cli`, `todo-tui`, `todo-web`).
- Fuzzy search, typo tolerance, or synonym expansion beyond FTS5 `unicode61` tokenizer.
- Search over categories or other joined tables (FTS5 indexes only `title`, `description`, `notes`, `tags`).
