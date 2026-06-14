# Design: v3.x-calendar — Calendar Domain End-to-End

## Architecture Overview

Calendar is the second new domain on the v3.x spine ([ADR-0002](../../docs/adr/0002-v3-replan.md) Change 1), the structural sibling of Finance. It validates nothing new architecturally — it reuses the proven domain pattern that Finance just shipped: a `Service` wrapping sqlc-generated queries, an `Attacher` implementing [`domain.Attacher`](../../internal/domain/with_tags.go), domain types for enums/inputs, a hand-written dynamic filter in [`store/list.go`](../../internal/store/list.go), an FTS5 virtual table with sync triggers, and a `today.Provider`. The only genuinely new surface area over Finance is in the **data semantics**: a datetime `start_at`, a nullable `end_at`, an `all_day` boolean, a `location` column, and an **iCal (RFC 5545) export** that replaces Finance's CSV export.

`CalendarProvider` registers into [`today.Service`](../../internal/today/today.go) alongside `TodosProvider`, `ResourcesProvider`, and `FinanceProvider`, contributing two sections (`events-today`, `events-upcoming`). The [`sectionOrder`](../../internal/today/sections.go) map extends with keys `7` and `8` (after finance's `5/6`); [`showAllURLFor`](../../internal/today/today.go) extends with two cases. The [`DeleteOrphanTags`](../../internal/store/queries/tags.sql) UNION extends to cover `calendar_tags`.

All three surfaces (CLI, TUI, web) are thin adapters over `calendar.Service`. The TUI replaces the `areaCalendar` placeholder in [`app.go`](../../internal/tui/app.go). The web adds `/calendar` routes and a sidebar count badge via a lightweight `COUNT(*)` in `commonPage()`.

All 6 locked product decisions (PQ1–PQ6 from [proposal.md](./proposal.md)) and the 8 technical decisions (T1–T8 from [explore.md](./explore.md)) are reflected below.

## Architecture Decisions

| Decision | Choice | Alternative | Rationale |
|----------|--------|-------------|-----------|
| Domain package shape | Mirror `internal/finance/` exactly: `event.go` (domain types), `attacher.go`, `service.go` | Extract shared domain base | ~40-line attacher is minimal overhead; the finance precedent is proven; no refactor justified for a 2nd consumer |
| `start_at` storage | Single TEXT column; `YYYY-MM-DDTHH:MM:SS` when timed, `YYYY-MM-DD` when `all_day=1` (PQ4) | Two columns (date + time) | One column maps cleanly to iCal DTSTART value type (DATE vs DATE-TIME); branch on `all_day` |
| `end_at` semantics | TEXT nullable; NULL = open-ended; maps to iCal DTEND (PQ1) | `duration_minutes`; both | Natural user input, direct DTEND map, no ambiguity |
| Recurrence | Metadata-only enum `none/daily/weekly/monthly/yearly`, no expansion (PQ2, PQ3) | Provider on-read expansion; materialize rows | Consistent with todos/finance; expansion is a non-breaking future add; `yearly` per ADR-0001 (birthdays/anniversaries) |
| FTS5 columns | `title, description, location` (3 cols) (PQ6) | 2 cols (`title, description`) | `location` is a natural calendar search target |
| iCal export | stdlib only; VCALENDAR + VEVENT; `recurrence`→RRULE; events only (PQ5, T8) | External ics library; include VTODO | No new dependency; VTODO complexity not worth it; domain boundary stays clean |
| Dynamic filter | Hand-written `ListCalendarFiltered` in `store/list.go` | sqlc-generated dynamic query | Matches `ListFinanceFiltered`; sqlc cannot generate dynamic WHERE |
| Orphan tag cleanup | Extend UNION in `DeleteOrphanTags` with `calendar_tags` (T6) | Shared `domain/orphan_tags.go` | One-line SQL change; no Go refactor for a 4th domain |
| Timezone | `today.UserLocation(ctx, db)` in `CalendarProvider`; `start_at` stored as local-time string without offset (T4, ADR-0003) | Store UTC + offset | Single-system-timezone assumption; reuses helper already used by FinanceProvider; spec documents reinterpretation risk |
| FTS5 `IF NOT EXISTS` | Omit guard on `CREATE VIRTUAL TABLE` (T3) | Wrap in try/catch | SQLite does not support it for virtual tables; goose runs once |
| Section ordering | Calendar after finance: `events-today: 7`, `events-upcoming: 8` | Interleave by recency | Matches `today-view` delta; preserves "actionable → informational" flow |

## Implementation Summary

- **Lines of code**: ~1600–2400 (source ~1500, tests ~700, generated ~200)
- **New packages**: `internal/calendar/` (domain, service, attacher, tests)
- **New CLI package extensions**: `internal/cli/calendar*.go` (7 files)
- **New TUI model**: `internal/tui/calendar.go`
- **New web handlers**: `internal/web/calendar.go` + `calendar.html` template
- **New provider**: `internal/today/providers/calendar.go` + tests
- **New migration**: `internal/migrations/20260614HHMMSS_calendar.sql`
- **Modified files**: ~10 (tui/app.go, web/handlers.go, cli/root.go, today/sections.go, etc.)
- **New spec files**: 7 full capability specs + 2 amended specs

## Decisions & Tradeoffs

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Domain package shape | Mirror `internal/finance/` exactly | Proven pattern; minimal attacher overhead |
| FTS5 columns | `title`, `description`, `location` | Natural search targets; `location` is domain-specific |
| Datetime storage | Single TEXT column, branching on `all_day` | Maps 1:1 to iCal DTSTART value types |
| iCal export scope | Events only (VEVENT); no VTODO | No new dependency; VTODO not worth the complexity |
| Orphan tag cleanup | Extend UNION in `DeleteOrphanTags` | One-line SQL change; no Go refactor for a 4th domain |
| Timezone model | Local-time strings without offset (ADR-0003) | Consistent with system-timezone assumption; `KeyUserTimezone` changes reinterpret historical rows |
| Section ordering | Calendar after finance (7, 8) | Preserves actionable → informational flow |

## Testing & Validation

All tests passing (19 packages):
- `go build ./...` ✓
- `go test ./... -race -count=1` ✓
- `go vet ./...` ✓
- `sqlc` drift check ✓

Strict TDD applied throughout:
- Migration tests verify tables, constraints, FTS5 sync, indices
- Service tests cover Create/Update/Delete/Restore/Purge/List/Export scenarios
- iCal tests cover RFC 5545 output, escaping, line folding, value types
- Provider tests cover section logic, timezone handling, error degradation
- TUI tests cover keybindings, state transitions, detail view
- Web tests cover routes, sidebar counts, HTMX fragments
- CLI tests cover subcommand execution, flags, ical output

## Cross-References

- **Proposal**: `openspec/changes/archive/2026-06-14-v3.x-calendar/proposal.md`
- **Tasks**: `openspec/changes/archive/2026-06-14-v3.x-calendar/tasks.md`
- **Specs**: `openspec/changes/archive/2026-06-14-v3.x-calendar/specs/`
- **ADR-0001**: `docs/adr/0001-v3-scope.md` (scope, calendar specs)
- **ADR-0002**: `docs/adr/0002-v3-replan.md` (Change 1, Change 4, Change 9)
- **ADR-0003**: `docs/adr/0003-timezone-handling.md` (timezone storage)
- **Finance precedent**: `openspec/changes/archive/2026-06-14-v3.x-finance/design.md`
