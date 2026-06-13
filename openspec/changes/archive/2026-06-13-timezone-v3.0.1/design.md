# Design: timezone-v3.0.1 — User-Configurable Timezone for Date Comparisons

## Architecture Overview

This change adds timezone-aware "today" computation across 4 call sites and 1 SQL expression. The design rationale lives in [ADR-0003](../../../docs/adr/0003-timezone-handling.md); this document is the implementation blueprint.

A single helper — `today.UserLocation(ctx, db) (*time.Location, error)` — reads `KeyUserTimezone` from `configstore`, resolves it via `time.LoadLocation`, and returns `*time.Location` (UTC on error). Every call site that computes "today" switches from `time.Now().UTC()` to `time.Now().In(loc)`. Storage stays UTC (`YYYY-MM-DD` text); only the "what is today" boundary shifts.

**Phase 1 (config + helper) and call site 1 (TodosProvider) are already implemented.** This design covers the remaining 3 call sites, 1 SQL rewrite, acceptance tests, and docs.

## Component Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                        configstore.Store                            │
│  arsenal_config WHERE k = "user_timezone"  →  "America/Argentina/…" │
└──────────────────────────┬──────────────────────────────────────────┘
                           │ GetDefault(ctx, KeyUserTimezone)
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│              today.UserLocation(ctx, db) → *time.Location           │
│  time.LoadLocation(value) → loc | invalid → log.Printf + time.UTC  │
└──────────────────────────┬──────────────────────────────────────────┘
                           │ *time.Location
          ┌────────────────┼────────────────┬──────────────┐
          ▼                ▼                ▼              ▼
   ┌─────────────┐ ┌──────────────┐ ┌────────────┐ ┌──────────────┐
   │ Call site 1 │ │ Call site 2  │ │ Call site 3│ │ Call site 4  │
   │ providers/  │ │ todos/       │ │ web/       │ │ web/         │
   │ todos.go    │ │ service.go   │ │ todos.go   │ │ handlers.go  │
   │ (DONE)      │ │ List()       │ │ listTodos  │ │ countOverdue │
   │             │ │              │ │ isOverdue  │ │ SQL rewrite  │
   └─────────────┘ └──────────────┘ └────────────┘ └──────────────┘
        │                │                │              │
        └────────────────┴────────────────┴──────────────┘
                           │
                           ▼
                  time.Now().In(loc).Format("2006-01-02")
                  → todayStr used in SQL WHERE / Go comparisons
```

## Sequence: UserLocation Helper

```
Caller                    UserLocation              configstore           time
  │                            │                        │                  │
  │──UserLocation(ctx,db)─────▶│                        │                  │
  │                            │──GetDefault("user_tz")▶│                  │
  │                            │◀──"America/Arg…"/err───│                  │
  │                            │                        │                  │
  │                            │──LoadLocation(val)───────────────────────▶│
  │                            │◀──*Location / err────────────────────────│
  │                            │                        │                  │
  │                            │ [if err: log.Printf, return UTC]         │
  │◀──*time.Location──────────│                        │                  │
```

## Data Flow

**Storage**: all `due_date` columns remain `TEXT` in `YYYY-MM-DD` — a calendar day, not a timestamp. No schema change.

**Display**: dates render verbatim as stored strings. The timezone does NOT alter how a date looks — only which day is "today" when comparing.

**Comparison**: the timezone shifts the `time.Now()` anchor. At UTC 02:00 on June 12, a user in `America/Argentina/Buenos_Aires` (UTC−3) sees local time 23:00 on June 11. "Today" becomes `2026-06-11` instead of `2026-06-12`. This single change propagates to overdue/due-today/upcoming boundaries and the sidebar overdue count.

## API Surface

### KeyUserTimezone (already implemented)

```go
// internal/config/keys.go:42
KeyUserTimezone Key = "user_timezone"

