# ADR-0002: Arsenal v3 Replan — Scope, Sequencing and Corrected Decisions

**Status:** Accepted
**Date:** 2026-06-08
**Supersedes:** [ADR-0001](./0001-v3-scope.md) (partially — see "What stays" below)
**Context:** Critical review of ADR-0001 surfaced scope inflation, a fragile shared-foundation that will produce copy-paste services, an oversold FTS5 cross-domain claim, and an under-specified "Today" view (the killer feature). This ADR replans v3 so it ships a working, focused v3.0 in weeks instead of months, and pushes finance/calendar to a follow-up release.

---

## What stays (still true from ADR-0001)

These decisions are unchanged and remain the spine of v3:

- One SQLite DB for all domains (single backup, single migration path, cross-domain transactions).
- Shared `tags` and `categories` tables across all domains.
- Sub-area UX in TUI and web (Today / Resources / Todos / Finance / Calendar / Trash) with shared sidebar.
- Migrations per domain, ordered by timestamp, owned by the domain.
- No reminders domain, no daemon, no OS notifications in v3.
- Calendar scope: "controlling the daily routine", simple recurrence only (no RRULE), single system timezone.
- v2 resources stay untouched and stable during rollout.

---

## What changes

### Change 1 — v3.0 scope is reduced: only resources + todos + Today view

**New v3.0 deliverables:**

- Resources (already done in v2, kept as-is).
- Todos (phase 2 as planned).
- Today view (phase 3) — the killer feature that aggregates across resources + todos.

**Deferred to v3.x (post-3.0, independently shippable):**

- Finance (was phase 4 in ADR-0001). Becomes a separate minor release.
- Calendar (was phase 5). Becomes a separate minor release.

**Why:** ADR-0001 estimated "5-10× current code" for 4 domains × 3 surfaces. That is a low estimate. Real cost is closer to 4 domains × 3 surfaces × ~4 views × sqlc + service + tests + templates + completions + exports. That is 6+ months of part-time work, with high risk of losing the user before v3 ships. v3.0 with the Today view delivers the differentiating feature in weeks; finance/calendar extend the surface without blocking the release.

**Consequence:** the "Today" view in v3.0 will only aggregate resources + todos. When finance and calendar ship in v3.x, the Today view grows incrementally — no architectural rework needed if it was designed with a registry pattern (see Change 4).

### Change 2 — Insert a "Phase 1.5: shared domain helpers" before phase 2

