# Exploration: v3.x-calendar — Calendar Domain End-to-End

## 1. What We Already Know

### Locked-In Decisions from ADR-0001 (spine) and ADR-0002 (sequencing)

These are settled and MUST NOT be re-litigated:

- **Calendar scope** (ADR-0001 §6 / ADR-0002 "What stays"): "Controlling the daily routine." Simple recurrence (daily/weekly/monthly). No RRULE. Single system timezone (now `KeyUserTimezone`). iCal (.ics) export for Google/Apple calendar interop.
- **Provider registry** (ADR-0002 Change 4): Calendar plugs in via `todaySvc.Register(providers.NewCalendarProvider(db))` — same pattern as Finance. No changes to `today.go`, `sections.go`, or the registry structure are needed beyond adding two new section keys.
- **No daemon, no notifications** (ADR-0001 §5): No background scheduler. Recurrence is metadata-only.
- **Single DB, shared tags/categories**: `calendar_events` + `calendar_tags` + `calendar_fts` follow the exact pattern of `finance_transactions` + `finance_tags` + `finance_fts`.
- **Forward-only migrations**: New file `internal/migrations/202606XXXXXXXX_calendar.sql`.
- **iCal export**: Confirmed in ADR-0001 §Calendar scope and ADR-0002 §Change 9 ("calendar=iCal (v3.x)"). This is Calendar's unique export capability (Finance has CSV).
- **TUI placeholder exists**: `areaCalendar` is defined in `internal/tui/app.go:35`. `View()` renders `placeholderView("Calendar (coming soon — v3.x)", …)`. Key `5` and Tab/Shift-Tab reach it. No `updateCalendar()` method exists yet.
- **Web sidebar gap**: No `/calendar` entry in `layout.html`. Sidebar currently has: Today, Resources, Todos, Finance, Trash. Calendar goes between Finance and Trash.

### Pattern Established by Finance (the Direct Template)

The Finance domain is the structural template. Calendar mirrors it at every layer:

| Layer | Finance path | Calendar equivalent |
|-------|-------------|---------------------|
| Domain types | `internal/finance/` domain | `internal/calendar/event.go` |
| Attacher | `internal/finance/attacher.go` | `internal/calendar/attacher.go` |
| Service | `internal/finance/service.go` | `internal/calendar/service.go` |
| Service tests | `internal/finance/service_test.go` | `internal/calendar/service_test.go` |
| Migration | `internal/migrations/20260613000000_finance.sql` | `internal/migrations/202606XXXXXXXX_calendar.sql` |
| sqlc queries | `internal/store/queries/finance.sql` | `internal/store/queries/calendar.sql` |
| Dynamic filter | `ListFinanceFiltered` in `store/list.go` | `ListCalendarFiltered` in `store/list.go` |
| CLI | `internal/cli/finance*.go` | `internal/cli/calendar*.go` |
| TUI | `internal/tui/finance.go` | `internal/tui/calendar.go` |
| Web handlers | `internal/web/finance.go` | `internal/web/calendar.go` |
| Web template | `internal/web/templates/finance.html` | `internal/web/templates/calendar.html` |
| Today provider | `internal/today/providers/finance.go` | `internal/today/providers/calendar.go` |
| Provider tests | `internal/today/providers/finance_test.go` | `internal/today/providers/calendar_test.go` |

**Structural delta from Finance**: Calendar has genuinely new field types — `start_at` as a full datetime, `end_at` (nullable), `all_day` boolean, and `location`. This adds ~20-30% implementation complexity over Finance but does not change the architectural pattern.

---

## 2. Question Analysis

### Q1: Minimal Viable Calendar Domain

**What maps 1:1 to the finance/todos pattern:**
- `id`, `title`, `description`, `notes`, `category_id`, `created_at`, `updated_at`, `deleted_at` — identical
- `recurrence` enum — identical to todos and finance
- Tags via `calendar_tags` junction + `domain.WithTags` + `Attacher` — identical
- FTS5 via `calendar_fts` + sync triggers — identical
- Service lifecycle: Create/Get/Update/SoftDelete/Restore/Purge/List/Export — identical
- 4-surface delivery: service, CLI, TUI, web — identical
- CalendarProvider → `today.Service.Register()` — identical pattern

