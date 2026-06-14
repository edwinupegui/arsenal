# Proposal: v3.x-calendar — Calendar Domain End-to-End

## Why

Calendar is the second v3.x deferred domain per [ADR-0002 Change 1](../../docs/adr/0002-v3-replan.md), the sibling of Finance. Its purpose, per [ADR-0001 §Calendar scope](../../docs/adr/0001-v3-scope.md) and ADR-0002 "What stays", is **controlling the daily routine**: simple recurring and one-off events, no RRULE, single system timezone, with iCal (.ics) export for Google/Apple Calendar interop. v3.0 shipped Resources + Todos + Today, and Finance has just validated the full domain pattern (service, CLI, TUI area, web surface, Today provider, dedicated migration). Calendar plugs into the same proven spine without architectural rework — one new domain following the exact pattern Finance established.

All 6 product decisions are **locked** (user accepted the proposal question round). Calendar carries genuinely new field types over Finance (`start_at` datetime, nullable `end_at`, `all_day` boolean, `location`) plus iCal export, which adds ~20-30% implementation overhead but does not change the architecture. See [explore.md](./explore.md) for the architecture map, affected files, and technical decisions T1-T8.

Locked product decisions:

| # | Decision | Choice |
|---|---|---|
| PQ1 | Event end time | `end_at` only (nullable; NULL = open-ended; maps to iCal DTEND) |
| PQ2 | Recurrence behavior | Metadata-only placeholder for v3.x (no expansion), same as todos/finance |
| PQ3 | Recurrence enum | Includes `yearly` → `none/daily/weekly/monthly/yearly` (Calendar-specific per ADR-0001) |
| PQ4 | All-day storage | Date-only `start_at` (e.g. `2026-06-15`) when `all_day = 1` |
| PQ5 | iCal export scope | Events only (VEVENT); no VTODO |
| PQ6 | FTS5 columns | `title`, `description`, `location` (3 columns) |

## What Changes

### New Capabilities (7)

| Capability | Scope |
|---|---|
| `calendar-service` | Domain types (`Event`, `Recurrence`, `CreateInput`), `Service` (Create/Get/Update/SoftDelete/Restore/Purge/List/Export), attacher via `domain.WithTags`. Handles `start_at` datetime, nullable `end_at`, `all_day`, `location` |
| `calendar-cli` | `arsenal calendar` subcommand: `add`, `list`, `show`, `edit`, `rm`, `restore`, `purge`, `export` |
| `calendar-tui` | Replace `areaCalendar` placeholder with real sub-model (`updateCalendar`, keybindings, status bar) |
| `calendar-web` | `/calendar` route, sidebar entry with count badge, all CRUD + restore/purge handlers + template |
| `calendar-provider` | `CalendarProvider` implements `today.Provider`, 2 sections: "Today's Events" + "Upcoming Events" |
| `calendar-ical-export` | `arsenal calendar export --format ical` → stdout or `--output`; VCALENDAR + VEVENT blocks, `recurrence` → RRULE mapping, `end_at` → DTEND, all-day → DATE value type. Events only (no VTODO) |
| `calendar-migration` | `202606XXXXXXXX_calendar.sql`: `calendar_events`, `calendar_tags`, `calendar_fts` (FTS5), sync triggers, indices |

### AMEND Capabilities (2)

| Capability | Change |
|---|---|
| `today-providers` | Registry now includes `CalendarProvider`; section keys `events-today` and `events-upcoming` added |
| `today-view` | `sectionOrder` map in `internal/today/sections.go` extended with `events-today: 7` and `events-upcoming: 8` (after finance 5/6); `showAllURLFor` extended with 2 new cases |

> **Note**: The `DeleteOrphanTags` UNION extension in `internal/store/queries/tags.sql` (adding `SELECT DISTINCT tag_id FROM calendar_tags`) is an implementation change, not a spec AMEND. No existing spec covers the attacher's orphan cleanup behavior — same precedent as Finance.

## Impact

### Affected Specs

| Spec | Action |
|---|---|
| `calendar-service` | NEW |
| `calendar-cli` | NEW |
| `calendar-tui` | NEW |
| `calendar-web` | NEW |
| `calendar-provider` | NEW |
| `calendar-ical-export` | NEW |
| `calendar-migration` | NEW |
| `today-providers` | AMEND — registry + section keys |
| `today-view` | AMEND — sectionOrder entries + `showAllURLFor` cases |
| `today-empty-state` | — no change |
| `today-tui` | — no change (Calendar is a separate TUI area, not a Today sub-view) |
| `today-web` | — no change |
| `finance-*` | — no change (independent sibling domain) |
| `todo-*` | — no change |

### New Code Locations

| Path | Purpose |
|---|---|
| `internal/calendar/event.go` | Domain types (`Event`, `Recurrence`, `CreateInput`) |
| `internal/calendar/attacher.go` | `Attacher` for `domain.WithTags` (mirrors `finance/attacher.go`) |
| `internal/calendar/service.go` | `Service` with all lifecycle + export methods |
| `internal/calendar/service_test.go` | Integration tests |
| `internal/calendar/migration_test.go` | Migration table/trigger verification tests |
| `internal/cli/calendar.go` + `calendar_*.go` | CLI subcommand tree (add/list/show/edit/rm/restore/purge/export) |
| `internal/tui/calendar.go` | TUI sub-model for `areaCalendar` |
| `internal/web/calendar.go` | HTTP handlers |
| `internal/web/templates/calendar.html` | Web templates (list, show, form, partials) |
| `internal/today/providers/calendar.go` | `CalendarProvider` |
| `internal/today/providers/calendar_test.go` | Provider tests |
| `internal/store/queries/calendar.sql` | sqlc queries |
| `internal/migrations/202606XXXXXXXX_calendar.sql` | Migration |

