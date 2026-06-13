# Proposal: timezone-v3.0.1 — User-Configurable Timezone for Date Comparisons

## Why

Arsenal v3.0 computes "today" exclusively in UTC (`time.Now().UTC()`, SQLite `date('now')`). For the maintainer in `America/Argentina/Buenos_Aires` (UTC−3), a todo due "tomorrow local" lands in the wrong section after 21:00 local time. The maintainer opens Today 5+ times per day — this is a daily-driver UX defect, not a theoretical edge case.

[ADR-0003](../../../docs/adr/0003-timezone-handling.md) (accepted 2026-06-11) resolves the design: add a single `KeyUserTimezone` config key with default `"UTC"` so non-UTC users get correct day boundaries. The fix is ~50 LOC across 4 call sites + 1 SQL expression, with backwards-compatible default behavior.

## What Changes

- **New config key**: `KeyUserTimezone` in `internal/config/keys.go` — IANA timezone string, default `"UTC"`, no enum. Invalid values silently fall back to UTC (follows `KeyLandingSurface` pattern from phase 3).
- **New helper**: `UserLocation(ctx, db) (*time.Location, error)` in `internal/today/user_location.go` — single source of truth for "today" computation.
- **4 call sites migrated** from `time.Now().UTC()` to `time.Now().In(UserLocation(...))`:
  1. `internal/today/providers/todos.go` — overdue / due-today / upcoming section boundaries
  2. `internal/todos/service.go` — `CountOverdueTodos`, `ListOverdueTodos`
  3. `internal/web/todos.go` — overdue badge + list overdue filter
  4. `internal/web/handlers.go` — `date('now')` → Go-bound parameter for sidebar count
- **2 acceptance tests**: one in service layer, one in web layer (non-UTC overdue behavior).
- **CHANGELOG**: remove "Timezone handling" from Known limitations.

## Impact

### Affected Specs

| Spec | Action | Reason |
|------|--------|--------|
| `today-providers` | AMEND | TodosProvider "today" boundary must respect user timezone. New scenario: "WHEN user timezone is configured, overdue/due-today/upcoming sections use that timezone's day boundaries." |
| `todo-web` | AMEND | Sidebar overdue badge and list overdue filter must respect user timezone. New scenario: "WHEN user timezone is configured, overdue count reflects configured timezone, not UTC." |

### Untouched Specs (12)

`today-view`, `today-tui`, `today-web`, `today-empty-state`, `todo-tui`, `todo-cli`, `todo-lifecycle`, `todo-listing`, `todo-recurrence-placeholder`, `todo-search`, `todo-status`, `todo-tags` — no requirement change. Rendering layers consume what providers/service return; the Provider interface (`today-view`) is unchanged.

### New Code Locations

| File | Purpose |
|------|---------|
| `internal/today/user_location.go` | `UserLocation` helper |
| `internal/today/user_location_test.go` | Helper tests (unset=UTC, valid IANA, invalid=UTC+log) |
| `internal/configstore/keys_test.go` | `KeyUserTimezone` default test |

### Modified Code Locations

| File | Change |
|------|--------|
| `internal/config/keys.go` | Add `KeyUserTimezone` key definition |
| `internal/today/providers/todos.go` | `time.Now().UTC()` → `UserLocation` (call site 1) |
| `internal/todos/service.go` | `time.Now().UTC()` → `UserLocation` (call site 2) |
| `internal/web/todos.go` | `time.Now().UTC()` → `UserLocation` (call site 3) |
| `internal/web/handlers.go` | `date('now')` → Go-bound `?` param (call site 4) |
| `internal/todos/service_test.go` | Overdue timezone acceptance test |
| `internal/web/todos_test.go` | Overdue badge timezone acceptance test |
| `CHANGELOG.md` | Remove timezone limitation line |

### Migrations

None. No schema changes. All `due_date` columns remain `TEXT` in `YYYY-MM-DD` format — a calendar day, not a timestamp. Behavior change is gated entirely on the user setting `KeyUserTimezone`.

## Rollout / Rollback

**Feature-flag style.** Default `"UTC"` means users without the config key see zero behavior change. Rollback: revert the 8-file commit set. No migration to unwind, no data to backfill.

## Open Questions

None. ADR-0003 resolved all design decisions (storage format, display layer, config key shape, validation strategy, SQL rewrite approach). The implementation path is fully specified by the ADR and [tasks.md](./tasks.md) (8 tasks, 4 phases).

## References

- [ADR-0003](../../../docs/adr/0003-timezone-handling.md) — full rationale, alternatives considered (4 alternatives rejected)
- [tasks.md](./tasks.md) — implementation breakdown (Phase 1: config + helper, Phase 2: 4 call sites, Phase 3: acceptance tests, Phase 4: docs)
- [phase-3-today proposal](../archive/2026-06-11-phase-3-today/proposal.md) Q3 — originally deferred timezone to v3.0.1
