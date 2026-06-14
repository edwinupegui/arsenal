# Tasks: v3.x-calendar — Calendar Domain End-to-End

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1600–2400 (source ~1500, tests ~700, generated ~200) |
| 400-line budget risk | High (4–6× over) |
| Chained PRs recommended | Yes |
| Suggested split | 5 PRs (Phases 1–3 / 4–5 / 6 / 7 / 8) |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending user decision |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Migration + sqlc + domain + service skeleton + attacher + filter/search/export data | PR 1 | Schema locks here; ~900 lines with tests; foundation for all other PRs |
| 2 | iCal writer + cross-domain tag cleanup | PR 2 | ~350 lines; depends on PR 1; iCal is self-contained |
| 3 | CLI (all subcommands + export + completions) | PR 3 | ~700 lines; depends on PR 1–2; first user-visible terminal surface |
| 4 | TUI sub-model + app wiring | PR 4 | ~380 lines; depends on PR 1; replaces placeholder |
| 5 | Web handlers + templates + sidebar + Provider + Today wiring | PR 5 | ~700 lines; depends on PR 1–2; closes all surfaces |

---

## Phase 1: Migration & Schema

- [x] 1.1 Write `internal/migrations/<timestamp>_calendar.sql` — `calendar_events` (13 cols, CHECK all_day 0/1, CHECK recurrence enum incl. yearly), `calendar_tags` junction, `calendar_fts` FTS5 on title/description/location, 3 indices, `updated_at` trigger, 3 FTS sync triggers. `IF NOT EXISTS` on tables/indices/triggers; omit on FTS virtual table (SQLite limitation). Include timezone storage comment. Down section with DROP in reverse order.
  - Files: `internal/migrations/<timestamp>_calendar.sql` (NEW)
  - Depends: —
  - Acceptance: `goose up` on fresh DB; CHECK rejects `all_day=2`, `recurrence='biweekly'`; nullable `end_at` inserts clean; FTS sync fires on insert/update/delete; 3 indices exist; timezone comment present. `go test ./... -race -count=1` green.
  - Tests: `internal/calendar/migration_test.go` (~70 LOC) — `newTestDB(t)`, assert schema, constraints (all_day, recurrence, nullable end_at, date-only start_at), FTS sync (insert/update/delete/location), indices exist, re-run table stmts safe. Covers: calendar-migration (all 14 scenarios).

## Phase 2: sqlc Queries & Store

- [x] 2.1 Write `internal/store/queries/calendar.sql` — 14 named queries: `CreateCalendarEvent`, `GetCalendarEvent`, `ListCalendarEvents`, `ListTrashedCalendarEvents`, `UpdateCalendarEvent`, `SoftDeleteCalendarEvent`, `RestoreCalendarEvent`, `PurgeCalendarEvent`, `CountCalendarEvents`, `ListEventsToday`, `ListEventsUpcoming`, `ListTagsForCalendar`, `AttachTagToCalendar`, `DetachAllTagsFromCalendar`. Add 5 sqlc.yaml overrides for `deleted_at`, `description`, `end_at`, `notes` (NullString), `category_id` (NullInt64).
  - Files: `internal/store/queries/calendar.sql` (NEW), `sqlc.yaml` (MOD +5 overrides)
  - Depends: 1.1
  - Acceptance: SQL parses correctly; sqlc.yaml override annotations valid. Covers: calendar-service (query contract).

- [x] 2.2 Run `sqlc generate` → `internal/store/calendar.sql.go`, updated `models.go`, `querier.go`. Add hand-written `ListCalendarFiltered` + `CalendarListFilter` + `ListedCalendar` to `internal/store/list.go` (mirrors `ListFinanceFiltered`). Add hand-written `SearchCalendar` to `internal/store/search.go`.
  - Files: `internal/store/calendar.sql.go` (GEN), `internal/store/models.go` (GEN), `internal/store/querier.go` (GEN), `internal/store/list.go` (MOD ~+70), `internal/store/search.go` (MOD ~+35)
  - Depends: 2.1
  - Acceptance: `go build ./internal/store/...` clean; `make sqlc` no drift. Covers: calendar-service (List filter, FTS5 search).