### Modified Code Locations

| Path | Change |
|---|---|
| `internal/store/queries/tags.sql` | Extend `DeleteOrphanTags` UNION: add `SELECT DISTINCT tag_id FROM calendar_tags` |
| `internal/store/list.go` | Add `ListCalendarFiltered` (hand-written dynamic filter, mirrors `ListFinanceFiltered`) |
| `internal/tui/app.go` | Wire `areaCalendar` → `updateCalendar()`; replace placeholder; register `CalendarProvider`; keybinding dispatch |
| `internal/tui/status.go` | Calendar context hints in status bar |
| `internal/web/handlers.go` | Register `/calendar/*` routes; extend `commonPage()` with `CalendarCount`; register `CalendarProvider` |
| `internal/web/server.go` | Wire `h.calendarRoutes` |
| `internal/web/templates/layout.html` | Sidebar Calendar entry (between Finance and Trash) with count badge; header nav entry |
| `internal/cli/root.go` | Register `newCalendarCmd()` |
| `internal/cli/completion.go` | Calendar completions |
| `internal/today/sections.go` | Add `events-today: 7` and `events-upcoming: 8` to `sectionOrder` |
| `internal/today/today.go` | Extend `showAllURLFor` with 2 calendar cases |

## Migrations

One new forward-only migration: `internal/migrations/202606XXXXXXXX_calendar.sql` (filename timestamp resolved at write time; disk is source of truth per ADR-0002 Change 8). Creates:

- `calendar_events` — columns: `id`, `title` (TEXT), `description` (TEXT), `start_at` (TEXT — full datetime `YYYY-MM-DDTHH:MM:SS`, or date-only `YYYY-MM-DD` when `all_day = 1`), `end_at` (TEXT, nullable — NULL means open-ended; maps to iCal DTEND), `all_day` (INTEGER, 0/1), `location` (TEXT), `category_id` (FK→categories), `notes`, `recurrence` (CHECK: none/daily/weekly/monthly/yearly), `created_at`, `updated_at`, `deleted_at`
- `calendar_tags` — junction table (FKs to `calendar_events` + `tags`)
- `calendar_fts` — FTS5 virtual table on `title`, `description`, `location`; sync triggers for insert/update/delete
- Indices: `idx_calendar_start`, `idx_calendar_deleted`, `idx_calendar_category`

Forward-only per ADR-0001 migration_policy. This is the **first** calendar migration — zero existing data to lose. Tables and triggers use `IF NOT EXISTS` guards where supported; the FTS5 virtual table omits the guard (unsupported for `VIRTUAL TABLE`, same SQLite limitation as Finance — see explore T3).

> **Timezone note (per [ADR-0003](../../docs/adr/0003-timezone-handling.md))**: `start_at` is stored as a local-time string without timezone offset, consistent with the single-system-timezone assumption. Changing `KeyUserTimezone` reinterprets historical `start_at` values. This is documented in the migration file as a comment and in the `calendar-service` spec.

## Rollout / Rollback

- **Feature flag**: Calendar is opt-in by domain. No other domain depends on calendar tables. TUI renders the existing placeholder until the migration is applied. Web sidebar entry is hidden when no calendar data exists (same pattern as Finance).
- **Rollback**: Revert commits. If the migration was applied but no user data exists, `DROP TABLE IF EXISTS calendar_events, calendar_tags, calendar_fts` is safe. If user data exists, rolling back requires a down-migration (documented in the migration file as a comment). Since this is v3.x fresh — no prior calendar data — the risk is minimal.
- **Cross-domain isolation**: The Today view gracefully degrades when `CalendarProvider.Sections()` returns an error (same pattern as existing provider error handling per `today-view`).

## Scope Warning

**Forecast**: 1600–2400 LOC across 4 surfaces (CLI, TUI, Web, Provider) + service + migration + iCal export + tests. This is **4–6× over the 400-line review budget guard**, with ~20-30% overhead beyond Finance from datetime/`end_at`/`all_day` handling and iCal output. This change WILL require chained PRs.

The orchestrator will present chain-strategy options after `sdd-tasks` produces the official forecast. Recommended split (mirrors Finance): (1) migration + service + attacher + tests; (2) CLI + iCal export; (3) TUI; (4) Web; (5) Provider + Today integration. Each slice is independently testable.

## Open Questions

None. All 6 product decisions (PQ1–PQ6) accepted. No ambiguities found beyond what the exploration already resolved.

## References

- [ADR-0001: v3 scope](../../docs/adr/0001-v3-scope.md) — spine (single DB, shared tags, sub-areas, no daemon); Calendar scope (daily routine, simple recurrence, single tz, iCal export)
- [ADR-0002: v3 replan](../../docs/adr/0002-v3-replan.md) — Change 1 (deferred to v3.x), Change 4 (Provider registry), Change 5 (FTS5 UNION ALL), Change 9 (calendar=iCal export)
- [ADR-0003: timezone handling](../../docs/adr/0003-timezone-handling.md) — single-timezone storage assumption for `start_at`
- [explore.md](./explore.md) — full exploration, 6 product decisions, technical decisions T1-T8, affected files
- [v3.x-finance proposal](../archive/2026-06-14-v3.x-finance/proposal.md) — direct structural template (sibling end-to-end domain)