**What is genuinely new vs Finance/Todos:**
- `start_at` — a datetime, not just a date like `due_date` or `date`
- `end_at` — events have duration/end time; todos/finance do not
- `all_day` boolean — changes the semantics of `start_at` storage
- `location` — no equivalent in todos/finance
- iCal export (RFC 5545) — `arsenal calendar export --format ical`
- CalendarProvider sections: "Today's Events" + "Upcoming Events"
- Timezone interplay: `start_at` needs `UserLocation` helper for "what events are today?"

**Estimated effort delta over Finance**: ~20-30% more, driven by datetime handling, duration/end_at edge cases, and iCal output.

### Q2: Relationship Between Calendar Events and Todo Due Dates

| Option | Description | Verdict |
|--------|-------------|---------|
| **A — Independent events table** | `calendar_events` owns its own schema; todos remain independent; Today view shows both via separate providers | **Recommended** |
| B — Calendar as aggregator | No new table; CalendarProvider reads todos + finance recurring items | Underdelivers ADR spec; no standalone events, no iCal |
| C — Hybrid: events + surface todo due dates in CalendarProvider | CalendarProvider reads both `calendar_events` and `todos` | Cross-domain provider dependency; duplication with TodosProvider |

**Recommendation: Option A.** A user sees "Today's Events" (CalendarProvider) and "Due Today" (TodosProvider) as two distinct sections — clear and non-redundant.

### Q3: Recurrence — Placeholder vs On-Read Expansion

| Option | Description | Verdict |
|--------|-------------|---------|
| **A — Metadata placeholder** | Match todos/finance: `recurrence` stores intent, no expansion | **Recommended for v3.x** |
| B — Provider-level expansion | CalendarProvider expands recurring events into virtual occurrences for a ±7-day window | Defined upgrade path; no schema changes needed later |
| C — Materialize to rows on create | Generate N rows with a `parent_event_id` FK | Rejected by ADR-0001 and ADR-0002 |

**Recommendation: Option A for v3.x.** Option B can be layered onto CalendarProvider later without touching the schema.

### Q4: Timezone Handling for Events with start_at

`UserLocation(ctx, db)` in `internal/today/user_location.go` returns `*time.Location`; FinanceProvider already uses it for month boundaries. `start_at` is stored as a local-time string without tz offset (e.g. `2026-06-15T09:00:00`), per the single-timezone assumption. CalendarProvider computes "today" in the user's tz, then compares `start_at` strings (branching on `all_day`). No new infrastructure needed. Spec must document that changing `KeyUserTimezone` reinterprets historical `start_at` values.

### Q5: CalendarProvider — Today View Sections

| Section key | Title | Content |
|------------|-------|---------|
| `events-today` | Today's Events | Events where `start_at` falls within today (user's tz); all-day events with `start_at` = today |
| `events-upcoming` | Upcoming Events | Events where `start_at` falls within today+1 to today+7 |

Section ordering in `internal/today/sections.go`: `events-today: 7`, `events-upcoming: 8` (after finance 5/6). `showAllURLFor` extended with 2 cases. Item `Subtitle` formats the event time ("09:00–10:30" or "All day" + optional location).

### Q6: Confirmed Surfaces and Registry Integration

All 4 surfaces follow the Finance template exactly. No registry structural changes are required — only `sections.go` (2 new keys) and `today.go::showAllURLFor` (2 new cases).

---

## 3. Affected Areas

**New files (14)**: `internal/calendar/{event,attacher,service}.go` + `service_test.go` + `migration_test.go`; `internal/store/queries/calendar.sql`; `internal/migrations/202606XXXXXXXX_calendar.sql`; `internal/today/providers/calendar.go` + `calendar_test.go`; `internal/tui/calendar.go`; `internal/web/calendar.go`; `internal/web/templates/calendar.html`; `internal/cli/calendar*.go` + tests.