## Phase 3: Service & Domain Package

- [x] 3.1 Create `internal/calendar/event.go` — `Recurrence` enum (none/daily/weekly/monthly/yearly) with `Valid()`, `String()`, `AllRecurrences()`; `CreateInput`, `Filter`, `ExportRow` structs; `validateCreate` (title required, recurrence valid, start_at parseable, all_day/format agreement, EndAt >= StartAt); `nullableString`, `nullableInt64`, `boolToInt` helpers.
  - Files: `internal/calendar/event.go` (NEW ~90 LOC)
  - Depends: 2.2
  - Acceptance: `go build ./internal/calendar/...` clean. Covers: calendar-service (Event domain type, validateCreate invariants).

- [x] 3.2 RED: Write `internal/calendar/service_test.go` — table-driven tests using `newTestDB(t)`: Create timed event, Create all-day event, Create open-ended (NULL end_at), reject invalid recurrence, reject all_day+datetime start_at mismatch, reject empty title, Update changes start_at/end_at, Update clears end_at to NULL, Update changes tags, Update non-existent fails, SoftDelete sets deleted_at, SoftDelete idempotent, Restore clears deleted_at, Restore idempotent, Purge hard-deletes + FTS entry removed, List filter by date range, List all-day only, List trashed, List by tag, Export returns all (no truncation), Export excludes trashed, Attacher creates junction rows, start_at stored without tz offset.
  - Files: `internal/calendar/service_test.go` (NEW ~650 LOC)
  - Depends: 3.1
  - Acceptance: `go test ./internal/calendar/...` FAILS (compile errors — service not yet written). Covers: calendar-service (all 22 scenarios), calendar-migration (constraint scenarios via service test).

- [x] 3.3 GREEN: Create `internal/calendar/attacher.go` (~40 LOC) — mirrors `finance/attacher.go`; `OwnerKind="calendar"`; `AttachTagToOwner` calls `AttachTagToCalendar(event_id, tag_id)`; `DeleteOrphanTags` via shared query. Create `internal/calendar/service.go` (~240 LOC) — `Service{db, q, now}`, `New`, `Create` (validate, tx, insert, attach tags PruneOrphans:false), `Get`, `Update` (detach-all + re-attach PruneOrphans:true), `SoftDelete`, `Restore`, `Purge` (WithTx + cascade). Add `List` (delegates `ListCalendarFiltered` or `SearchCalendar`) and `Export` (resolves category names + tags → ExportRow).
  - Files: `internal/calendar/attacher.go` (NEW ~40 LOC), `internal/calendar/service.go` (NEW ~240 LOC)
  - Depends: 3.2
  - Acceptance: `go test ./internal/calendar/... -race -count=1` PASSES (all service tests green). Covers: calendar-service (all scenarios).

- [x] 3.4 REFACTOR: Verify service compiles clean; verify orphan cleanup covers `calendar_tags` (attacher calls shared `DeleteOrphanTags`). Run `go test ./... -race -count=1` — no regressions in resources/todos/finance.
  - Files: — (verification only)
  - Depends: 3.3
  - Acceptance: All existing tests green; orphan cleanup verified across all 4 domains. Covers: calendar-service (REQ: Attacher for domain.WithTags).

## Phase 4: iCal Writer

- [x] 4.1 RED: Write `internal/calendar/ical_test.go` — table-driven tests: VCALENDAR envelope (BEGIN/END/VERSION/PRODID), VEVENT required fields (UID, SUMMARY, DTSTART, DTSTAMP), DTSTART timed format `20260615T090000`, DTEND timed format, DTSTART;VALUE=DATE all-day `20260615`, DTEND;VALUE=DATE all-day, DTEND omitted when end_at empty, DESCRIPTION included/omitted, LOCATION included/omitted, RRULE per recurrence value (daily/weekly/monthly/yearly), no RRULE for `none`, empty export produces valid envelope, RFC 5545 text escaping (backslash, semicolon, comma, newline), line folding at 75 octets (CRLF + space), all lines end with CRLF, stdlib-only (no external dep).
  - Files: `internal/calendar/ical_test.go` (NEW ~200 LOC)
  - Depends: 3.1
  - Acceptance: `go test ./internal/calendar/... -run TestICal` FAILS (ical.go not yet written). Covers: calendar-ical-export (all 17 scenarios).

