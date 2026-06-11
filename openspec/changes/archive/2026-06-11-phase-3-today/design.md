# Design: phase-3-today — "Today" Cross-Domain View

## Goals & Non-Goals

**Goals:**
- Deliver 5 capabilities (today-view, today-providers, today-empty-state, today-tui, today-web) across 3 surfaces.
- Implement the Provider registry pattern from ADR-0002 Change 4: `Provider` interface, `Registry`, `Section`/`Item` types, graceful degradation.
- Wire `areaToday` from placeholder to real rendering; change TUI default landing surface from `areaResources` to `areaToday`.
- Add `/today` web route, sidebar Today entry with overdue-count badge, HTMX partial refresh.
- Keep the data layer unchanged — reuse existing store queries (`ListTodosDueBefore`, `ListTodosDueBetween`, `ListResourcesFiltered`).

**Non-Goals:**
- Finance and calendar providers (v3.x; ADR-0002 Change 1).
- Recurrence auto-expansion when a recurring todo is marked done (v3.x).
- Cross-domain search UI integration (deferred to phase 4).
- Pinned items, custom sections, user-configurable section ordering.
- Daemon, push notifications, background polling, goroutines.
- Timezone handling beyond `date('now')` in SQLite (known limitation, documented).
- New migration file or new virtual table.
- Import/export for the Today view.

## Architecture Overview

Phase 3 introduces a lightweight `internal/today/` package that implements the Provider registry from ADR-0002 Change 4. Each domain that wants to contribute to the Today view implements the `Provider` interface (`Name() string` + `Sections(ctx) ([]Section, error)`). The `Registry` holds ordered providers and delegates to each on request. A `Service` orchestrates the registry: iterates providers, collects sections, sorts by the fixed order (Overdue → Due Today → Upcoming → Recent Resources), applies the 5-item-per-section density cap, and returns the final page. The TUI and web surfaces are thin adapters over the same `Service`, calling `Build(ctx)` and rendering the result in their respective view layers. The CLI command (`arsenal today`) reuses the same service.

No new sqlc queries, no new migrations, no new virtual tables. The providers reuse existing store methods:
- `TodosProvider`: calls `ListTodosDueBefore`, `ListTodosDueBetween` (twice: today and tomorrow-to-7d), and `ListTodosFiltered`.
- `ResourcesProvider`: calls `ListResourcesFiltered({Limit: 5})` — the same query already used by `buildAside()` in the web layer.

The sidebar's overdue-count badge uses `CountOverdueTodos` which already exists in `handlers.go`. The `commonPage()` function stays lightweight: it computes only sidebar counts.

## File Layout

```
internal/
├── today/                                  # NEW package
│   ├── provider.go                         # Provider interface + Registry
│   ├── sections.go                         # Section + Item types + order map
│   ├── today.go                            # Service (Build method + density + degradation)
│   ├── empty.go                            # Empty-state renderer
│   ├── service_test.go                     # TDD: registry, ordering, density, degradation
│   └── providers/
│       ├── todos.go                        # TodosProvider
│       ├── resources.go                    # ResourcesProvider
│       ├── todos_test.go                   # TDD: section construction
│       └── resources_test.go               # TDD: recent section
├── cli/
│   ├── today.go                            # NEW: arsenal today command
│   └── root.go                             # MOD: register todayCmd
├── tui/
│   ├── today.go                            # NEW: updateToday + todayModel
│   ├── app.go                              # MOD: wire areaToday case, default landing
│   └── keys.go                             # MOD: nextArea/prevArea naming (no new key needed)
├── web/
│   ├── today.go                            # NEW: /today handlers + HTMX partials
│   ├── handlers.go                         # MOD: extend commonPage sidebar badge
│   └── templates/
│       ├── today.html                      # NEW: Today page template
│       └── layout.html                     # MOD: Today entry in sidebar + header nav
├── config/
│   └── keys.go                             # MOD: expand KeyLandingSurface EnumValues
└── store/
    (no changes — all queries already exist)
```

