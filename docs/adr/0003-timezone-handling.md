# ADR-0003: Timezone Handling for Date-Based Comparisons

**Status:** Accepted
**Date:** 2026-06-11
**Context:** Date-based comparisons in Arsenal (overdue, due-today, upcoming, this-week) all use UTC. For users in non-UTC timezones, "today" and "due today" can be off by hours near the day boundary. This was flagged as a v3.0 known limitation; this ADR decides the fix.

---

## Context

Arsenal v3.0 stores dates as `TEXT` in `YYYY-MM-DD` format. The "today" computation in date comparisons uses Go's `time.Now().UTC().Format("2006-01-02")` and SQLite's `date('now')` — both UTC. For the maintainer in `America/Argentina/Buenos_Aires` (UTC−3) at 22:00 local, "today" in UTC is already the next day. So a todo due "tomorrow local" shows as "due today" in the DB and lands in the wrong section.

This affects four call sites:

- `internal/today/providers/todos.go:28` — Today / due-today / upcoming sections
- `internal/todos/service.go:209` — Overdue check
- `internal/web/todos.go:48, 412` — Overdue badge, list overdue filter
- `internal/web/handlers.go:627` — `WHERE due_date < date('now')` for the sidebar count

The user opens Today 5+ times per day. Off-by-one-day is a real UX bug, not a theoretical one.

---

## Decision

Add a `KeyUserTimezone` config key (IANA timezone, e.g., `America/Argentina/Buenos_Aires`) that:

1. Defaults to `UTC` (backwards compatible with v3.0).
2. Is read by all four call sites that compute "today".
3. Affects ONLY date comparisons, not full timestamps. Backup file names and audit timestamps stay in UTC (they are operations, not human time).

### Storage stays UTC

All `due_date` columns remain `TEXT` in `YYYY-MM-DD` format representing a calendar day. We do **not** store a timezone with each row. Rationale: a todo is "due 2026-06-15" — that's a day, not a moment. The user's timezone determines which day is "today", not which day the due date falls on.

### Display layer: local

When rendering dates to the user (TUI, web, CLI), display the date in the user's configured timezone. Since dates are stored as `YYYY-MM-DD` (no time component), rendering is already locale-agnostic — the string itself is shown verbatim. The only thing that changes is "what is today" when comparing.

### Configuration

New key in `internal/config/keys.go`:

```go
KeyUserTimezone: configstore.KeyDef{
    Type:        configstore.TypeString,
    Default:     "UTC",
    Description: "IANA timezone for date comparisons (e.g., America/Argentina/Buenos_Aires)",
    EnumValues:  nil, // free-form IANA, validated via time.LoadLocation
},
```

Validation on read: `time.LoadLocation(value)`. Invalid values fall back to `UTC` with a warning logged (not surfaced to the user — keeps the silent-fallback pattern from `KeyLandingSurface`).

### Implementation touchpoints

For each call site that uses `time.Now().UTC()` to compute "today":

```go
// before
now := time.Now().UTC()
today := now.Format("2006-01-02")

// after
loc := userLocation(ctx, db) // returns *time.Location
now := time.Now().In(loc)
today := now.Format("2006-01-02")
```

For the SQL call site (`handlers.go:627`):

```go
// before
SELECT COUNT(*) FROM todos WHERE due_date < date('now')

// after
SELECT COUNT(*) FROM todos WHERE due_date < ?  // bound to todayStr computed in Go
```

The SQL rewrite moves "today" into Go, consistent with the other call sites.

### Migration

No schema migration needed. Existing `due_date` values are interpreted the same way (they're already calendar days). Behavior change is gated on the user setting `KeyUserTimezone`.

---

## Consequences

### Positive

- Off-by-one-day bug fixed for non-UTC users (the maintainer, the primary user).
- Single source of truth for "today": one helper function reads the config once and returns a `*time.Location`.
- Backwards compatible: users who don't configure the key see no behavior change.
- Future-friendly: when finance/calendar domains ship, they reuse the same helper for due/period calculations.

### Negative

- One more config key the user must know about (mitigated by documenting the date-comparison impact in the config key description).
- SQL `date('now')` must be replaced with a Go-computed value, which means the query is no longer a constant string. Slight readability cost; we add a comment.
- Tests that hardcode UTC dates need to either use the helper or continue testing UTC behavior with the default. We add a parallel test for the configured-tz path.

### Neutral

- Backup file names, audit timestamps, and `created_at` / `updated_at` columns continue to use UTC. These are operations, not user-facing times.
- FTS5 index columns (`todo_search.due_date`) stay as-is; the index is text, not temporal.

---

## Alternatives considered

### A. Store full ISO 8601 timestamps with timezone

`due_date` becomes `2026-06-15T00:00:00-03:00`. Comparison is exact. **Rejected** because a todo "due 2026-06-15" is a calendar day, not a moment at midnight. Storing timezones per row complicates queries, storage, and display. The user's local timezone at view time is the right semantics.

### B. Always use the OS local timezone via `time.Local`

`time.Now()` without `.UTC()` returns the OS local time. **Rejected** because:
- The web server may run in a different timezone than the user (containers, remote hosts).
- A user might want to log todos with one timezone and view them in another.
- Making the timezone explicit in config beats implicit OS behavior.

### C. Per-row timezone

Each todo has its own timezone. **Rejected** for the same reason as A — a todo's due date is a calendar day, not a moment.

### D. Date-only comparison, no timezone awareness

`due_date` compared as text: "is `2026-06-15` < `2026-06-16`?" works. But "what is `today`?" still needs a timezone. This is a partial fix that still suffers the original bug. **Rejected**.

---

## Implementation plan (separate from this ADR)

1. Add `KeyUserTimezone` to `internal/config/keys.go` (1 key, default UTC).
2. Add a `userLocation(ctx, db) (*time.Location, error)` helper in `internal/today/`.
3. Replace `time.Now().UTC()` with the helper at the four call sites.
4. Replace `date('now')` in `handlers.go:627` with a Go-bound parameter.
5. Add tests:
   - Config defaults to UTC.
   - Config rejects invalid IANA names with silent fallback.
   - A todo due today in the configured tz appears in the right section.
   - A todo due tomorrow in the configured tz does NOT appear in due-today.
6. Update `CHANGELOG.md` Known limitations: remove the timezone line.

Estimated effort: ~50 LOC, 1 config key, 1 helper, 4 call sites, ~6 tests.

---

## References

- ADR-0001 §6: "Single timezone (system)" — original v3 assumption.
- ADR-0002: scope/sequencing replan; did not address timezone.
- `internal/today/providers/todos.go:28` — first affected call site.
- `internal/web/handlers.go:627` — only SQL `date('now')` use.