- [x] 4.2 GREEN: Create `internal/calendar/ical.go` (~150 LOC) — `WriteICal(w io.Writer, rows []ExportRow) error`; `formatICalDateTime(startAt string, allDay bool) string`; `mapRRULE(r Recurrence) string`; `escapeText(s string) string`; `foldLine(line string) string`. CRLF line endings. DTSTART;VALUE=DATE for all-day; floating local datetime for timed. DTSTAMP from `created_at`. RRULE mapping table. CATEGORIES from category + tags (escaped, comma-joined). Empty rows → valid empty VCALENDAR.
  - Files: `internal/calendar/ical.go` (NEW ~150 LOC)
  - Depends: 4.1
  - Acceptance: `go test ./internal/calendar/... -race -count=1` PASSES including iCal tests. Covers: calendar-ical-export (all scenarios).

## Phase 5: Cross-Domain Tag Cleanup

- [x] 5.1 Extend `DeleteOrphanTags` UNION in `internal/store/queries/tags.sql` — add `UNION SELECT DISTINCT tag_id FROM calendar_tags`. Run `sqlc generate` to regenerate `tags.sql.go`.
  - Files: `internal/store/queries/tags.sql` (MOD +2), `internal/store/tags.sql.go` (GEN)
  - Depends: 2.2
  - Acceptance: `sqlc generate` clean; `go build ./internal/store/...` clean. Covers: calendar-service (REQ: Attacher — orphan cleanup across 4 domains).

- [x] 5.2 Verify orphan cleanup in tests: add `Purge prunes calendar orphans; finance/todo/resource tags untouched` case to `service_test.go`. Run `go test ./... -race -count=1` — all green.
  - Files: `internal/calendar/service_test.go` (MOD ~+30 LOC)
  - Depends: 5.1, 3.3
  - Acceptance: `go test ./... -race -count=1` PASSES; purge removes only calendar orphans. Covers: calendar-service (Purge hard-deletes row, cross-domain isolation).

## Phase 6: CLI

- [x] 6.1 Create `internal/cli/calendar.go` (~25 LOC) — `newCalendarCmd()` parent, help, subcommand registration. Create `internal/cli/calendar_add.go` (~110 LOC) — `add` subcommand, flags: `--title`, `--start`, `--end`, `--all-day`, `--location`, `--cat`, `--tag` (repeatable), `--description`, `--notes`, `--recurrence`. Normalizes `start_at`/`end_at` from `YYYY-MM-DDTHH:MM` to storage format; infers/validates `all_day`. `--json` flag.
  - Files: `internal/cli/calendar.go` (NEW ~25 LOC), `internal/cli/calendar_add.go` (NEW ~110 LOC)
  - Depends: 3.3
  - Acceptance: `arsenal calendar add` creates event; `--all-day` stores date-only; invalid recurrence exits non-zero; missing `--title` exits non-zero; missing `--start` exits non-zero. Covers: calendar-cli (add scenarios).

- [x] 6.2 Create `internal/cli/calendar_list.go` (~110 LOC) — `list` with flags `--from`, `--to`, `--all-day`, `--recurrence`, `--cat`, `--tag`, `--trashed`, `--json`. Create `internal/cli/calendar_show.go` (~80 LOC) — `show <id>` with all fields including all_day/location; non-existent exits non-zero. `--json` flag.
  - Files: `internal/cli/calendar_list.go` (NEW ~110 LOC), `internal/cli/calendar_show.go` (NEW ~80 LOC)
  - Depends: 6.1
  - Acceptance: `list --from/--to` returns filtered events; `list --json` valid JSON array; `list --trashed` returns only deleted; `show` prints all fields; `show 9999` exits non-zero. Covers: calendar-cli (list, show scenarios).

