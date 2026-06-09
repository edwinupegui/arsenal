# Proposal: phase-2-todos — Todos End-to-End

## Why

Phase 2 is the second of three v3.0 pillars (Resources + Todos + Today). Phase 1.5 shared domain helpers (`domain.WithTags`, `config/keys.go`) are merged, so todos validates the abstraction and proves the pattern scales to a second domain.

This proposal follows the replan in [ADR-0002](../docs/adr/0002-v3-replan.md), which reduced v3.0 scope to Resources + Todos + Today and introduced shared helpers, typed config keys, and a Provider registry for the Today view. The spine of [ADR-0001](../docs/adr/0001-v3-scope.md) (single DB, shared tags/categories, sub-areas, no daemon) remains valid.

## What Changes

Nine new capabilities are introduced. No existing specs are modified.

1. **todo-lifecycle** — Create, update, soft-delete, restore, and purge todos.
2. **todo-status** — Mark a todo as done or reopen it.
3. **todo-listing** — Filtered list views by status, priority, overdue, category, tag, and trashed.
4. **todo-search** — Full-text search over todos using the per-domain FTS5 virtual table (`todos_fts`), participating in the cross-domain UNION ALL strategy defined in ADR-0002 Change 5.
5. **todo-tags** — Attach tags to todos via `domain.WithTags`, reusing the shared tag namespace and normalization already proven in resources.
6. **todo-cli** — CLI commands: `add`, `list`, `show`, `done`, `open`, `rm`, `restore`, `edit`, `purge`.
7. **todo-tui** — TUI sub-area for todos, plus the area-switcher prototype with all five areas wired as placeholders (Resources functional, Todos functional, Today/Finance/Calendar showing `(coming soon)`), per ADR-0002 Change 6.
8. **todo-web** — Web views for list, new, edit, and show, plus sidebar updates.
9. **todo-recurrence-placeholder** — The `recurrence` field is persisted and displayed in forms and listings, but auto-expansion is out of scope for v3.0. See Q1 below.

## Impact

### Affected specs
- None modified.

### New code locations
- `internal/store/queries/todos.sql` — sqlc queries for todos.
- `internal/todos/` — domain types, service, and tests.
- `internal/cli/todo.go` — CLI commands.
- `internal/tui/todos.go` — TUI sub-area and area-switcher integration.
- `internal/web/templates/todos.html` — Web templates.

### Modified code locations
- `internal/store/db.go` — register todo queries.
- `internal/cli/root.go` — add `todo` subcommand.
- `internal/cli/completion.go` — todo completions.
- `internal/tui/app.go` — area-switcher and placeholder areas.
- `internal/web/handlers.go` — todo HTTP handlers.
- `internal/web/templates/layout.html` — sidebar links.

### Migrations
- No new migration required. The todos schema migration is already on disk (`migrations/20260608000002_todos.sql`). Disk is the source of truth per ADR-0002 Change 8.

### Tag orphan cleanup
- `DeleteOrphanTags` in the resources attacher already covers `todo_tags` in its UNION (per the comment in `resources/attacher.go`). No additional pruning logic is needed.

## Open Questions

1. **Q1 (recurrence semantics) — RESOLVED (option A)**: recurrence is metadata only. The field is persisted and displayed in forms and listings, but **no** auto-expansion happens when a recurring todo is marked done. This matches ADR-0002's "placeholder" framing. Auto-expansion and exception handling are explicitly out of v3.0 scope and will be revisited in v3.x. See engram obs 1449, topic `arsenal/v3/phase-2-recurrence-decision`.

2. **Q2 (default landing surface in TUI)**: Today (recommended) vs Resources (current). To confirm in phase 2.5.

3. **Q3 (TUI area-switching keybindings)**: ADR-0001 suggested `1`=Today, `2`=Resources, `3`=Todos. Confirm or pick something more discoverable (`Tab` to cycle, or a sidebar). Recommend: `Tab` to cycle forward, `Shift+Tab` backward, plus number keys `1-5` as direct jump. To confirm in phase 2.5.

4. **Q4 (todo listing truncation)**: How many items per filtered list before truncation/pagination? Recommend: 50 with "show more" in web, scrollable in TUI. To confirm in spec.

5. **Q5 (overdue definition)**: Is "overdue" strictly `due_date < today()` at midnight, or does time-of-day matter? Recommend: date-only comparison (todos are date-level granularity). To confirm in spec.

6. **Q6 (purge behavior)**: Hard-delete a soft-deleted todo immediately, or require a grace period / confirmation? Recommend: immediate purge with a confirmation prompt in CLI (`--yes` flag), direct in web with a confirmation dialog. To confirm in spec.

## Out of Scope

- Auto-expansion of recurring todos on completion (deferred to v3.x).
- Finance domain (deferred to v3.x per ADR-0002).
- Calendar domain (deferred to v3.x per ADR-0002).
- Today view aggregation (phase 3; this phase only wires the placeholder sub-area).
- Nested projects, dependencies, GTD contexts, waiting-for/someday buckets.
- Multi-currency, multi-account, budgets, bank sync (finance deferred).
- RRULE-grade recurrence, attendees, external calendar sync (calendar deferred).
- Cross-domain search UI (phase 3; per-domain FTS5 is wired here but not aggregated).
- Reminders, push notifications, daemon.

## Review Workload Forecast

- Estimated changed lines: ~400+ (new domain × 3 surfaces + tests + templates + completions).
- 400-line budget risk: High.
- Delivery strategy: ask-on-risk / chained PRs recommended.
- Chained PR strategy: pending orchestrator decision.
