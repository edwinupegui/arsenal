# Exploration: phase-3-today — "Today" Cross-Domain View

## 1. What We Already Know

### Locked-In Decisions from ADR-0002

These are settled and MUST NOT be re-litigated in phase 3:

- **Provider registry (Change 4)**: The Today view is a registry of `Provider` implementations, not a hardcoded switch statement. Each provider contributes `Sections(ctx) → []Section` where sections are named groups like "overdue", "due-today", "upcoming", "recent". The registry sorts and renders them. v3.0 ships two providers: `TodosProvider` and `ResourcesProvider`.

- **Per-domain FTS5 + UNION ALL (Change 5)**: Cross-domain search uses `UNION ALL` across `resources_fts` and `todos_fts`, not a unified virtual table. Result shape: `{domain, id, title, snippet, score}`. No new sync triggers, no new virtual table.

- **No daemon, no notifications (What stays)**: No background refresh, no OS-level notifications, no polling goroutines. The Today view refreshes on explicit user action (key press, page load, `r` refresh key).

- **One SQLite DB, shared tags/categories (What stays)**: Single migration path, cross-domain transactions possible via `sqliteutil.WithTx`.

- **v3.0 scope = resources + todos + Today only (Change 1)**: Finance and calendar providers are deferred to v3.x. Today view in v3.0 aggregates only resources + todos.

### Surface Patterns from Phase 2

From `phase-2-todos/design.md` and the actual code:

- **TUI area-switcher is already wired**: `internal/tui/app.go` has `areaID` enum with `areaToday` (placeholder), `areaResources`, `areaTodos`, `areaFinance`, `areaCalendar`. Tab/Shift+Tab cycles, 1-5 direct jump. Default landing is `areaResources`. The placeholder renders "Today (coming soon — phase 3)".

- **Area dispatch pattern**: `App.Update()` routes by `a.currentArea` — `areaTodos` goes to `updateTodos()`, `areaResources` goes to resource sub-views. `areaToday` currently hits the `default` case and returns `nil` (no-op). Phase 3 must wire `areaToday` to an `updateToday()` method.

- **Web sidebar has no Today entry**: `layout.html` header nav and sidebar list Resources, Todos, Categories, Tags, Trash. No "Today" link. The `commonPage()` function builds sidebar counts for resources and todos separately. A Today entry needs to be added.

- **`commonPage()` data source**: Already queries `CountResources`, `CountOpenTodos`, `CountOverdueTodos`. Today view will need richer data (overdue list, due-today list, upcoming list, recent resources) — these are heavier than the sidebar counts.

- **Todo queries available in store**:
  - `ListTodosDueBefore(dueDate)` — overdue items
  - `ListTodosDueBetween(start, end)` — due-today and upcoming items
  - `CountOpenTodos` — sidebar count
  - `ListTodosFiltered(TodoListFilter)` — flexible filter with `OnlyOverdue`, `DueBefore`, `Status`, `Priority`
  - `SearchTodos(query, limit)` — FTS5 search
  - `ListResourcesFiltered(ListFilter)` — resources with category/tag/type/language/fav filters

- **Resources queries available**: `ListResourcesFiltered` with `OnlyFavorite`, `Trashed` flags. Recent resources = `ListResourcesFiltered({Limit: 5})` (already used by `buildAside`).

### Patterns Established

- Services own business logic, store owns queries, TUI/web/CLI are thin adapters.
- HTMX fragments for in-place updates (mark-done, star, soft-delete).
- Confirm modal pattern: `data-confirm` attribute on forms, JS intercepts submit.
- Status bar in TUI shows area name + key hints.

---

## 2. Product Decisions That Need User Input

These are UX/product questions the user MUST answer before spec or design begins.

### Q1: Section Ordering Within Today View

**Question**: In what order should sections appear in the Today view?

**Recommended default**: Overdue → Due Today → Upcoming (next 7 days) → Recent Resources