// Catalog entry (line 87-91)
KeyUserTimezone: {
    Type:        TypeString,
    Default:     "UTC",
    Description: "IANA timezone for date comparisons (e.g., America/Argentina/Buenos_Aires).",
},
```

No enum, no `Validate` func. Invalid IANA names pass `configstore.Set` and are caught at read time by `UserLocation`.

### UserLocation (already implemented)

```go
// internal/today/user_location.go:17
func UserLocation(ctx context.Context, db *sql.DB) (*time.Location, error)
```

Returns `time.UTC` on: config read error (wrapped), invalid IANA name (logged + nil error). Returns resolved `*time.Location` on success.

## Call Site Rewrites

### Call Site 1: `internal/today/providers/todos.go` — DONE

Already migrated in Phase 2.1. Lines 30-34 call `today.UserLocation(ctx, p.db)` and use `p.now().In(loc)`.

### Call Site 2: `internal/todos/service.go:207-209`

**Before** (line 207-209):
```go
var today string
if f.OnlyOverdue {
    today = time.Now().UTC().Format("2006-01-02")
}
```

**After**:
```go
var today string
if f.OnlyOverdue {
    loc, err := todayPkg.UserLocation(ctx, s.db)
    if err != nil {
        return nil, fmt.Errorf("user location: %w", err)
    }
    today = time.Now().In(loc).Format("2006-01-02")
}
```

The `Service` struct already holds `db *sql.DB` (line 23). New import: `"github.com/edwinupegui/arsenal/internal/today"` aliased as `todayPkg` to avoid collision with the local `today` variable.

**Test**: add `TestList_FilterOverdue_Timezone` in `service_test.go` — set `KeyUserTimezone` to `America/Argentina/Buenos_Aires`, create a todo due on a date that is overdue in UTC but due-today in Argentina, assert it does NOT appear in overdue results.

### Call Site 3: `internal/web/todos.go`

Two sub-sites:

#### 3a: `listTodos` line 48

**Before**:
```go
now := time.Now().UTC()
```

**After**:
```go
loc, err := today.UserLocation(r.Context(), h.db)
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
now := time.Now().In(loc)
```

New import: `"github.com/edwinupegui/arsenal/internal/today"`. `Handlers` already holds `db *sql.DB` (line 24).

#### 3b: `isOverdue` helper line 411-412

**Before**:
```go
func isOverdue(dueDate, status string) bool {
    return status == "open" && dueDate != "" && dueDate < time.Now().UTC().Format("2006-01-02")
}
```

**After**:
```go
func isOverdue(dueDate, status string, todayStr string) bool {
    return status == "open" && dueDate != "" && dueDate < todayStr
}
```

The caller computes `todayStr` once via `UserLocation` and passes it in. This keeps `isOverdue` a pure function — testable without DB. All 4 call sites of `isOverdue` (lines 71, 177, 356, and the `renderTodoCard` path) must pass the precomputed `todayStr`.

**Optimization**: `listTodos` already computes `now` from `UserLocation` — derive `todayStr` from the same `now`. For `showTodo` and `renderTodoCard`, compute `todayStr` once at the top of each handler.

**Test**: add `TestOverdueBadge_Timezone` in `todos_test.go` — set timezone, create borderline todo, assert sidebar count via `countOverdueTodos`.

### Call Site 4: `internal/web/handlers.go:624-629`

**Before**:
```go
func countOverdueTodos(ctx context.Context, db *sql.DB) (int64, error) {
    var n int64
    err := db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM todos WHERE status = 'open' AND deleted_at IS NULL AND due_date < date('now')`,
    ).Scan(&n)
    return n, err
}
```

**After**:
```go
func countOverdueTodos(ctx context.Context, db *sql.DB) (int64, error) {
    loc, err := today.UserLocation(ctx, db)
    if err != nil {
        return 0, fmt.Errorf("user location: %w", err)
    }
    todayStr := time.Now().In(loc).Format("2006-01-02")
    var n int64
    err = db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM todos WHERE status = 'open' AND deleted_at IS NULL AND due_date < ?`,
        todayStr,
    ).Scan(&n)
    return n, err
}
```

New imports: `"time"`, `"github.com/edwinupegui/arsenal/internal/today"`. The `date('now')` SQL function is replaced with a Go-bound `?` parameter — "today" is now computed in Go, consistent with all other call sites.

**Test**: covered by the acceptance test in `web/todos_test.go`.

## Error Handling

| Scenario | Behavior | Logger |
|----------|----------|--------|
| `KeyUserTimezone` unset | Returns `time.UTC`, no log | — |
| Valid IANA value | Returns resolved `*time.Location` | — |
| Invalid IANA value | Returns `time.UTC`, logs warning | `log.Printf` (stdlib) |
| DB read error | Returns `time.UTC` + wrapped error | caller decides |

**Logging convention**: the codebase uses `log.Printf` from stdlib exclusively (the only usage is `user_location.go:25`). No structured logger exists. The invalid-timezone warning follows the same pattern: `log.Printf("invalid user_timezone %q, falling back to UTC", v)`.

## Testing Strategy

Strict TDD per task: RED → GREEN → REFACTOR.

| Layer | File | Test | Boundary Case |
|-------|------|------|---------------|
| Unit | `internal/configstore/keys_test.go` | `TestGetDefault_UserTimezone_Unset` | Unset → "UTC" (DONE) |
| Unit | `internal/configstore/keys_test.go` | `TestSet_UserTimezone_InvalidIANA_Stored` | Invalid stored, not rejected at write (DONE) |
| Unit | `internal/today/user_location_test.go` | `TestUserLocation_Unset` | Unset → `time.UTC` (DONE) |
| Unit | `internal/today/user_location_test.go` | `TestUserLocation_Valid` | Valid IANA → location (DONE) |
| Unit | `internal/today/user_location_test.go` | `TestUserLocation_Invalid` | Invalid → UTC + log output (DONE) |
| Integration | `internal/todos/service_test.go` | `TestList_FilterOverdue_Timezone` | Argentina UTC−3 at 02:00 UTC: due-today, not overdue |
| Integration | `internal/web/todos_test.go` | `TestOverdueBadge_Timezone` | Sidebar count respects timezone |

**Boundary test scenario** (for both acceptance tests):
- Set `KeyUserTimezone = "America/Argentina/Buenos_Aires"` (UTC−3)
- Current UTC: `2026-06-12 02:00` → local: `2026-06-11 23:00`
- Create open todo with `due_date = "2026-06-11"`
- In UTC: overdue (2026-06-11 < 2026-06-12). In Argentina: due-today (2026-06-11 == 2026-06-11)
- Assert: todo does NOT appear in overdue results; sidebar overdue count = 0

**Test implementation note**: tests cannot control `time.Now()`. Use a fixed `due_date` relative to a known timezone offset and document the time-of-day constraint. Alternatively, the `TodosProvider` already accepts a `now func() time.Time` field — the service layer may need a similar injection point for deterministic testing.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/config/keys.go` | Already modified | `KeyUserTimezone` added (Phase 1, done) |
| `internal/today/user_location.go` | Already created | Helper function (Phase 1, done) |
| `internal/today/user_location_test.go` | Already created | 3 unit tests (Phase 1, done) |
| `internal/configstore/keys_test.go` | Already modified | 3 config tests (Phase 1, done) |
| `internal/today/providers/todos.go` | Already modified | Call site 1 migrated (Phase 2.1, done) |
| `internal/todos/service.go` | Modify | Call site 2: `List()` overdue filter uses `UserLocation` |
| `internal/web/todos.go` | Modify | Call site 3: `listTodos` + `isOverdue` signature change |
| `internal/web/handlers.go` | Modify | Call site 4: `countOverdueTodos` SQL rewrite |
| `internal/todos/service_test.go` | Modify | Add timezone acceptance test |
| `internal/web/todos_test.go` | Modify | Add timezone acceptance test |
| `CHANGELOG.md` | Modify | Remove timezone from Known Limitations |

