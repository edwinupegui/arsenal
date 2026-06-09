# ADR-0001: Arsenal v3 Scope and Architecture

**Status:** Superseded by [ADR-0002](./0002-v3-replan.md)
**Date:** 2026-06-08
**Date superseded:** 2026-06-08

> The spine of this ADR is still valid: single DB, shared tags/categories, sub-areas UX, no daemon, calendar scope. The **sequencing** and several **technical decisions** (cross-domain FTS5, copy-paste services, deferred TUI architecture) were corrected in ADR-0002 after a critical review.

---

**Original content follows for traceability.**

## Context

Arsenal v2 ships a single domain (resources) accessible from CLI, TUI, and a local web UI. The user wants to evolve Arsenal from a resource manager into a daily-driver personal cockpit.

## Decision

We will extend Arsenal with three new domains — todos, finance, calendar — plus a cross-domain "Today" view. The existing resources domain stays untouched. All four domains share a single SQLite database, a shared tag/category namespace, and a unified sub-area UX in CLI/TUI/web.

## Decisions in detail

### 1. One database, not many
One SQLite file holds resources, todos, finance rows, and calendar events. Trade-offs considered: (a) one DB = single backup with VACUUM INTO, single FTS5 index that crosses domains (search "factura" finds expense + resource + todo), cross-domain transactions, single file to migrate. (b) many DBs = isolated blast radius, smaller per-domain file, no FTS5 cross, multi-file backup. Chosen: one DB. The user explicitly wants "centralized" and personal scale (<10k rows) makes size irrelevant.

### 2. Shared tags and categories across all domains
The tags and categories tables from v2 are reused. A tag like "urgente" can apply to a todo, a resource, an expense. The web sidebar unifies. FTS5 crosses naturally. Trade-off: tag namespace is global; documented convention is lowercase + dedup (already enforced by domain.NormalizeTag).

### 3. Sub-areas in the UI with a cross-domain "Today" view
TUI and web get a top-level menu: Today / Resources / Todos / Finance / Calendar / Trash. Each domain keeps its own list + detail UX (proven in v2 with resources). "Today" is the killer feature: it aggregates overdue todos, due-today, events today, upcoming 7 days, and recent expenses in one screen. This is what the user opens 5x/day.

### 4. Migrations per domain, ordered by timestamp
Each domain owns its schema migrations as separate files: `migrations/20260601000001_init_resources.sql` (existing), `migrations/20260601000002_init_todos.sql`, `migrations/20260601000003_init_finance.sql`, `migrations/20260601000004_init_calendar.sql`, `migrations/20260601000005_today_view.sql`. Goose applies in order. Easy to skip a domain in a future v4 if a feature is dropped (vs. mixing tables across one giant migration).

### 5. No reminders domain, no daemon, no OS notifications in v3
User's flow: "I check todos and calendar constantly" — centralization IS the answer; push isn't needed. Trade-off: if push is wanted later, that's v4+ work (daemon + terminal-notifier/notify-send/BurntToast per OS). v3 stays single-process, no background services, simpler release.

### 6. Calendar's primary purpose: controlling the daily routine
Design for "my day" not "corporate agenda". Simple recurrence (daily/weekly/monthly/yearly), no RRULE. Week + month views, day detail. Single timezone (system). Trade-off: less powerful than Google Cal, but matches the user's actual use case.

## Per-domain scope (v3)

### Todos
Fields: title, description, priority (low/med/high), due_date, status (open/done), simple recurrence (none/daily/weekly/monthly), tags, soft-delete.
Views: today, overdue, this week, all open, by tag.
Service: Create/Update/MarkDone/SoftDelete/Restore/Purge.
Not in scope: nested projects, dependencies, GTD contexts, waiting-for/someday buckets.

### Finance
Fields: date, amount, currency (single config-wide), account (free-text), category, kind (expense/income), notes, tags, soft-delete.
Views: current month, previous month, by category, by account, balance.
Service: Create/Update/Delete + aggregated views.
Export: CSV (more useful than markdown for finance).
Not in scope: multi-currency in same DB, multi-account accounting, budgets, bank sync, investments.

### Calendar
Fields: title, description, start_at, duration_minutes, all_day, recurrence (none/daily/weekly/monthly/yearly), tags, optional link to a resource or todo, soft-delete.
Views: week, month, day detail, upcoming list.
Export: iCal (.ics) for Google/Apple calendar interop.
Service: Create/Update/Delete + simple occurrence expansion (NOT full RRULE).
Not in scope: RRULE-grade recurrence, attendees, timezones other than system, sync with external services.

### Resources (unchanged)
Existing v2 surface. No breaking changes.

## Out of scope for v3 (deferred to v4+)
Notes/journal, code snippets, contacts, books/movies with state, habits/streaks, reminders with push, multi-currency finance, multi-account finance, RRULE-grade recurrence, timezones other than system.

## Consequences

### Positive
- Single backup, single migration path, single FTS5 index that crosses domains.
- "Today" view becomes a real differentiator: see everything that matters in <1s.
- Each new domain follows the v2 pattern (domain types → sqlc queries → service → CLI+TUI+web), so we have a template.
- v2 stays untouched and stable during rollout.

### Negative / risks
- Scope: 4 domains × 3 surfaces × CLI × tests × docs = 5-10× current code. Mitigation: phased rollout, each phase ends with a working binary.
- TUI density: each new domain adds another view. Mitigation: tabbed/sub-area model; Today is the default landing.
- Single global tag namespace: collision possible. Mitigation: tags are already lowercase + deduped by domain.NormalizeTag; documented convention.
- Shared DB means a bug in one domain could affect others. Mitigation: per-domain service layer with explicit transactions; no cross-domain write shortcuts.

## Rollout phases (original)

| Phase | Deliverable | Working binary check |
|---|---|---|
| 0 | This ADR | N/A |
| 1 | Foundations: extract `internal/sqliteutil`, unify Tx helper, extend `config` | `arsenal list` works exactly as v2 |
| 2 | Todos end-to-end (schema + service + CLI + TUI section + web page + tests) | `arsenal todo add "pagar luz" --due 2026-06-10` works on all 3 surfaces |
| 3 | "Today" cross-domain view (TUI + web) | `arsenal today` (or TUI default) shows overdue + today + upcoming + events |
| 4 | Finance end-to-end + CSV export | `arsenal finance add 5000 --cat servicios --kind expense` and CSV export |
| 5 | Calendar end-to-end + iCal export | `arsenal calendar add "rutina mañana" --at 07:00 --duration 30 --recurrence daily`; .ics importable in Google Cal |
| 6 | Polish, unified UX, completions, docs, release v3.0 | `arsenal` opens into a polished v3 with all 4 domains and Today view |

## Open questions (original, to resolve in phase 1)
- Single config file location: SQLite `config` table (preferred for consistency) vs `~/.arsenal/config.toml`. Recommend: SQLite `config` table.
- Default landing surface: TUI vs web. Recommend: configurable; default TUI for terminal-native users.
- Active currency change in finance: refuse the change with a clear error if any row uses a different currency.