- [x] 6.3 Create `internal/cli/calendar_edit.go` (~110 LOC) — `edit <id>` same flags as add. Create `internal/cli/calendar_rm_restore_purge.go` (~160 LOC) — `rm <id>` (soft-delete), `restore <id>`, `purge <id>` (requires `--yes` or TTY).
  - Files: `internal/cli/calendar_edit.go` (NEW ~110 LOC), `internal/cli/calendar_rm_restore_purge.go` (NEW ~160 LOC)
  - Depends: 6.2
  - Acceptance: `rm <id>` sets deleted_at, event absent from default list; `purge` without `--yes` on non-TTY exits non-zero with error. Covers: calendar-cli (edit, rm, restore, purge scenarios).

- [x] 6.4 Create `internal/cli/calendar_export.go` (~110 LOC) — `export --format ical` (only format in v3.x; `--format csv` exits with error); filter flags `--from`, `--to`, `--cat`, `--tag`; `--output path` writes file, stdout by default. Calls `Service.Export()` then `WriteICal()`. Register `newCalendarCmd()` in `internal/cli/root.go` (+3 lines) after `newFinanceCmd()`. Add completions in `internal/cli/completion.go` (+15 lines): recurrence values, `--format ical`, calendar subcommands.
  - Files: `internal/cli/calendar_export.go` (NEW ~110 LOC), `internal/cli/root.go` (MOD +3), `internal/cli/completion.go` (MOD +15)
  - Depends: 6.3, 4.2
  - Acceptance: `export --format ical` outputs `BEGIN:VCALENDAR`; `--output` writes to file, stdout silent; `--format csv` exits non-zero; date range filter works; empty export outputs valid envelope. Covers: calendar-cli (export scenarios), calendar-ical-export (output destination, filter, empty export).

- [x] 6.5 RED: Write `internal/cli/calendar_test.go` — `cmd.ExecuteContext` + capture stdout tests: help lists subcommands, add timed event, add all-day event, add missing title fails, add missing start fails, add invalid recurrence fails, list date range, list JSON, list trashed, show existing, show non-existent fails, purge non-TTY no-yes fails, rm soft-deletes, export stdout ical, export unsupported format fails, export --output file, tab-complete recurrence, tab-complete subcommands.
  - Files: `internal/cli/calendar_test.go` (NEW ~280 LOC)
  - Depends: 6.4
  - Acceptance: `go test ./internal/cli/... -race -count=1` PASSES. Covers: calendar-cli (all 16 scenarios), calendar-ical-export (stdout/file/filter/empty scenarios via CLI).

## Phase 7: TUI

- [ ] 7.1 Create `internal/tui/calendar.go` (~360 LOC) — `calendarItem` (list.Item: `Title()` = event title, `Description()` = formatted start_at + location + recurrence + tag count); `calendarDetailModel` (viewport, renders all fields including all_day indicator, time range or date-only); `calendarViewState` enum (List/Detail/Trash/ConfirmDelete); `updateCalendar()` dispatcher; `loadCalendarCmd()` (calls `svc.List(ctx, Filter{Limit:500})`); keybindings: `n` new, `e` edit, `d` soft-delete, `r` restore (in trash), `x` purge+confirm, `j`/`k` navigate, `enter` detail, `Tab` area switch.
  - Files: `internal/tui/calendar.go` (NEW ~360 LOC)
  - Depends: 3.3
  - Acceptance: Direct `Model.Update()` bubbletea teatest: `j`/`k` navigate; `enter` opens detail; `d` soft-deletes; `r` restores; `x` shows confirm; all-day detail shows "All day"; timed detail shows "HH:MM–HH:MM"; open-ended shows "—" for end; detail shows tags. Covers: calendar-tui (keybinding scenarios, detail view scenarios, list item formatting).

