# Proposal: phase-3-today — "Today" Cross-Domain View

## Why

Phase 3 is the third and final v3.0 pillar. Phase 1.5 (shared `domain.WithTags`, `config/keys.go`) and phase 2 (todos end-to-end + TUI area-switcher prototype with `areaToday` placeholder) are merged. The data layer is ready: `ListTodosDueBefore`, `ListTodosDueBetween`, `ListTodosFiltered`, `CountOverdueTodos` and `ListResourcesFiltered` already exist. What's missing is the **aggregation layer that turns Arsenal from "another TODO app" into a daily-driver command center**.

This proposal follows [ADR-0002](../docs/adr/0002-v3-replan.md). Two ADR changes are directly relevant:

- **Change 4 (Provider registry)**: the Today view is a registry of `Provider` implementations, not a hardcoded switch statement. v3.0 ships two providers — `TodosProvider` and `ResourcesProvider` — and v3.x adds finance/calendar by registration, not by rewriting the renderer.
- **Change 7 (explore-first)**: phase 3 began with `sdd-explore` instead of `sdd-spec`. The five closed product questions (section ordering, density, refresh, default landing, empty state) are documented in `explore.md` and inform every capability below.

The spine of [ADR-0001](../docs/adr/0001-v3-scope.md) — single DB, shared tags/categories, sub-areas, no daemon — remains valid. Timezone is a known limitation surfaced by R5; it is explicitly out of scope for phase 3 (see Open Questions).

## What Changes

Five new capabilities are introduced. The two existing capabilities (`todo-tui`, `todo-web`) get AMEND scenarios for the area-switcher default change; `todo-cli` does not (no `arsenal today` command surface in the existing todo CLI).

1. **`today-view`** — Core: `Provider` interface, `Registry`, `Section`/`Item` types, ordering, density, refresh contract, graceful degradation on provider error.
2. **`today-providers`** — Concrete providers: `TodosProvider` (overdue, due-today, upcoming) and `ResourcesProvider` (recent / favorites). Kept **merged** under one capability because they are structurally identical, share lifecycle, and have no independent churn surface; splitting them would create two empty spec files that always change together. v3.x providers (finance, calendar) get their own capability then.
3. **`today-empty-state`** — First-run and zero-data rendering with shortcut hints, both in TUI and web.
4. **`today-tui`** — Wire `areaToday` from placeholder to real `updateToday()`; add `r` (refresh) and `n` (new todo) keybindings; replace the default TUI landing surface from `areaResources` to `areaToday` with `KeyLandingSurface` config override.
5. **`today-web`** — `/today` route, HTMX partials for section refresh, sidebar entry with overdue-count badge, `commonPage()` data-source isolation so non-Today pages stay lightweight.

**AMEND scenarios** for existing capabilities (spec-level behavior change):
- **`todo-tui`** — area-switcher default landing surface changes from `areaResources` to `areaToday`. AMEND scenario covering the `KeyLandingSurface` override.
- **`todo-web`** — sidebar gains a "Today" entry with overdue badge. AMEND scenario covering sidebar nav ordering and badge data source.
- `todo-cli` is **not** amended: no `arsenal today` command lives under the `todo` subcommand (the new `arsenal today` top-level command is part of `today-tui`/`today-web` and lives in `internal/cli/today.go`, not in the todo CLI surface).

## Impact

### Affected specs
- `todo-tui` — AMEND: area-switcher default + `KeyLandingSurface` config key valid values.
- `todo-web` — AMEND: sidebar nav adds Today entry, sidebar counts include overdue.
- `todo-cli` — no AMEND.
- `resource-tui`, `resource-web` — no AMEND (no behavior change for resources users).

### New code locations
- `internal/today/` — NEW package: `provider.go` (Provider interface + Registry), `sections.go` (Section/Item types), `today.go` (Service that orchestrates the registry), `empty.go` (empty-state renderer), `providers/todos.go`, `providers/resources.go`.
- `internal/cli/today.go` — NEW: `arsenal today` top-level command with `--json` flag.
- `internal/tui/today.go` — NEW: today sub-model.
- `internal/web/today.go` — NEW: handlers for `/today` and HTMX partials.
- `internal/web/templates/today.html` — NEW.