**Modified files (10)**: `internal/store/queries/tags.sql` (UNION `calendar_tags`); `internal/store/list.go` (`ListCalendarFiltered`); `internal/tui/app.go` (wire `areaCalendar`); `internal/tui/status.go` (hints); `internal/web/handlers.go` (routes + `CalendarCount` + provider); `internal/web/server.go` (`h.calendarRoutes`); `internal/web/templates/layout.html` (sidebar + nav); `internal/today/sections.go` (2 keys); `internal/today/today.go` (`showAllURLFor`); `internal/cli/root.go` (`newCalendarCmd()`).

---

## 4. Open Product Decisions (Proposal Question Round)

| # | Question | Options | Recommendation |
|---|----------|---------|----------------|
| PQ1 | Event end time — `end_at`, `duration_minutes`, or both? | (a) `end_at` only; (b) `duration_minutes` only; (c) both | **(a) `end_at` only** — natural input, direct iCal DTEND map, no ambiguity |
| PQ2 | Recurrence — placeholder or CalendarProvider on-read expansion? | (a) placeholder; (b) expansion | **(a) for v3.x** — consistent with todos/finance; (b) is a non-breaking future add |
| PQ3 | Recurrence enum — include `yearly`? | (a) no (match todos/finance); (b) yes (ADR-0001 Calendar scope) | **(b) add yearly** — ADR-0001 explicit; birthdays/anniversaries are real use cases |
| PQ4 | All-day `start_at` — date-only or midnight datetime? | (a) `'2026-06-15'`; (b) `'2026-06-15T00:00:00'` | **(a) date-only** — consistent with `due_date` in todos; avoids midnight ambiguity |
| PQ5 | iCal export scope — events only, or include todos as VTODO? | (a) events only; (b) events + todos | **(a) events only** — VTODO complexity not worth it; domain boundary stays clean |
| PQ6 | FTS5 columns — `title,description` or `title,description,location`? | (a) 2 cols; (b) 3 cols | **(b) 3 cols** — `location` is a natural calendar search target |

---

## 5. Technical Decisions Locked (No User Input Needed)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| T1 — Package shape | `internal/calendar/` with `event.go`, `attacher.go`, `service.go` | Mirrors Finance |
| T2 — Dynamic filter | `ListCalendarFiltered` hand-written in `store/list.go` | Matches `ListFinanceFiltered` |
| T3 — FTS5 IF NOT EXISTS | Omit guard (unsupported for VIRTUAL TABLE) | Same SQLite limitation as Finance |
| T4 — Timezone | `today.UserLocation(ctx, db)` in CalendarProvider | Already implemented; used by FinanceProvider |
| T5 — Provider registration | `todaySvc.Register(...)` in `handlers.go::newHandlers()` and `app.go::New()` | Identical to FinanceProvider wiring |
| T6 — Orphan tag cleanup | Extend UNION in `tags.sql` with `calendar_tags` | One-line SQL change |
| T7 — Sidebar count | `SELECT COUNT(*) FROM calendar_events WHERE deleted_at IS NULL` | Identical to Finance count pattern |
| T8 — iCal export | stdlib only; VCALENDAR + VEVENT blocks; map `recurrence` → RRULE | No external library |

---

## 6. Scope Forecast

**Estimated LOC**: 1600–2400 across 14 new + 10 modified files. Comparable to Finance with ~20-30% overhead from datetime handling and iCal. Chained PRs required.

**Recommended PR split** (mirrors Finance): (1) migration + service + attacher + tests; (2) CLI + iCal export; (3) TUI; (4) Web; (5) Provider + Today integration.

---

## 7. Risks

| Risk | Severity | Mitigation |
|------|----------|-----------|
| `start_at` as local-time string without tz offset | Medium | Spec documents the single-timezone assumption |
| NULL `end_at` edge cases in CalendarProvider | Medium | Table-driven tests covering null end_at paths |
| iCal RRULE translation per recurrence value | Low | Mechanical mapping; tested per value |
| 1600–2400 LOC requires chained PRs | High | `sdd-tasks` produces PR slices |
| Changing `KeyUserTimezone` reinterprets historical `start_at` | Low | Document in spec + migration comment |

---

## 8. Ready for Proposal

**Yes** — pending user answers to PQ1–PQ6. All have concrete recommendations.