- [ ] 7.2 Wire `areaCalendar` in `internal/tui/app.go` — add fields `calendarService *calendar.Service`, `calendarList list.Model`, `calendarDetail calendarDetailModel`, `calendarConfirm`, `calendarShowTrashed bool`, `calendarState calendarViewState`. In `View()` replace `placeholderView("Calendar (coming soon — v3.x)")` with `a.calendarView()`. In `Update()` add `case areaCalendar: return a.updateCalendar(msg)`. In `loadCurrentAreaCmd()` add `case areaCalendar: return loadCalendarCmd(...)`. Register `providers.NewCalendarProvider(db)` in `New()` after finance registration.
  - Files: `internal/tui/app.go` (MOD ~+20 LOC)
  - Depends: 7.1
  - Acceptance: Placeholder message gone; `key 5` activates Calendar area; Tab cycles to Calendar; list renders events; calendar area shown in status bar with hints `n new · e edit · d del · Tab switch`. Covers: calendar-tui (placeholder gone, key 5, status bar hints).

- [ ] 7.3 Extend `internal/tui/status.go` with Calendar area hints — `n new · e edit · d del · Tab switch` hint string for `areaCalendar`.
  - Files: `internal/tui/status.go` (MOD ~+4 LOC)
  - Depends: 7.2
  - Acceptance: Status bar text matches spec when Calendar area active. Covers: calendar-tui (REQ: Status bar context hints).

## Phase 8: Web + Provider + Today Integration

- [ ] 8.1 RED: Write `internal/today/providers/calendar_test.go` (~150 LOC) — seed events, `WithCalendarClock`: "events-today" includes timed event today, includes all-day event today, excludes other days, excludes soft-deleted, omitted when empty; "events-upcoming" includes today+1..today+7 events, boundary today+7 included, excluded beyond today+7, omitted when empty; item mapping (Domain, Title, Subtitle timed range, Subtitle all-day, Subtitle no-end, Subtitle + location, URL); timezone `UserLocation` respected; UTC fallback on invalid tz; error degrades gracefully.
  - Files: `internal/today/providers/calendar_test.go` (NEW ~150 LOC)
  - Depends: 3.3
  - Acceptance: `go test ./internal/today/providers/... -run TestCalendar` FAILS (provider not yet written). Covers: calendar-provider (all 14 scenarios), today-providers (REQ-TP-08 all 9 scenarios).

- [ ] 8.2 GREEN: Create `internal/today/providers/calendar.go` (~130 LOC) — `CalendarProvider{queries, db, now}`; `WithCalendarClock(now func() time.Time) CalendarProviderOption`; `NewCalendarProvider(db *sql.DB, opts ...) *CalendarProvider`; `Name() = "calendar"`; `Sections(ctx)` — `today.UserLocation(ctx,db)`; compute `dayStart`, `dayEnd`, `weekEnd`; query `ListEventsToday` + `ListEventsUpcoming`; `mapCalendarItem` (subtitle: all-day→"All day", timed→"HH:MM–HH:MM"/"HH:MM", append " · location"); append sections only when non-empty; return error to Registry.
  - Files: `internal/today/providers/calendar.go` (NEW ~130 LOC)
  - Depends: 8.1, 2.2
  - Acceptance: `go test ./internal/today/providers/... -race -count=1` PASSES. Covers: calendar-provider, today-providers REQ-TP-08.

- [ ] 8.3 Wire provider + extend Today ordering — extend `sectionOrder` in `internal/today/sections.go`: add `"events-today": 7`, `"events-upcoming": 8`. Extend `showAllURLFor` in `internal/today/today.go`: add `"events-today"` → `"/calendar?when=today"` and `"events-upcoming"` → `"/calendar?when=upcoming"`.
  - Files: `internal/today/sections.go` (MOD +4 LOC), `internal/today/today.go` (MOD +4 LOC)
  - Depends: 8.2
  - Acceptance: Today view renders calendar sections after finance sections; empty calendar sections omitted; "show all →" links point to `/calendar?when=today|upcoming`. Covers: today-view (all 7 scenarios: REQ-TV-03, REQ-TV-09).