### Modified code locations
- `internal/tui/app.go` — MOD: wire `areaToday` → `updateToday()`; change default `currentArea` from `areaResources` to `areaToday` (read from `KeyLandingSurface` with default `today`).
- `internal/tui/keys.go` — MOD: add `r` (refresh) and `n` (new todo) keybindings; add `KeyLandingSurface` documentation.
- `internal/tui/status.go` (or wherever the status bar lives) — MOD: render context-aware hints (`r` to refresh, `n` to add todo).
- `internal/web/handlers.go` — MOD: register `/today` route; extend `commonPage()` sidebar counts with overdue (lightweight, no full aggregation).
- `internal/web/templates/layout.html` — MOD: add "Today" entry to sidebar with overdue-count badge; add to header nav.
- `internal/config/keys.go` — MOD: confirm `KeyLandingSurface` exists in the catalog (it should from phase 1.5); document new valid values `today` (default) and `resources` (legacy).
- `internal/cli/root.go` — MOD: register `today` top-level subcommand.

### Migrations
- None. No new schema, no new virtual tables. The two existing `*_fts` virtual tables are untouched (per ADR-0002 Change 5, cross-domain search is UNION ALL, not a unified index).

### Tests
- Primary coverage at the `internal/today/` package level: registry, provider ordering, density truncation, error degradation.
- `internal/todos/service_test.go` — extend only if the provider requires a new query shape (likely not; `ListTodosDueBefore`/`ListTodosDueBetween` are sufficient).
- `internal/resources/service_test.go` — extend only if the recent-resources provider needs a new query shape (likely not; `ListResourcesFiltered({Limit: 5})` is already used by `buildAside`).

## Open Questions

1. **Q1 — `arsenal today` CLI command scope**: is the top-level `arsenal today` command in phase 3, or a follow-up? **Recommend: in phase 3**, mirrors `arsenal todo list` and `arsenal resource list`, one-liner with `--json`. Low cost, high discoverability, and it gives the user a headless path to the killer feature. To confirm in spec.
2. **Q2 — "show all →" link target**: route to the existing `/todos?status=open&due_before=today` query, or to a new dedicated view? **Recommend: existing route with query params** — keeps the surface count down and reuses the proven todo list filter. To confirm in spec.
3. **Q3 — Timezone (R5 from explore)**: ADR-0002 says "single system timezone" for v3.0, but `date('now')` in SQLite is UTC. **Recommend: defer to a separate small ADR for v3.0.1** unless the user wants it resolved in phase 3. Documenting as a known limitation in `docs/v3-limits.md` is the minimum bar. To confirm.
4. **Q4 — Cross-domain search UI (T4 from explore)**: the Today view is a dashboard, not a search box. Where does cross-domain search live? **Recommend: NOT in phase 3.** Defer to phase 4 polish; if it must ship in 3.0, a minimal inline filter on the existing `/search` route is the lowest-cost option. To confirm in spec.

## Out of Scope

- Finance providers (v3.x; ADR-0002 Change 1).
- Calendar providers (v3.x; ADR-0002 Change 1).
- Recurrence auto-expansion when a recurring todo is marked done (v3.x; phase 2's `todo-recurrence-placeholder` decision holds).
- Pinned items within sections.
- Custom user-defined sections (section set is fixed by providers).
- Cross-domain search UI (Q4 above; deferred to phase 4 unless promoted).
- Daemon, push notifications, background refresh, polling goroutines.
- Timezone handling beyond documenting the limitation (Q3 above).
- Mobile-specific layout for the web view (the existing responsive layout is sufficient; revisit if usage data shows otherwise).

## Review Workload Forecast

- Estimated changed lines: **800–1200 LOC** (new `internal/today/` package ~400 LOC, TUI sub-model + keybindings ~150, web handlers + template + partials ~250, CLI command ~80, tests ~200, layout/config/sidebar edits ~70).
- 400-line budget risk: **High (2–3× the per-PR budget).** **Size exception required.**
- Delivery strategy: **chained PRs** (per the `chained-pr` skill), sliced along capability boundaries:
  1. `today-view` + `today-providers` (core + providers, testable in isolation)
  2. `today-tui` (wires the placeholder, keybindings, default landing)
  3. `today-web` (route, templates, sidebar entry, HTMX partials)
  4. `today-empty-state` (lands last so empty states are designed against the real renderer)
- Chained PR strategy: pending orchestrator decision; the four-slice split above is the recommendation.