## Key Contracts

### `internal/today/provider.go`

```go
package today

import "context"

// Provider contributes named sections to the Today view. Implementations are
// domain-specific (todos, resources, finance, calendar). Each provider is
// independently queried; one failure does not block the others.
type Provider interface {
    Name() string                               // "todos", "resources", ...
    Sections(ctx context.Context) ([]Section, error)
}

// Registry holds ordered providers. v3.0 registers TodosProvider and
// ResourcesProvider. v3.x adds finance/calendar by calling Register.
type Registry struct { providers []Provider }

func NewRegistry() *Registry
func (r *Registry) Register(p Provider)
func (r *Registry) Collect(ctx context.Context) ([]Section, []ProviderError)
```

### `internal/today/sections.go`

```go
package today

// Section is a named group of items within the Today view.
type Section struct {
    Key        string // "overdue", "due-today", "upcoming", "recent"
    Title      string // "Overdue", "Due Today", "Upcoming", "Recent Resources"
    Items      []Item
    ShowAllURL string // empty when Items ≤ 5; otherwise link to domain list
    IsEmpty    bool   // true when provider returned 0 items → omitted from render
}

// Item is the cross-domain common shape rendered by TUI and web.
type Item struct {
    Domain   string   // "todos" | "resources"
    ID       int64
    Title    string
    Subtitle string   // due date, resource type, etc.
    Priority string   // "high" | "med" | "low" | "" (resources have no priority)
    Tags     []string
    URL      string   // "/todos/42" or "/resources/7" — empty for TUI-only items
}

// sectionOrder defines the fixed ordering for v3.0. Sections not in this
// map are appended at the end in their provider-defined order.
var sectionOrder = map[string]int{
    "overdue":   1,
    "due-today": 2,
    "upcoming":  3,
    "recent":    4,
}
```

### `internal/today/today.go`

```go
package today

import (
    "context"
    "database/sql"
)

const maxItemsPerSection = 5

// Service orchestrates the Today view. It owns the registry and applies
// section ordering, density truncation, and empty-state decisions.
type Service struct {
    db       *sql.DB
    registry *Registry
}

// New builds a Service with the standard v3.0 providers registered.
func New(db *sql.DB) *Service

// Build collects sections from all providers, orders them, truncates to
// density limits, and sets ShowAllURL for overflow sections. Returns the
// final ordered slice plus any provider errors for graceful degradation.
func (s *Service) Build(ctx context.Context) ([]Section, []ProviderError)
```

### `internal/today/providers/todos.go`

```go
package providers

import "github.com/edwinupegui/arsenal/internal/today"

type TodosProvider struct {
    db      *sql.DB
    queries *store.Queries
}

func NewTodosProvider(db *sql.DB) *TodosProvider
// Implements today.Provider.
// Sections() returns up to 3 sections: overdue, due-today, upcoming.
// Overdue: store.ListTodosFiltered({OnlyOverdue: true, Today: todayStr, Limit: 5})
// Due-today: store.ListTodosFiltered({DueBefore: tomorrowStr, Limit: 5}) with client-side filter for due_date==today
// Upcoming: store.ListTodosFiltered({Limit: 5}) with client-side filter for [tomorrow, today+7d]
```

### `internal/today/providers/resources.go`

```go
package providers

type ResourcesProvider struct {
    queries *store.Queries
}

func NewResourcesProvider(db *sql.DB) *ResourcesProvider
// Implements today.Provider.
// Sections() returns 1 section: recent.
// Recent: store.ListResourcesFiltered({Limit: 5})
```

### Web route contract

| Method | Path | Handler | Template | HTMX |
|--------|------|---------|----------|------|
| GET | `/today` | `todayPage` | `today.html` | No |
| GET | `/today/sections` | `todaySections` | `today.html` (sections fragment) | Yes |

### `KeyLandingSurface` expansion