## Migration / Rollback

No schema migration. No data migration. Default `"UTC"` preserves v3.0 behavior for all existing users.

**Rollback**: revert the commit set. No state to unwind.

## Open Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| `isOverdue` signature change ripples to 4 call sites | Medium — must update all callers or compilation breaks | Compute `todayStr` once per handler, pass to all `isOverdue` calls within that handler |
| `countOverdueTodos` called on every page render via `commonPage` | Low — adds 1 config read per request | `configstore.GetDefault` is a single-row PK lookup (<1ms). Acceptable for single-user CLI app |
| No `time.Now` injection in `service.go` or `web/` | Low — acceptance tests depend on wall clock | Use far-past/far-future dates for non-timezone tests; for timezone tests, document time-of-day constraint or add `now` injection |
| Concurrent config reads | None — SQLite WAL mode + single-user app | No mitigation needed |
| `isOverdue` in `renderTodoCard` path (HTMX mark-done) | Low — must compute `todayStr` in card render | Add `UserLocation` call in `renderTodoCard` |

## Cross-References

- **Rationale**: [ADR-0003](../../../docs/adr/0003-timezone-handling.md) — alternatives considered, consequences
- **Scope**: [proposal.md](./proposal.md) — impact analysis, affected specs
- **Tasks**: [tasks.md](./tasks.md) — 8 tasks, 4 phases
- **Specs**: [today-providers/spec.md](./specs/today-providers/spec.md), [todo-web/spec.md](./specs/todo-web/spec.md) — AMEND scenarios
- **Prior design**: [phase-3-today/design.md](../archive/2026-06-11-phase-3-today/design.md) — Today view architecture