**Alternatives**:
- *Overdue first always*: Matches the urgency model — overdue is always the most actionable signal, even if there are zero overdue items. The empty section is skipped.
- *Due Today first*: If there are no overdue items, the user lands on today's agenda immediately. More optimistic framing.
- *Recent Resources first*: Deprioritizes todos in favor of "what did you recently save?" This is the wrong default because Today is about *action*, not *browsing*.

**Why it matters**: This is the visual hierarchy of the killer feature. Ordering communicates what Arsenal thinks is important. Getting it wrong makes the Today view feel like a dump of data instead of a curated agenda.

### Q2: Items Per Section / Density

**Question**: How many items should each section show before truncating with "show all"?

**Recommended default**: 5 items per section, with a "show all →" link to the full domain list filtered to that section's criteria.

**Alternatives**:
- *3 items per section*: Tighter, forces the user to drill down. Good for mobile web. Risk: if you have 4 overdue items, you see all 4 — no truncation benefit.
- *10 items per section*: More data, less navigation. Risk: Today view becomes a wall of text. Loses the "glanceable" quality.
- *Unlimited*: No truncation. Defeats the purpose of a curated view — you're just reproducing the domain list.

**Why it matters**: Density controls whether Today is a dashboard or a list. The 5-item default balances completeness with scannability. The "show all" link provides an escape hatch without cluttering the default view.

### Q3: Refresh Cadence

**Question**: When does the Today view refresh its data?

**Recommended default**: On explicit user action only — `r` key in TUI, page reload in web. No background polling.

**Alternatives**:
- *On every key press in TUI*: Reloads on every navigation. Wastes SQLite roundtrips. The user sees stale data for at most one keypress anyway.
- *On a 5-minute tick in TUI*: Adds a goroutine, timer, and state management. Violates the "no daemon" constraint. Overkill for a local-first tool.
- *On web page focus event*: HTMX can listen for `visibilitychange` and trigger a GET. Good for web, but adds JS complexity. Not needed for v3.0.

**Why it matters**: Refresh strategy touches the "no daemon" constraint in ADR-0002. Choosing on-demand keeps the architecture simple and predictable.

### Q4: Default TUI Landing Area

**Question**: Should the TUI open to Today (new default) or Resources (current default)?

**Recommended default**: `areaToday` — the Today view becomes the home screen. Users launch Arsenal to see what's due today, then navigate to Resources/Todos as needed.

**Alternatives**:
- *Keep `areaResources`*: Preserves existing behavior. Users who are used to Resources don't experience a breaking change. Risk: the "killer feature" is hidden behind a `1` keypress on first launch.
- *Configurable via `configstore`*: Let the user pick. Adds a config key (`landing_surface`), which is already in the ADR-0003 config catalog. Low effort, but defers the decision.

**Why it matters**: The default landing area sets the tone for the entire v3.0 experience. Today-as-default signals that Arsenal is a daily driver, not just a bookmark manager.

### Q5: Empty State and First-Run UX

**Question**: What should the user see when the Today view has no items?

**Recommended default**: A friendly empty state message: "Nothing due today. Add a todo or browse your resources." with a keyboard shortcut hint (`n` to add todo, `2` to go to Resources).

**Alternatives**:
- *Show placeholder text only*: "No items to show today." Minimal, but gives no guidance. The user has to discover next actions on their own.
- *Show a quick-start guide*: "Welcome to Arsenal! Here's how to get started..." Adds onboarding complexity. Overkill for a local tool — the user already installed it.
- *Hide empty sections entirely*: If a section has 0 items, don't render the header. Clean, but the user can't tell if the section exists. Confusing for new users.

**Why it matters**: First-run experience determines whether the user explores Arsenal or closes it. The empty state is the first thing a new user sees if they have no data yet.

---

## 3. Technical Decisions Still to Make

These are ONLY decidable AFTER the product questions above are resolved.

### T1: Section Data Shape

**Depends on**: Q1 (ordering), Q2 (density).

The `Section` struct in the Provider interface needs to carry enough data for both TUI and web rendering:

```go
type Section struct {
    Key     string // "overdue", "due-today", "upcoming", "recent"
    Title   string // "Overdue", "Due Today", "Upcoming", "Recent Resources"
    Items   []Item
    ShowAll string // URL or command to show full list (optional)
}
```