- [ ] 8.4 Create `internal/web/calendar.go` (~430 LOC) — 9 handlers: `listCalendar` (filter: `?from`, `?to`, `?when=today|upcoming`, recurrence, cat, tag, trashed), `newCalendarForm`, `createCalendar` (compose `start_at`/`end_at` from date+time inputs, all_day from checkbox), `showCalendar` (404 on missing), `editCalendarForm`, `updateCalendar`, `softDeleteCalendar` (HTMX empty fragment), `restoreCalendar` (HTMX card), `purgeCalendar` (redirect). Add `calendarVM` to `internal/web/viewmodel.go` (+30 LOC). Add `calendarService *calendar.Service` to `Handlers` struct. Register routes `h.calendarRoutes(r)` in `internal/web/server.go` after `h.financeRoutes(r)`.
  - Files: `internal/web/calendar.go` (NEW ~430 LOC), `internal/web/viewmodel.go` (MOD +30), `internal/web/server.go` (MOD +3), `internal/web/handlers.go` (MOD ~+20)
  - Depends: 3.3, 4.2
  - Acceptance: All 9 routes return correct status codes; HTMX fragments swap; all-day form accepted; 404 on unknown ID. Covers: calendar-web (list/create/detail/edit/lifecycle route scenarios).

- [ ] 8.5 Create `internal/web/templates/calendar.html` (~240 LOC) — list view (card-based, sorted by start_at, filter controls: date range, recurrence, tag, all-day toggle), show view (all fields; all-day shows date-only badge; timed shows time range; NULL end shown as "—"), create/edit form (title, description, start date + time, end date + time, all-day checkbox, location, category select, tags, recurrence select, notes), HTMX card fragment (soft-delete swap target), empty state ("No events — Add event" link).
  - Files: `internal/web/templates/calendar.html` (NEW ~240 LOC)
  - Depends: 8.4
  - Acceptance: List renders with filter controls; show renders all-day correctly; form accepts all-day; empty state shown on first visit; HTMX delete removes card. Covers: calendar-web (list, show, create all-day, empty state, HTMX scenarios).

- [ ] 8.6 Extend `internal/web/templates/layout.html` — add "Calendar" sidebar link between "Finance" and "Trash" with `{{.CalendarCount}}` badge (hidden when 0); add `<a href="/calendar">Calendar</a>` to header nav. Extend `commonPage()` in `internal/web/handlers.go` — add `CalendarCount int64` computed via `CountCalendarEvents`; register `NewCalendarProvider(db)` in `newHandlers()` after finance registration.
  - Files: `internal/web/templates/layout.html` (MOD +12 LOC), `internal/web/handlers.go` (MOD +8 LOC)
  - Depends: 8.4, 8.2
  - Acceptance: Sidebar shows "Calendar" between Finance and Trash; badge shows count when >0, hidden when 0; `CalendarCount` available on every page. Covers: calendar-web (sidebar badge, CalendarCount, positioning scenarios).

- [ ] 8.7 Write `internal/web/calendar_test.go` (~110 LOC) — `httptest.NewServer` tests: GET /calendar returns 200 + event cards, POST /calendar creates + redirects, GET /calendar/{id} shows detail, POST /calendar/{id}/delete returns HTMX empty fragment, sidebar badge present, GET /calendar/9999 returns 404, all-day create round-trip.
  - Files: `internal/web/calendar_test.go` (NEW ~110 LOC)
  - Depends: 8.5, 8.6
  - Acceptance: `go test ./internal/web/... -race -count=1` PASSES. Covers: calendar-web (all 12 scenarios).

- [ ] 8.8 Final integration verification — `go build ./...` clean; `go test ./... -race -count=1` all green; `go vet ./...` clean; `make sqlc` no drift. Verify Today view section ordering (calendar sections after finance); verify TUI placeholder gone; verify CLI `arsenal calendar` lists all subcommands.
  - Files: — (verification only)
  - Depends: 8.7, 7.3, 6.5
  - Acceptance: Zero build errors; zero test failures; zero regressions in resources/todos/finance/today.

---

## Slice Strategies