```go
// internal/config/keys.go — MODIFIED EnumValues
KeyLandingSurface: {
    Type:        TypeEnum,
    Default:     "today",
    Description: "Default area shown when the TUI launches.",
    EnumValues:  []string{"today", "resources"},
},
```

The existing `["tui", "web"]` values are replaced. The old `"tui"` value has no equivalent — users who set `landing_surface=tui` will get the fallback `today` default. This is acceptable because v3.0 changes the TUI default anyway. The web landing surface concept is handled separately (the `arsenal web` subcommand always opens to `/resources` — no config key needed).

## Data Model

No new tables, no new columns, no new sqlc queries. The Today view reads existing data through existing store methods:

| Section | Store method | Filter |
|---------|-------------|--------|
| Overdue | `ListTodosFiltered` | `OnlyOverdue=true, Today=<today ISO>, Limit=5` |
| Due Today | `ListTodosFiltered` | `DueBefore=<tomorrow ISO>, Limit=50` → client-filter `due_date==today`, cap 5 |
| Upcoming | `ListTodosFiltered` | `Limit=50` → client-filter `due_date` in `[tomorrow, today+7d]`, cap 5 |
| Recent Resources | `ListResourcesFiltered` | `Limit=5` (default sort: `created_at DESC`) |
| Sidebar badge | `countOverdueTodos` | Already in `handlers.go:617` — raw `SELECT COUNT(*)` |

The Due-Today and Upcoming sections use a broader server-side filter with client-side refinement because the existing `ListTodosDueBetween` (sqlc-generated) exists but `ListTodosFiltered` offers a more flexible dynamic WHERE builder. The client-side filter is O(50) per section — negligible cost.

## Sequence Diagrams

### Today view load (TUI or Web)

```
User opens Today
    → today.Service.Build(ctx)
        → for each p in registry.providers:
            → p.Sections(ctx)
                → TodosProvider: calls store.ListTodosFiltered × 3 (overdue, due-today, upcoming)
                → ResourcesProvider: calls store.ListResourcesFiltered({Limit:5})
            → if err: collect ProviderError, skip provider, continue
        → sort sections by sectionOrder map
        → for each section: if len(items) > 5, truncate + set ShowAllURL
        → return []Section, []ProviderError
    → UI renders sections with items, density cap, "show all →" links
```

### Mark-done triggers Today refresh via hx-swap-oob (web)

```
POST /todos/42/done
    → handlers.markTodoDone(42)
    → todoService.MarkDone(42)
    → return todoCard fragment (existing behavior)
    + return sidebar-oob fragment with updated CountOverdueTodos badge
    → hx-swap-oob updates sidebar badge in-place
    → if user is on /today, hx-trigger="refreshSections" → GET /today/sections
```

### Refresh on `r` key in TUI

```
User presses r in areaToday
    → App.Update sees key.Matches(r, a.keys.Refresh) && currentArea == areaToday
    → return a, reloadTodayCmd(a.todayService)
        → reloadTodayCmd runs Build(ctx) in background
        → todayReloadedMsg{sections, errors} delivered
    → a.todayModel.sections = msg.sections
    → re-render
```

Note: the existing `r` binding in `keys.go` is `key.WithKeys("r")` for restore (trash view). In `areaToday`, refresh uses the lowercase `r`; the trash-view restore also uses `r` but only activates in trashed areas. Since `areaToday` has no trash toggle, there is no conflict. The spec clarifies: trash restore takes precedence in trash context; otherwise `r` refreshes Today.

## Test Strategy (Strict TDD)

**Mandatory** (must pass before PR merges):

| File | What it tests | Approach |
|------|--------------|----------|
| `internal/today/service_test.go` | Registry aggregation, section ordering, density cap, empty state, provider error degradation | Mock provider returning configurable sections/errors. Table-driven. |
| `internal/today/providers/todos_test.go` | Overdue/Due-Today/Upcoming section construction with real DB | `newTestDB(t)` pattern from resources/todos tests. Seed known todos, call Sections, assert. |
| `internal/today/providers/resources_test.go` | Recent section with `Limit:5` | `newTestDB(t)`, seed resources, assert 5 most recent returned. |