The `Item` struct is the cross-domain shape. It needs to work for both TUI list items and web card templates. The exact fields depend on what the user needs to see at a glance (title, domain, due date, priority, tags — but not full description or notes).

### T2: Provider Interface Refinement

**Depends on**: Q3 (refresh), Q2 (density).

The ADR-0002 `Provider` interface is a sketch. Phase 3 must finalize:
- Does `Sections(ctx)` accept a limit parameter, or does the provider hardcode density?
- Does the provider return items already sorted, or does the registry sort?
- What happens when a provider errors? Does the whole Today view fail, or does that section degrade gracefully?

### T3: TUI vs Web Rendering Split

**Depends on**: Q1 (ordering), Q2 (density), Q5 (empty state).

- **TUI**: Sections render as grouped list items in a Bubbletea viewport. Each section has a header line. Overflow items hidden with "+N more".
- **Web**: Sections render as card groups in the main content area. HTMX can refresh individual sections. The sidebar needs a Today entry with an item count (overdue count as the badge, like TodoCounts.Open for Todos).

### T4: Cross-Domain Search UI Integration

**Depends on**: ADR-0002 Change 5 (UNION ALL).

ADR-0002 flags this as "ambiguous" for v3.0 scope. The Today view is NOT a search UI — it's a curated dashboard. Cross-domain search is a separate feature that may land in the Today view's search box, or may live at `/search` in the web UI. This needs clarification.

---

## 4. Out of Scope (Phase 3)

These items are explicitly excluded from phase 3. Do NOT design or implement them.

- **Finance providers**: Deferred to v3.x. No `FinanceProvider` in the registry.
- **Calendar providers**: Deferred to v3.x. No `CalendarProvider` in the registry.
- **Recurrence auto-expansion**: When a recurring todo is marked done, the next occurrence is NOT auto-created. This is a v3.x feature.
- **Cross-domain search UI integration**: ADR-0002 Change 5 defines the query strategy (UNION ALL), but the UI for cross-domain search is flagged as ambiguous. The Today view's search behavior (if any) should be minimal — redirect to domain-specific search or a simple inline filter, not a full cross-domain search experience.
- **Pinned items**: No "pin to top" functionality within sections.
- **Custom sections**: Users cannot add their own sections to the Today view. The section set is fixed by the providers.
- **Daemon / background refresh**: No goroutines, no timers, no OS notifications. Refresh is user-initiated only.

---

## 5. Risks & Open Questions

### R1: Area-Switcher Default Change (Q4)

Changing the TUI default from `areaResources` to `areaToday` is a breaking change for existing users. The `App` struct in `app.go` line 148 hardcodes `currentArea: areaResources`. If Q4 resolves to Today-as-default, this line changes. Users who muscle-memory `Tab` from launch will land in a different place.

**Mitigation**: This is acceptable because v3.0 is a major version. But it should be called out in release notes.

### R2: Sidebar Entry for Today (Web)

The web sidebar (`layout.html`) currently lists Resources, Todos, Categories, Tags, Trash. Adding Today requires:
- A new `<a href="/today">` link in the sidebar.
- A count badge (overdue count is the natural choice — same pattern as `TodoCounts.Open`).
- The `commonPage()` function in `handlers.go` must query the Today provider(s) for the count. This is heavier than the current count queries but runs once per page load.
- The `sidebar-oob` template block (used for HTMX sidebar refreshes) must also include the Today link.

**Risk**: If the Today provider aggregation is slow (multiple queries), the sidebar count adds latency to every page. Mitigation: cache the count in `pageData` and only recompute on explicit refresh.

### R3: `commonPage()` Data Source Expansion

`commonPage()` currently calls 4 lightweight count queries. For the Today view to render, it needs the full section data (overdue list, due-today list, upcoming list, recent resources). This is significantly more expensive than count queries.

**Decision needed**: Should `commonPage()` always compute full Today data (wasted on non-Today pages), or should the Today handler compute its own data independently?

