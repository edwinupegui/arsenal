# todo-status

## Purpose

Defines the state machine for todo completion: transitioning between `open` and `done`. This is intentionally separate from the general lifecycle update to keep the done/open semantics explicit and testable. The `done_at` timestamp is the single source of truth for "when was this completed".

## Requirements

### Requirement: Mark done

The system MUST transition a todo from `open` to `done` by setting `status = 'done'` and `done_at = now()` (UTC ISO-8601). If the todo is already `done`, the operation SHALL be a no-op: `done_at` MUST NOT be overwritten and `updated_at` MUST NOT be bumped.

#### Scenario: Open to done sets done_at

- **WHEN** a todo with `status = 'open'` and `done_at IS NULL` is marked done
- **THEN** `status` becomes `'done'` and `done_at` is set to the current UTC timestamp
- **AND** `updated_at` is bumped by the database trigger

#### Scenario: Done to done is a no-op

- **WHEN** a todo with `status = 'done'` is marked done again
- **THEN** `status`, `done_at`, and `updated_at` are all unchanged
- **AND** no error is returned

### Requirement: Mark open

The system MUST transition a todo from `done` to `open` by setting `status = 'open'` and clearing `done_at` to `NULL`. If the todo is already `open`, the operation SHALL be a no-op.

#### Scenario: Done to open clears done_at

- **WHEN** a todo with `status = 'done'` and `done_at IS NOT NULL` is marked open
- **THEN** `status` becomes `'open'` and `done_at` is set to `NULL`
- **AND** `updated_at` is bumped

#### Scenario: Open to open is a no-op

- **WHEN** a todo with `status = 'open'` is marked open again
- **THEN** no columns change and no error is returned

## Out of Scope

- Partial completion or sub-task tracking.
- Completion percentage or progress indicators.
- Automatic status transitions based on due dates.
- Status values beyond `open` and `done` (no `cancelled`, `deferred`, `waiting` in v3.0).
