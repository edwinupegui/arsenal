# CONTINUE — Arsenal v3 Resume Guide

> **Read this when resuming work on Arsenal.** Last updated: 2026-06-14 (calendar session).
> TL;DR: v3.0.0 shipped all 3 surfaces (TUI, web, CLI). Both deferred v3.x domains — **Finance** and **Calendar** — are now implemented end-to-end across all 4 surfaces (service, CLI, TUI, web) plus the Today view, all green with `-race`. With Calendar done, **all five TUI areas are functional** and the v3.x domain backlog from ADR-0002 is complete.

---

## 1. TL;DR

- **v3.0.0 feature-complete** with all 3 surfaces (TUI, web, CLI) shipped.
- **31 commits ahead of main on `develop`**, all green (15 test packages with `-race`).
- **Working tree clean**. Nothing uncommitted.
- **4 v3.0.1 backlog items done** in this session: TUI n-key form, timezone ADR, ShowAllURL provider relaxation, DueAfter in ListFilter.
- **Only one follow-up remains**: implement the timezone changes per ADR-0003 (~50 LOC, 4 call sites, 6 tests). The decision is documented; the code is a separate task.
- **v3.x Finance — DONE**: `internal/finance/` service, `arsenal finance` CLI (incl. CSV export), TUI `areaFinance`, web `/finance` routes + sidebar badge, and `FinanceProvider` wired into the Today view. Migration `20260613000000_finance.sql`. Archived in `openspec/changes/archive/2026-06-14-v3.x-finance/`.
- **v3.x Calendar — DONE**: `internal/calendar/` service, `arsenal calendar` CLI (incl. iCal/RFC 5545 export), TUI `areaCalendar`, web `/calendar` routes + sidebar badge, and `CalendarProvider` (today's events + upcoming) wired into the Today view. Migration `20260614000000_calendar.sql`. See `openspec/changes/v3.x-calendar/`.
- **Next**: no deferred v3.x domains remain from ADR-0002. Candidate follow-ups: recurrence expansion (todos/finance/calendar share a metadata-only `recurrence` enum with no scheduler), and the `Calendar` web `?when=today|upcoming` filter is currently a display hint (no separate server-side predicate).

---

## 2. Current state

```
Branch:    develop (31 commits ahead of main, working tree clean)
Last commit: 62259e8 docs: drop DueAfter in ListFilter from v3.0.1 backlog
Tests:     15/15 packages green with -race
Build:     go build ./... clean
Vet:       go vet ./... clean
sqlc:      no drift
```

Recent commit history (this session only):

```
62259e8 docs: drop DueAfter in ListFilter from v3.0.1 backlog
68310ca feat(todos): add DueAfter filter to ListFilter for precise date ranges
7c52e2b docs: drop ShowAllURL provider relaxation from v3.0.1 backlog
b1c51fa fix(today): let Service.Build set ShowAllURL by removing provider caps
ae620ae docs(adr): add ADR-0003 for timezone handling
ab4e96e docs: move TUI n-key new-todo form from deferred to shipped in v3.0.0
83d0234 feat(tui): open inline new-todo form with 'n' in areaToday
185b0a4 docs: move arsenal today CLI from deferred to shipped in v3.0.0
bdafd03 feat(cli): add arsenal today command with table and JSON output
6163e70 chore(openspec): archive phase-2-todos and phase-3-today changes
3a1cc0c docs(release): add v3.0.0 release notes
42d7964 docs(changelog): flag arsenal today CLI deferral to v3.0.1
ccc3fb6 Merge branch 'feat/phase-3-today-web' into develop
... (and 18 more from prior sessions)
```

---

## 3. What v3.0.0 ships

### Three surfaces, one engine

The Today view (`internal/today/`) is the cross-domain engine. All 3 surfaces read from the same `today.Service.Build(ctx)`:

| Surface | Entry point | File |
|---------|-------------|------|
| TUI | `areaToday` (default landing) | `internal/tui/today.go` |
| Web | `GET /today` | `internal/web/today.go` + `templates/today.html` |
| CLI | `arsenal today` | `internal/cli/today.go` |

### v3.0.0 capabilities

- **Today cross-domain view**: 4 sections (Overdue, Due Today, Upcoming, Recent Resources), density cap of 5, "show all" links, empty state
- **Provider registry pattern** (ADR-0002 Change 4): independent providers with graceful degradation. v3.0 ships TodosProvider + ResourcesProvider. v3.x adds Finance + Calendar without registry changes.
- **TUI area-switcher**: Tab/Shift+Tab cycle, 1-5 direct jump, default landing is Today
- **TUI inline new-todo form** (added in v3.0.1, ships in v3.0.0): `n` in areaToday opens an inline form, enter saves, esc cancels, default priority `med`
- **Web /today route**: sectioned cards, sidebar "Today" entry with overdue count badge, `hx-swap-oob` for badge refresh
- **CLI `arsenal today`**: table + JSON output, smoke-tested end-to-end
- **Todos end-to-end** (phase 2): CLI, TUI, Web; lifecycle, status, search, tags, soft-delete, restore, purge
- **Shared foundations** (phase 1.5): `domain.WithTags`, `Attacher` interface, typed config catalog
- **Migrations**: `20260608000002_todos.sql` creates todos + tags + FTS5

### Breaking change

- **`KeyLandingSurface` enum values** changed: `["tui", "web"]` → `["today", "resources"]`. Old configs silently fall back to the default. Documented in CHANGELOG and release notes.

### v3.0.0 docs

- `CHANGELOG.md` — Unreleased section has the full change set
- `docs/releases/v3.0.0.md` — release notes for the upcoming tag
- `docs/adr/0001-v3-scope.md` — v3 scope (superseded by 0002)
- `docs/adr/0002-v3-replan.md` — current architecture decision
- `openspec/specs/{todo-*,today-*}/*.md` — 14 canonical specs (9 from phase-2, 5 from phase-3)
- `openspec/changes/archive/2026-06-11-phase-{2-todos,3-today}/` — frozen planning artifacts

---

## 4. What's done in v3.0.1 backlog

| Item | Status | Commits | Notes |
|------|--------|---------|-------|
| TUI tecla `n` con form inline | ✅ Done | `83d0234` + `ab4e96e` | Opens inline form in areaToday, default `med` priority |
| Timezone ADR (ADR-0003) | ✅ Done (ADR only) | `ae620ae` | 158 lines; implementation is a separate task |
| ShowAllURL provider relaxation | ✅ Done | `b1c51fa` + `7c52e2b` | Providers no longer cap; Service.Build caps + sets ShowAllURL |
| DueAfter en ListFilter | ✅ Done | `68310ca` + `62259e8` | Used by Today upcoming section; SQL-side date filtering |

**Known limitations of v3.0.0**: only one remains, and it's the timezone code (decision is made, implementation deferred):

- `date('now')` in SQLite is UTC. Off-by-one-day for non-UTC users. **ADR-0003** has the fix plan.

---

## 5. What's pending

### 5.1 Timezone implementation (only v3.0.1 item left)

**Source of truth**: `docs/adr/0003-timezone-handling.md`

**Summary of the decision** (from the ADR):

- Add `KeyUserTimezone` config key (IANA timezone, e.g., `America/Argentina/Buenos_Aires`).
- Default `UTC` (backwards compatible).
- Validate via `time.LoadLocation(value)`; invalid → silent UTC fallback.
- Storage stays UTC text `YYYY-MM-DD`. The user's timezone only affects "what is today" in date comparisons.
- Replace `time.Now().UTC()` at 3 Go call sites and `date('now')` at 1 SQL call site.

**Files to touch** (per ADR §Implementation plan):

1. `internal/config/keys.go` — add `KeyUserTimezone` key (1 entry, default `"UTC"`)
2. `internal/today/user_location.go` (NEW) — `userLocation(ctx, db) (*time.Location, error)` helper
3. `internal/today/providers/todos.go:28` — replace `time.Now().UTC()` with `time.Now().In(loc)` via helper
4. `internal/todos/service.go:209` — same replacement
5. `internal/web/todos.go:48, 412` — same replacement (2 sites)
6. `internal/web/handlers.go:627` — replace `date('now')` with a Go-bound parameter
7. `internal/today/user_location_test.go` (NEW) — helper tests
8. `internal/configstore/keys_test.go` (MOD) — `KeyUserTimezone` defaults + invalid fallback
9. `internal/todos/service_test.go` (MOD) — test with non-UTC timezone
10. `internal/web/todos_test.go` (MOD) — overdue badge with non-UTC timezone
11. `CHANGELOG.md` — remove the "Timezone handling" line from Known limitations

**Estimated effort**: ~50 LOC code, 1 ADR-driven test set (~6 tests), 1 config key, 1 helper.

**How to start this in a new session**:

```
You: "implementá el timezone change per ADR-0003"
Me: launches sdd-apply with the ADR as spec, ~50 LOC, ~6 tests
```

Or, if a full SDD cycle is overkill for ~50 LOC: read ADR-0003, do strict TDD inline, push.

### 5.2 v3.x deferred domains

Per ADR-0002 Change 1, v3.0 only ships resources + todos + Today view. Finance and Calendar are deferred to v3.x.

**To start either, use SDD**:

```
/sdd-new finance
/sdd-new calendar
```

Both follow the same pattern as the archived changes:
- `sdd-explore` — investigate the idea, ask 3-5 product questions
- `sdd-propose` — write proposal
- `sdd-spec` — write delta specs (one per capability)
- `sdd-design` — architecture
- `sdd-tasks` — break into work units
- `sdd-apply` — implement
- `sdd-verify` — validate
- `sdd-archive` — sync specs to canonical

The Provider registry already supports new domains without changes. The new work is:
- Domain logic (migrations, service, queries) per ADR-0001 §6
- Concrete provider for the new domain (e.g., `FinanceProvider` returning subscriptions + recurring expenses)
- Optionally: web routes, TUI area, CLI commands
- Wire into `today.New(db)` registration

---

## 6. How to resume

### 6.1 Verify state (always first)

```bash
cd /Users/edwindev/Documents/Work/Side\ Projects/EdwinLab/arsenal
git status
git log main..develop --oneline | head -5
go test ./... -count=1 -race
```

Expected: clean working tree, 31 commits ahead, 15/15 packages green.

### 6.2 Release v3.0.0 to main (the merge the user does)

The user opens PRs and tags manually. The orchestrator never uses `gh pr create`.

```bash
# On develop, verify state
git status  # clean
git log main..develop --oneline | wc -l  # 31

# Open the PR (manual, via GitHub UI or gh)
gh pr create --base main --head develop \
  --title "release: v3.0.0 — Today cross-domain view" \
  --body "See docs/releases/v3.0.0.md for the full release notes."

# After PR is approved and merged to main, tag the merge commit
git checkout main
git pull
git tag -a v3.0.0 -m "v3.0.0 — Today cross-domain view"
git push origin v3.0.0

# Create the GitHub release, pasting the body of docs/releases/v3.0.0.md
gh release create v3.0.0 --notes-file docs/releases/v3.0.0.md
```

### 6.3 Implement the timezone changes (per ADR-0003)

1. Read `docs/adr/0003-timezone-handling.md` end-to-end. The decision is locked in; don't re-debate it.
2. Strict TDD: RED test for the helper, then GREEN impl.
3. Touch the 4 call sites one at a time, each with its own test.
4. Update CHANGELOG to remove the "Timezone handling" Known limitation.
5. Commit, push to develop.
6. Update `docs/releases/v3.0.0.md` if you want a v3.0.1 follow-up release (or just keep going on v3.x).

### 6.4 Start v3.x (Finance or Calendar)

1. Use `/sdd-new finance` or `/sdd-new calendar` from the orchestrator.
2. The orchestrator will run `sdd-explore` first to ask 3-5 product questions.
3. Pattern is well-established — see `openspec/changes/archive/2026-06-11-phase-{2-todos,3-today}/` for prior proposals, designs, and tasks as templates.

---

## 7. Critical context for the next session

### 7.1 Engram memory (cross-session)

Engram is the persistent memory system. These topic keys are relevant:

- `arsenal/v3-release-decisions` — umbrella for v3 release-time choices
- `arsenal/v3-cli-shipped` — the `arsenal today` CLI decision and implementation
- `sdd-init/arsenal` — project init envelope (stack, testing, strict TDD)
- `sdd/phase-2-todos/{explore,proposal,spec,design,tasks,apply-progress,archive-report}` — phase 2 artifacts
- `sdd/phase-3-today/{explore,proposal,spec,design,tasks,apply-progress,verify-report,archive-report}` — phase 3 artifacts
- `architecture/auth-model` and other topic keys from prior work

**For a new session, the orchestrator will search engram for context. The above keys give a complete picture.**

### 7.2 Key files (the "you must understand these" list)

| File | Role |
|------|------|
| `internal/today/today.go` | The cross-domain engine. `Service.Build(ctx)` is the single entry point. |
| `internal/today/provider.go` | `Provider` interface + `Registry`. Future domains plug in via `Register()`. |
| `internal/today/providers/todos.go` | TodosProvider: overdue, due-today, upcoming sections |
| `internal/today/providers/resources.go` | ResourcesProvider: recent section |
| `internal/tui/today.go` | TUI areaToday: state machine, form dispatch, view rendering |
| `internal/tui/today_new_form.go` | The inline new-todo form model (v3.0.1) |
| `internal/web/today.go` | Web `GET /today` handler |
| `internal/web/templates/today.html` | Web Today template |
| `internal/cli/today.go` | CLI `arsenal today` command + table/JSON renderers |
| `internal/todos/service.go` | Todos service: Create, List, Update, MarkDone, SoftDelete, etc. |
| `internal/config/keys.go` | Typed config catalog (KeyLandingSurface, KeyUserTimezone TBD) |
| `internal/store/list.go` | SQL builder for `ListTodosFiltered` (the dynamic filter system) |
| `docs/adr/0001-v3-scope.md` | v3 architecture (superseded but spine is still valid) |
| `docs/adr/0002-v3-replan.md` | v3 replan: scope reduction, sequencing, design corrections |
| `docs/adr/0003-timezone-handling.md` | Timezone decision (only v3.0.1 follow-up) |
| `openspec/specs/*.md` | 14 canonical specs (the source of truth for behavior) |

### 7.3 Conventions (do not break these)

- **Conventional commits only**. No "Co-Authored-By" or AI attribution. Examples in git log.
- **Strict TDD** is active. Tests written before implementation. Test runner is `go test ./... -count=1 -race`. Strict mode per `sdd-init/arsenal`.
- **Spanish for direct conversation, English for artifacts**. The user uses Rioplatense Spanish in chat; code, comments, UI strings, and commit messages are English.
- **The user opens PRs manually**. Orchestrator can: merge feature branches to develop, push to origin/develop, commit and push to develop. Cannot: `gh pr create`, push to main, tag releases.
- **No schema migration** without a corresponding file in `internal/migrations/`. Migrations are forward-only.
- **Provider registry is the integration point for new domains**. Don't bypass it. Don't add a special case in `Service.Build` for a specific provider.
- **Density cap is 5** (`const maxItemsPerSection = 5` in `internal/today/today.go`). ShowAllURL mapping is `showAllURLFor(key)` in the same file.

### 7.4 Project quirks worth knowing

- **Date format**: `YYYY-MM-DD` text columns. Comparisons are string-based. `date('now')` is UTC.
- **Soft delete**: every domain table has `deleted_at`; queries filter `WHERE deleted_at IS NULL` by default.
- **Tags are global**, not per-domain. Shared `tags` table. Reused across resources, todos, future domains.
- **`domain.WithTags` helper** in `internal/domain/with_tags.go` is the canonical way to attach tags. Both resources and todos use it (validates the helper).
- **Config catalog is typed** (`configstore.KeyDef` with Type/Default/EnumValues). Adding a new config key means adding it to `internal/config/keys.go`, not reading env vars ad hoc.
- **TUI area enum**: `areaToday, areaResources, areaTodos, areaFinance, areaCalendar`. Default landing is `areaToday` (v3.0.0+).
- **Web sidebar** is rendered by `internal/web/handlers.go::commonPage()` on every page. Today badge is computed here.

---

## 8. Open follow-ups (v3.0.1 → v3.x)

| Priority | Item | Effort | Reference |
|----------|------|--------|-----------|
| **P1** | Implement timezone changes per ADR-0003 | ~50 LOC, ~6 tests | `docs/adr/0003-timezone-handling.md` |
| P2 | Finance domain + FinanceProvider | full SDD cycle | `/sdd-new finance` |
| P2 | Calendar domain + CalendarProvider | full SDD cycle | `/sdd-new calendar` |
| P3 | TUI `n` in areaTodos also opens the new-todo form | small (extract from areaToday) | pattern in `internal/tui/today_new_form.go` |
| P3 | `arsenal todo list --due` flag with `today`/`upcoming`/`overdue` presets | ~30 LOC | build on `DueAfter`/`DueBefore` |
| P3 | Web `/todos` filters for `today`/`upcoming`/`overdue` | ~50 LOC | mirrors CLI flag |

P1 is the only blocker to remove "Timezone handling" from Known limitations.

---

## 9. Quick reference: commands the next session will need

```bash
# Verify state
git status && go test ./... -count=1 -race

# Continue with timezone impl
# (then in chat: "implementá el timezone change per ADR-0003")

# Continue with a new domain
# (then in chat: "/sdd-new finance" or "/sdd-new calendar")

# Run the orchestrator status check
# (then in chat: "/sdd-status" — for any active change; none currently)
```

---

## 10. References

- `CHANGELOG.md` — full change log
- `docs/releases/v3.0.0.md` — release notes (paste into GitHub release)
- `docs/adr/0001-v3-scope.md` — v3 architecture
- `docs/adr/0002-v3-replan.md` — v3 replan
- `docs/adr/0003-timezone-handling.md` — timezone decision
- `openspec/specs/todo-*/spec.md` — 9 canonical todos specs
- `openspec/specs/today-*/spec.md` — 5 canonical today specs
- `openspec/changes/archive/2026-06-11-phase-2-todos/` — phase 2 planning
- `openspec/changes/archive/2026-06-11-phase-3-today/` — phase 3 planning
- engram topic keys (see §7.1)