Three strategies for splitting into chained PRs. All exceed 400 lines per PR — this is unavoidable with strict TDD requiring tests to ship with code.

### Strategy A: Design-phase split (5 PRs, stacked-to-main) ← Recommended

Mirrors the design's own PR 1–5 split exactly.

| PR | Scope | Est. lines | Review focus |
|----|-------|-----------|--------------|
| 1 | Phases 1–3: migration + sqlc + domain + service + attacher + filter/search/export data | ~900 | Schema correctness, service contract, tag lifecycle, FTS5 |
| 2 | Phases 4–5: iCal writer + cross-domain tag cleanup | ~350 | RFC 5545 correctness, RRULE, escaping, folding |
| 3 | Phase 6: CLI (all subcommands + export + completions) | ~700 | CLI surface, flags, ical output, --format guard |
| 4 | Phase 7: TUI sub-model + app wiring | ~380 | Keybindings, detail view, placeholder removal |
| 5 | Phase 8: Web + Provider + Today wiring | ~700 | Web routes, templates, sidebar, Today sections, section ordering |

- **Pros**: Each PR matches a design phase boundary; PR 2 is genuinely small and reviewable; schema + service locked before any surface work; clear rollback boundary.
- **Cons**: 5 PRs; PR 1 and PR 3 are still large.
- **Working binary at each boundary**: PR 1 → `go test` green, no user-visible surface; PR 2 → iCal writer testable; PR 3 → `arsenal calendar` CLI fully works; PR 4 → TUI Calendar area works; PR 5 → all surfaces complete.

### Strategy B: Layer-first (3 PRs, stacked-to-main)

| PR | Scope | Est. lines | Review focus |
|----|-------|-----------|--------------|
| 1 | Phases 1–3 + 4–5: migration + service + iCal + tag cleanup | ~1250 | Full domain + writer |
| 2 | Phases 6–7: CLI + TUI | ~1080 | Terminal surfaces |
| 3 | Phase 8: Web + Provider + Today | ~700 | Web surface + section wiring |

- **Pros**: Only 3 PRs, less merge overhead.
- **Cons**: PR 1 and PR 2 are very large (3× over budget); reviewers must absorb domain + iCal simultaneously.

### Strategy C: Feature-branch chain (4 PRs, feature-branch-chain)

| PR | Scope | Base branch | Est. lines | Review focus |
|----|-------|-------------|-----------|--------------|
| 1 | Phases 1–3: migration + service foundation | feature/v3.x-calendar | ~900 | Schema + service |
| 2 | Phases 4–5 + 6: iCal + cleanup + CLI | PR 1 branch | ~1050 | iCal + CLI |
| 3 | Phase 7: TUI | PR 2 branch | ~380 | TUI keybindings |
| 4 | Phase 8: Web + Provider + Today | PR 3 branch | ~700 | Web + Today |

- **Pros**: Each child PR diff is focused; rollback boundary per surface.
- **Cons**: Feature-branch-chain requires tracker branch discipline; PR 2 still large; rebasing after PR 1 merges requires care.

### Recommendation

**Strategy A (Design-phase split, stacked-to-main)** is recommended because:
1. PR 2 (iCal, ~350 lines) is genuinely under-budget and easy to review in isolation.
2. The design's own PR 1–5 boundary is pre-validated by the architecture author.
3. Stacked-to-main fits the existing `develop`-based flow.
4. Clear working-binary milestone at each PR boundary.

If the team prefers fewer merges and accepts larger PRs, **Strategy B** is a practical alternative.

---

## Spec Traceability

| Spec | Implementing Phases | Scenarios |
|------|---------------------|-----------|
| calendar-migration | 1 | 14 |
| calendar-service | 2, 3, 5 | 22 |
| calendar-ical-export | 4, 6 | 17 |
| calendar-cli | 6 | 16 |
| calendar-tui | 7 | 17 |
| calendar-web | 8 | 12 |
| calendar-provider | 8 | 14 |
| today-providers (AMEND) | 8 | 9 |
| today-view (AMEND) | 8 | 7 |
| **Total** | | **128** |