**Optional** (can defer, not blocking):

| File | What it tests | Approach |
|------|--------------|----------|
| `internal/tui/today_test.go` | Sub-model keybindings (`r` refresh, `n` new-todo), default landing | Direct `Update()` tests on `App`; check `currentArea` after init. |
| `internal/web/today_test.go` | `/today` route renders, sidebar badge, HTMX partials | `httptest.NewServer` + `http.Get` + assert template content. |
| `internal/cli/today_test.go` | Cobra `arsenal today` command with `--json` flag | `cmd.ExecuteContext(ctx)` + capture stdout. |

Pattern: `go test ./... -race -count=1` is the canonical command. All mandatory tests follow strict TDD: write test → see it fail → implement → see it pass.

## Migration & Rollout

- **No new migration**: data layer unchanged. Existing `20260608000002_todos.sql` and resources migrations are sufficient.
- **Branch strategy**: feature branch from `develop`, push, user opens PR to `develop`, user merges `develop` → `main` manually.
- **Verify**: `go test ./... -race -count=1` (TODAY tests pass) + manual TUI smoke (open, see Today, press `r`, press `n`, Tab-switch) + manual web smoke (`/today` renders, mark-done from `/todos` updates sidebar badge via OOB).
- **Config migration**: if `KeyLandingSurface` was previously set to `"tui"` or `"web"`, the value becomes invalid after the EnumValues change. The `configstore.Validate` path will reject it, and `GetLandingSurface()` falls back to default `"today"`. No data migration needed — the config row can stay as-is; reads will ignore it.

## Risks & Open Questions

- **R1 — `KeyLandingSurface` EnumValues backward incompatibility**: Current values are `["tui", "web"]`. Phase 3 changes them to `["today", "resources"]`. Users who set `landing_surface=tui` see fallback to `today` (harmless, since `today` is the new default). Mitigation: document in CHANGELOG.
- **R2 — 800-1200 LOC exceeds 400-line budget**: Size exception required (same as phase 2). Delivery via chained PRs: (1) core + providers, (2) TUI, (3) web, (4) empty-state.
- **R3 — Default landing change is a behavior break**: Users with muscle memory for Resources-on-launch will land on Today instead. AMEND scenarios in `todo-tui/spec.md` cover it. `KeyLandingSurface=resources` restores legacy behavior. CHANGELOG must call this out.
- **R4 — `commonPage()` isolation**: The Today handler computes full section data independently. `commonPage()` stays lightweight (count queries only). If this contract is violated, all web pages become expensive. Enforce in code review.
- **R5 — Provider error degradation**: If a provider fails, its sections are skipped. The `Service.Build()` returns `[]ProviderError` alongside `[]Section`. The caller renders a muted error indicator. This MUST be implemented from day 1, not deferred.
- **R6 — Due-Today/Upcoming use client-side date filtering**: The existing `ListTodosFiltered` doesn't have a `DueAfter` parameter. Workaround: broad `DueBefore` query + client-side date comparison. Acceptable at <50 rows per query. If performance becomes an issue, add a `DueAfter` field to `TodoListFilter` in a future patch. Not blocking.
- **R7 — Timezone (R5 from explore)**: `date('now')` is UTC. Due-today comparison may be off by hours for non-UTC users. Documented as known limitation; separate ADR for v3.0.1.

## Out of Scope (implementation-level)

- No new migration file.
- No new virtual table.
- No new sqlc query generation (`make sqlc` unchanged).
- No cross-domain search UI.
- No recurrence auto-expansion.
- No finance/calendar providers.
- No daemon/notifications.
- No timezone handling (separate ADR).
- No pinned items.
- No custom sections.
- No import/export for Today.
- No `commonPage()` expansion (Today data isolated to Today handler).