**Problem:** the `resources.Service` has 30+ domain-specific call sites (CreateResource, ListTagsForResource, DetachAllTagsFromResource, DeleteOrphanTags, AttachTag with `ResourceID`, etc.). If todos/finance/calendar each get a 300-line copy of this with the suffix swapped, the codebase will diverge within 2 months and subtle bugs will appear (e.g., one domain prunes orphan tags inside the tx, another doesn't).

**Decision:** before phase 2, extract two small, generic helpers into `internal/domain/`:

- `domain.WithTags[T any](ctx, attachFn, ownerID, tags) error` — encapsulates the upsert-tag + attach + (optional) prune-orphans flow that every domain needs.
- `domain.NormalizeTags` already exists; keep it as the single source of truth for tag normalization.

The `resources.Service` is refactored to use the new helper. No public API change.

**Why now:** one-time 1-2 day cost that prevents 4× duplication and a year of drift. Doing it before phase 2 means todos is the second caller, which validates the abstraction. Doing it after phase 2 means refactoring 2 services + reconciling test deltas.

### Change 3 — `arsenal_config` gets a typed key catalog, not just a generic key/value table

**Problem:** the new `arsenal_config` table accepts any string. The ADR-0001 mentioned keys like `currency`, `landing_surface`, `active_domains`, but there is no catalog. Callers will spell keys differently, the `arsenal config list` command won't know how to describe them, and `arsenal config get curency` (typo) will fail silently with `ErrNotFound` instead of suggesting the right key.

**Decision:** add `internal/config/keys.go` with:

- A typed `Key` type (string under the hood) with package-level constants: `KeyCurrency`, `KeyLandingSurface`, `KeyActiveDomains`, etc.
- A `Catalog` map from `Key` to a `KeyMeta` struct: `Type` (string/bool/list), `Default`, `Description`, `Validate func(string) error`.
- `configstore.Set` accepts a `Key` plus `string`; lookup and validation go through the catalog.

**Why:** single source of truth for what config keys exist, what their types are, and how to validate them. The `arsenal config` CLI commands (`get`/`set`/`list`/`unset`) become trivially self-documenting.

### Change 4 — Today view is a registry, not a switch statement

**Problem:** ADR-0001's "Today view" is vague. A naive implementation will hardcode "SELECT from todos WHERE due < today UNION SELECT from events WHERE ..." — and the moment finance or calendar ship, the query has to be rewritten and the layout rearranged.

**Decision:** design the Today view around a small registry interface in `internal/today/`:

```go
// Provider contributes items to the Today view.
type Provider interface {
    Name() string                                 // "todos", "resources", ...
    Sections(ctx context.Context) ([]Section, error)  // overdue, due-today, upcoming, recent
}

// Registry aggregates providers and produces a sorted Today page.
type Registry struct { providers []Provider }
```

v3.0 ships with two providers: `TodosProvider` and `ResourcesProvider` (resources contribute "recently added / favorites" as a soft signal). When finance/calendar land in v3.x, they register themselves — no change to the rendering layer.

**Why:** keeps the killer feature extensible without rewriting. Costs a day of design now, saves weeks later.

### Change 5 — Cross-domain search is UNION ALL over per-domain FTS, not a unified FTS5

**Problem:** ADR-0001 says "single FTS5 index that crosses domains (search 'factura' finds expense + resource + todo)". The on-disk migrations disagree: every domain has its own virtual table (`resources_fts`, `todos_fts`) with its own sync triggers. There is no `all_fts`. Building a unified FTS5 means:
- One extra virtual table with rows for every domain insert/delete/update.
- Sync triggers in every domain migration writing into the unified table.
- A DELETE handler that has to know which domain the row came from.
- A debugging nightmare: "why is `factura` not in the index?"

**Decision:** cross-domain search in v3.0 is implemented as a UNION ALL across the per-domain `*_fts` virtual tables, with a common result shape (`{domain, id, title, snippet, score}`). Cheap to add a new domain to the union. No sync triggers added. No new virtual table.

**Why:** same user-visible result, far less infrastructure, no migration to revisit when a new domain ships. Performance is fine at <10k rows per domain.

### Change 6 — TUI area-switching is prototyped in phase 2, not deferred

**Problem:** ADR-0001's phase 2 plans to add the "Todos" sub-area in the TUI alongside resources, and phase 3 plans to add "Today" and the others. That is four areas, each with its own list/detail/keybindings, sharing one Bubble Tea root model. Getting the state machine and key-binding scopes wrong here means rewriting the TUI later.

**Decision:** in phase 2.5, prototype the TUI area-switcher with **all five areas wired as placeholders** (Resources, Todos, Today, Finance, Calendar). Only Resources and Todos are functional; the others show `(coming soon)`. This validates the area-switcher, the key-binding scoping, and the shared status bar **before** phase 3 builds Today on top of it.

**Why:** the cost of getting the TUI architecture wrong is a TUI rewrite. The cost of building three placeholder screens for one day is trivial.

### Change 7 — Phase 3 starts with `sdd-explore`, not `sdd-spec`

**Problem:** the "Today" view in ADR-0001 is "overdue todos, due-today, events today, upcoming 7 days, and recent expenses in one screen." That sentence hides at least these decisions:

- What is "today" (timezone — we said system tz, but events spanning midnight?).
- Ordering: overdue > due-today > upcoming > recent expenses — why that order? Are pinned items above all?
- How many items per section? Truncation rules?
- Refresh cadence: live, on-key, on-tick?
- Empty state and first-run UX.
- What does the TUI render vs what does the web render? Same data, different shapes.

**Decision:** phase 3 begins with a dedicated `sdd-explore` (or a focused `sdd-design`) on the Today view **before** writing specs. The exploration produces 3-5 concrete design questions for the user (order, density, refresh, empty state, mobile considerations for the web), gets them answered, and only then writes spec/design/tasks.

**Why:** the Today view is the differentiator. Building it from a 1-paragraph spec is how you ship a mediocre killer feature.

### Change 8 — Migrations: disk is source of truth, ADR gets updated

**Problem:** ADR-0001 says `migrations/20260601000002_init_todos.sql` but the on-disk file is `migrations/20260608000002_todos.sql`. Documentation desync.

**Decision:** from now on, **disk is source of truth for migration filenames**. ADRs do not restate filenames. If a phase plan needs to reference a specific migration, it references by its semantic purpose ("the todos migration") and the file is found by purpose, not by literal name.

### Change 9 — Performance ceiling and cross-version import are documented up front

**Decision:** add a short `docs/v3-limits.md` (or a section in DESIGN.md) that captures:

- Target scale: <10k rows per domain. SQLite + FTS5 + 5 virtual tables starts to feel the weight beyond that.
- WAL mode + `busy_timeout=5s` + `synchronous=NORMAL` are configured in `sqliteutil.Open`; documented in the file's godoc.
- v3 → v3.x import: there is none. Users stay on the same DB. v2 → v3.0 import is already handled by `arsenal migrate`.
- Export formats per domain: resources=markdown (existing), todos=markdown (new), finance=CSV (v3.x), calendar=iCal (v3.x). No unified export in v3.0.

---

## Updated rollout phases

| Phase | Deliverable | Working binary check | Status |
|---|---|---|---|
| 0 | ADR-0001: v3 scope (now superseded by this ADR) | N/A | DONE |
| 1 | Foundations: `sqliteutil`, `configstore`, thin store shim | `arsenal list` works identical to v2 | DONE |
| **1.5** | **Shared domain helpers (`internal/domain/with_tags.go`) + `config/keys.go` typed catalog** | **`go test ./...` green, no behavior change in v2 paths** | **NEW** |
| 2 | Todos end-to-end (schema + service + CLI + TUI area + web + tests). Includes 2.5 placeholder TUI areas for Today/Finance/Calendar. | `arsenal todo add "..." --due 2026-06-10` works on all 3 surfaces; TUI switches between Resources/Todos/(placeholders) | PENDING |
| 3 | "Today" cross-domain view (TUI + web), starts with `sdd-explore`. Uses Provider registry. Aggregates resources + todos. | `arsenal today` shows overdue + due-today + upcoming + recent resources in both TUI and web | PENDING |
| 4 (renamed) | Polish, completions, docs, release v3.0 | `arsenal` opens to a polished v3 with Resources, Todos, and Today | PENDING |
| v3.x (deferred) | Finance end-to-end + CSV export | `arsenal finance add 5000 --cat servicios` works | DEFERRED |
| v3.x (deferred) | Calendar end-to-end + iCal export | `arsenal calendar add "rutina" --at 07:00 --recurrence daily`; .ics importable in Google Cal | DEFERRED |
| v3.x (deferred) | Providers for finance/calendar register into Today registry | No code change to rendering layer | DEFERRED |

**No phase begins until the previous one's tests pass.** Each phase ends with a working binary.

---

## Consequences

### Positive

- v3.0 ships in weeks, not months, with the differentiating feature (Today).
- Foundation helpers prevent 4× duplication of tag/transaction logic.
- Today view as a registry means new domains plug in, not rewrite the view.
- Cross-domain search is honest about how it's built (UNION ALL) and trivial to extend.
- TUI architecture is validated with placeholders before any of the 4 areas are real.
- Configuration is typed and self-documenting.
- Disk is the single source of truth for migration filenames.

### Negative / risks

- Users who want finance/calendar in v3.0 wait for v3.x. Mitigation: finance and calendar are independent minor releases after v3.0, not blocked by it.
- Adding the Provider interface for Today means a small upfront design cost. Mitigation: pays back the first time a new domain lands.
- v3.x releases need their own ADR when they begin, to confirm finance/calendar scope still holds. Not a real cost.

---

## Open questions (to resolve in phase 1.5)

- TUI area-switching key: ADR-0001 suggested `1`=Today, `2`=Resources, `3`=Todos. Confirm or pick something more discoverable (`Tab` to cycle, or a sidebar). Recommend: `Tab` to cycle forward, `Shift+Tab` backward, plus number keys `1-5` as direct jump. To confirm with user in phase 1.5.
- Today view ordering and density: see Change 7 — resolved in phase 3 exploration.
- Default landing surface in TUI: Today (recommended) vs Resources (current). To confirm with user in phase 2.5.

---

## Related

- [ADR-0001: v3 scope](./0001-v3-scope.md) — superseded by this ADR for sequencing and technical decisions; still valid for the spine (single DB, shared tags, sub-areas, no daemon, calendar scope).
- Engram: `arsenal/adr/0001-v3-scope` (id 1434) is now marked `supersedes` relationship to this ADR's topic.
- Session summary: `arsenal-v3-session-2026-06-08` (id 1435) — updated to point here.