**Recommended**: The Today handler computes its own data. `commonPage()` stays lightweight. The sidebar Today badge uses `CountOverdueTodos` (already exists) — no full aggregation needed for the sidebar.

### R4: Provider Error Degradation

If `TodosProvider.Sections()` fails (e.g., DB locked), should the entire Today view fail, or should the Todos sections be omitted and the Resources sections still render?

**Recommended**: Graceful degradation. Each provider is independent. If one fails, its sections are skipped with a muted error message in that section's place.

### R5: Timezone Handling

ADR-0002 says "single system timezone." The Today view's "due today" comparison uses `date('now')` in SQLite (already in the overdue count query at `handlers.go:620`). This is UTC-based. If the user's system timezone is not UTC, "due today" may be off by hours. This is an existing behavior in the todo queries, not new to phase 3, but it surfaces when Today is the home screen.

**Mitigation**: Document as a known limitation. A future improvement could use Go's `time.Now().In(loc)` to compute the local date and pass it to queries.

---

## 6. Recommended Path Forward

```
sdd-explore (this document)
    ↓
sdd-propose  →  User answers Q1-Q5, confirms out-of-scope
    ↓
sdd-spec     →  Delta specs for today/domain, today/tui, today/web
    ↓
sdd-design   →  Provider interface finalization, section data shape, TUI/web rendering
    ↓
sdd-tasks    →  Task breakdown with 400-line budget guard
    ↓
sdd-apply    →  Implementation in deliverable work units
    ↓
sdd-verify   →  Tests prove specs match implementation
    ↓
sdd-archive  →  Merge deltas into main specs
```

**What the orchestrator should tell the user**: "Before we design the Today view, I need answers to 5 product questions about section ordering, density, refresh, default landing, and empty state. Each has a recommended default — you can accept all defaults or override any."

---

## Exploration: phase-3-today

### Current State
The TUI has a placeholder `areaToday` that renders "Today (coming soon — phase 3)". The web has no Today route or sidebar entry. The todo store has `ListTodosDueBefore`, `ListTodosDueBetween`, `CountOpenTodos`, and `ListTodosFiltered` queries ready. The resources store has `ListResourcesFiltered` with favorite/recent capabilities. The ADR-0002 Provider registry pattern is accepted but not implemented.

### Affected Areas
- `internal/tui/app.go` — Wire `areaToday` to `updateToday()`, change default `currentArea`
- `internal/tui/todos.go` — Potentially share todo item rendering with Today view
- `internal/web/handlers.go` — New `/today` route, `commonPage()` sidebar Today entry
- `internal/web/templates/layout.html` — Today link in header nav + sidebar
- `internal/today/` (new package) — Provider interface, Registry, TodosProvider, ResourcesProvider
- `internal/store/list.go` — May need a "due between" helper or the existing `ListTodosDueBetween` is sufficient

### Approaches

1. **Provider-first (recommended)** — Build the Provider interface and Registry first, then wire providers, then wire TUI/web rendering on top.
   - Pros: Clean separation, extensible for v3.x, matches ADR-0002 vision
   - Cons: More upfront design, slightly more code before first visible result
   - Effort: Medium

2. **View-first** — Build the TUI Today view with hardcoded queries, extract Provider interface later.
   - Pros: Faster first visible result, validates the UX before abstracting
   - Cons: Refactoring cost when providers are introduced, risk of designing the interface around the implementation
   - Effort: Low initially, Medium overall (refactor tax)

### Recommendation
Provider-first. The ADR already committed to the registry pattern. Building it first means the TUI and web renderers are thin adapters from day one. The view-first approach saves a day upfront but costs two days of refactoring when the registry lands.

### Risks
- Changing TUI default landing is a breaking change for existing users
- `commonPage()` expansion could add latency to all web pages if not careful
- Timezone handling is a known limitation that surfaces when Today is the home screen

### Ready for Proposal
Yes — pending user answers to Q1-Q5 (product questions). The orchestrator should present the 5 questions with recommended defaults and ask the user to accept or override.
