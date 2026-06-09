# Tasks: phase-3-today — "Today" Cross-Domain View

## Review Workload Forecast

| Field                    | Value                                                                                   |
|--------------------------|-----------------------------------------------------------------------------------------|
| Estimated changed lines  | ~1000 (core ~250, providers ~200, TUI ~150, web ~200, tests ~150, config/layout ~50)    |
| 400-line budget risk     | High (2.5× over)                                                                        |
| Chained PRs recommended  | Yes — 2 slices                                                                          |
| Suggested split          | PR 1: core + providers + TUI (WUs A–C) → PR 2: web + verification (WUs D–E)             |
| Delivery strategy        | size:exception (single PR) OR chained (2 PRs) — user decision                           |
| Chain strategy           | stacked-to-main (pending user decision)                                                 |
| Commit plan              | 5 work units (A–E), ~16 commits, each independently revertable                          |

Decision needed before apply: Yes (chained PRs)
Chained PRs recommended: Yes
Chain strategy: pending user decision
400-line budget risk: High

**Justification**: 5 capabilities, ~1000 LOC forecast, 2 chained PRs recommended (core+providers+TUI in PR 1, web+verification in PR 2). Stacked-to-main fits the existing `develop`-based flow from phase 2. If maintainer prefers a single PR, `size:exception` is acceptable (same pattern as phase 2).

---

## Work Unit A — Core package: Provider + Registry + Service

- [ ] A1. Define `Provider`, `Section`, `Item` types
  Create `internal/today/provider.go` with the `Provider` interface (`Name() string`, `Sections(ctx) ([]Section, error)`). Create `internal/today/sections.go` with `Section` struct (Key, Title, Items, ShowAllURL, IsEmpty), `Item` struct (Domain, ID, Title, Subtitle, Priority, Tags, URL), and the `sectionOrder` map (`overdue=1`, `due-today=2`, `upcoming=3`, `recent=4`).
  - Files: `internal/today/provider.go` (NEW), `internal/today/sections.go` (NEW)
  - Depends: —
  - Acceptance: `go build ./internal/today/...` succeeds; types match design §Key Contracts
  - Tests: None (type definitions only)
  - Specs: REQ-TV-01 (Provider interface), REQ-TV-07 (Item shape), REQ-TV-03 (section order map)

- [ ] A2. Define `Registry` with Register + Collect
  Add `Registry` struct to `internal/today/provider.go` with `NewRegistry()`, `Register(p Provider)`, and `Collect(ctx) ([]Section, []ProviderError)`. `Collect` iterates providers, calls `Sections(ctx)`, and aggregates results. On provider error, records `ProviderError{Name, Err}` and continues to next provider.
  - Files: `internal/today/provider.go` (MOD)
  - Depends: A1
  - Acceptance: `Registry.Collect` aggregates sections from multiple providers; errors are captured, not propagated
  - Tests: None (covered by A3 RED)
  - Specs: REQ-TV-02 (Registry aggregation), REQ-TV-06 (provider error degradation)

- [ ] A3. RED: registry aggregation + ordering + density cap + empty state tests
  Create `internal/today/service_test.go` with table-driven tests: `TestRegistry_CollectsFromTwoProviders`, `TestRegistry_SectionOrderingFixed`, `TestService_DensityTruncatesAt5`, `TestService_NoTruncationBelowDensity`, `TestService_EmptySectionsOmitted`, `TestService_ProviderErrorDegradesGracefully`, `TestService_ShowAllURLSetOnOverflow`. Use a mock `Provider` that returns configurable sections/errors.
  - Files: `internal/today/service_test.go` (NEW)
  - Depends: A1, A2
  - Acceptance: `go test ./internal/today/...` FAILS (Service doesn't exist yet)
  - Tests: `TestRegistry_CollectsFromTwoProviders`, `TestRegistry_SectionOrderingFixed`, `TestService_DensityTruncatesAt5`, `TestService_NoTruncationBelowDensity`, `TestService_EmptySectionsOmitted`, `TestService_ProviderErrorDegradesGracefully`, `TestService_ShowAllURLSetOnOverflow`
  - Specs: REQ-TV-02, REQ-TV-03, REQ-TV-04, REQ-TV-06; Scenarios: Registry collects from two providers, Section ordering is fixed, Density truncates at 5, No truncation when at or below density, Provider error degrades gracefully, Empty sections are omitted

- [ ] A4. GREEN: implement Service.Build
  Create `internal/today/today.go` with `Service` struct (db, registry), `New(db)`, and `Build(ctx) ([]Section, []ProviderError)`. `Build` calls `registry.Collect`, sorts sections by `sectionOrder`, truncates each to 5 items, sets `ShowAllURL` for overflow sections, and omits empty sections.
  - Files: `internal/today/today.go` (NEW)
  - Depends: A3
  - Acceptance: `go test ./internal/today/...` PASSES (all A3 tests green)
  - Tests: A3 tests now pass
  - Specs: REQ-TV-02, REQ-TV-03, REQ-TV-04, REQ-TV-05, REQ-TV-06

- [ ] A5. Wire default providers + ShowAllURL mapping
  Wire `New(db)` to register `TodosProvider` and `ResourcesProvider` by default (v3.0 standard set). Add `ShowAllURL` generation logic: Overdue → `/todos?status=open&overdue=true`, Due Today → `/todos?status=open&due=today`, Upcoming → `/todos?status=open&due=upcoming`, Recent → `/resources`.
  - Files: `internal/today/today.go` (MOD)
  - Depends: A4
  - Acceptance: `New(db)` registers both providers; `ShowAllURL` values match design
  - Tests: Existing A3 tests still pass; add `TestService_ShowAllURLMapping` if not covered
  - Specs: REQ-TV-04 (show-all links), REQ-TW-03 (URL mapping)

- [ ] A6. RED+GREEN: empty state renderer
  Create `internal/today/empty.go` with `IsEmptyPage(sections []Section) bool` and `RenderEmptyState(surface string) string` (returns TUI or web empty message). RED: write tests first. GREEN: implement.
  - Files: `internal/today/empty.go` (NEW), `internal/today/empty_test.go` (NEW)
  - Depends: A4
  - Acceptance: `IsEmptyPage` returns true only when all sections empty; `RenderEmptyState` returns surface-specific hints
  - Tests: `TestIsEmptyPage_AllEmpty`, `TestIsEmptyPage_PartialData`, `TestRenderEmptyState_TUI`, `TestRenderEmptyState_Web`
  - Specs: REQ-ES-01 (global empty state), REQ-ES-02 (per-section empty), REQ-ES-03 (shortcut hints); Scenarios: Global empty state in TUI, Global empty state in web, Partial data skips empty sections

---

## Work Unit B — Concrete providers (TodosProvider + ResourcesProvider)

- [ ] B1. RED: TodosProvider tests with real test DB
  Create `internal/today/providers/todos_test.go` with `newTestDB(t)` pattern. Seed known todos with varying due dates and statuses. Assert section construction for overdue, due-today, upcoming. Verify done/deleted exclusion and item URL mapping.
  - Files: `internal/today/providers/todos_test.go` (NEW)
  - Depends: A4
  - Acceptance: `go test ./internal/today/providers/...` FAILS (TodosProvider doesn't exist)
  - Tests: `TestTodosProvider_OverdueSection`, `TestTodosProvider_DueTodaySection`, `TestTodosProvider_UpcomingSection`, `TestTodosProvider_OmitsEmptySections`, `TestTodosProvider_ExcludesDoneAndDeleted`, `TestTodosProvider_ItemMappingIncludesURL`
  - Specs: REQ-TP-01, REQ-TP-02, REQ-TP-03, REQ-TP-04, REQ-TP-06; Scenarios: TodosProvider returns overdue/due-today/upcoming sections, omits empty, excludes done/deleted, item mapping

- [ ] B2. GREEN: implement TodosProvider
  Create `internal/today/providers/todos.go` with `TodosProvider` struct (db, queries). Implement `Name() "todos"` and `Sections(ctx)`. Use `ListTodosDueBefore(today)` for overdue, `ListTodosDueBetween(today, today)` for due-today, `ListTodosDueBetween(tomorrow, today+7d)` for upcoming. Map rows to `Item` with `Domain="todos"`, `URL="/todos/{id}"`.
  - Files: `internal/today/providers/todos.go` (NEW)
  - Depends: B1
  - Acceptance: `go test ./internal/today/providers/...` PASSES (all B1 tests green)
  - Tests: B1 tests now pass
  - Specs: REQ-TP-01, REQ-TP-02, REQ-TP-03, REQ-TP-04, REQ-TP-06

- [ ] B3. RED: ResourcesProvider tests
  Create `internal/today/providers/resources_test.go`. Seed 8 resources, assert only 5 most recent returned. Verify empty DB returns no section. Verify item mapping with `Domain="resources"`.
  - Files: `internal/today/providers/resources_test.go` (NEW)
  - Depends: A4
  - Acceptance: `go test ./internal/today/providers/...` FAILS (ResourcesProvider doesn't exist)
  - Tests: `TestResourcesProvider_RecentSection`, `TestResourcesProvider_OmitsSectionWhenNoResources`, `TestResourcesProvider_Limit5`, `TestResourcesProvider_ItemMapping`
  - Specs: REQ-TP-05, REQ-TP-06; Scenarios: ResourcesProvider returns recent section, omits when no resources

- [ ] B4. GREEN: implement ResourcesProvider
  Create `internal/today/providers/resources.go` with `ResourcesProvider` struct. Implement `Name() "resources"` and `Sections(ctx)`. Use `ListResourcesFiltered({Limit: 5})`. Map to `Item` with `Domain="resources"`, `Subtitle=resource.Type`, `Priority=""`, `URL="/resources/{id}"`.
  - Files: `internal/today/providers/resources.go` (NEW)
  - Depends: B3
  - Acceptance: `go test ./internal/today/providers/...` PASSES (all B3 tests green)
  - Tests: B3 tests now pass
  - Specs: REQ-TP-05, REQ-TP-06

- [ ] B5. Integration: full Build returns expected ordered sections
  Add `internal/today/integration_test.go` with `TestService_Build_Integration` using a real test DB. Seed 3 overdue, 2 due-today, 4 upcoming todos + 8 resources. Call `Service.Build(ctx)`. Assert 4 sections in order, each truncated to 5, with correct `ShowAllURL`.
  - Files: `internal/today/integration_test.go` (NEW)
  - Depends: B2, B4
  - Acceptance: Full integration test passes; sections ordered correctly; density cap applied
  - Tests: `TestService_Build_Integration`
  - Specs: REQ-TV-02, REQ-TV-03, REQ-TV-04; Scenarios: Registry collects from two providers, Section ordering is fixed, Density truncates at 5

- [ ] B6. Provider error degradation: mock provider returns error
  Add `TestRegistry_ProviderErrorSkipped` in `service_test.go`. Register a mock provider that returns an error. Call `Collect(ctx)`. Assert the error is captured in `[]ProviderError`, the other provider's sections still render, and no panic occurs.
  - Files: `internal/today/service_test.go` (MOD)
  - Depends: A4
  - Acceptance: Mock provider error doesn't break the registry; other sections render
  - Tests: `TestRegistry_ProviderErrorSkipped`
  - Specs: REQ-TV-06; Scenario: Provider error degrades gracefully

---

## Work Unit C — TUI sub-area + default-landing change

- [ ] C1. RED: app_test for areaToday dispatching + r key reload
  Create `internal/tui/today_test.go` with tests for areaToday dispatching to updateToday, `r` key triggering reload, and `n` key opening new-todo form. Use direct `App.Update()` calls with mock key events.
  - Files: `internal/tui/today_test.go` (NEW)
  - Depends: A5
  - Acceptance: `go test ./internal/tui/...` FAILS (updateToday doesn't exist)
  - Tests: `TestApp_AreaToday_DispatchesToUpdateToday`, `TestApp_AreaToday_RKeyTriggersReload`, `TestApp_AreaToday_NKeyOpensNewTodo`
  - Specs: REQ-TT-01 (wire areaToday), REQ-TT-02 (r key refresh), REQ-TT-03 (n key new todo); Scenarios: Today area renders real data, r key refreshes, n key opens new-todo form

- [ ] C2. GREEN: implement updateToday + viewToday
  Create `internal/tui/today.go` with `updateToday(msg tea.Msg)` and `viewToday() string`. `updateToday` handles `todayReloadedMsg` (updates model sections) and triggers `reloadTodayCmd` on init. `viewToday` renders sections with headers, items, density, and empty state (via `today.RenderEmptyState("tui")`).
  - Files: `internal/tui/today.go` (NEW)
  - Depends: C1
  - Acceptance: `go test ./internal/tui/...` PASSES; Today area renders real data
  - Tests: C1 tests now pass
  - Specs: REQ-TT-01, REQ-TT-02, REQ-TT-03

- [ ] C3. RED: status bar shows "Today" + key hints in areaToday
  Add `TestApp_StatusBar_TodayHints` in `today_test.go`. Assert status bar renders "Today" with hints: `r` refresh, `n` new todo, `Tab`/`Shift+Tab` switch, `1-5` jump.
  - Files: `internal/tui/today_test.go` (MOD)
  - Depends: C2
  - Acceptance: `go test ./internal/tui/...` FAILS (status bar doesn't show Today hints)
  - Tests: `TestApp_StatusBar_TodayHints`
  - Specs: REQ-TT-04; Scenario: Status bar shows Today hints

- [ ] C4. GREEN: status bar update
  Modify `internal/tui/app.go` (or `status.go`) to render context-aware hints when `currentArea == areaToday`. Replace placeholder hints with Today-specific hints.
  - Files: `internal/tui/app.go` (MOD)
  - Depends: C3
  - Acceptance: `go test ./internal/tui/...` PASSES; status bar shows Today hints
  - Tests: C3 test now passes
  - Specs: REQ-TT-04

- [ ] C5. Default landing change: KeyLandingSurface config lookup
  Modify `internal/tui/app.go` `New()` to read `KeyLandingSurface` from config. If value is `"today"` or missing, set `currentArea = areaToday`. If `"resources"`, set `currentArea = areaResources`. If invalid, fall back to `areaToday`. Modify `internal/config/keys.go` to expand `EnumValues` from `["tui", "web"]` to `["today", "resources"]`.
  - Files: `internal/tui/app.go` (MOD), `internal/config/keys.go` (MOD)
  - Depends: C2
  - Acceptance: TUI launches to Today by default; `KeyLandingSurface=resources` launches to Resources; invalid value falls back to Today
  - Tests: `TestApp_DefaultLanding_Today`, `TestApp_LandingSurface_Resources`, `TestApp_LandingSurface_InvalidFallback`
  - Specs: REQ-TT-05 (default landing), REQ-TT-06 (config override); Scenarios: Default landing is Today, KeyLandingSurface=resources overrides, Invalid falls back

- [ ] C6. Regression: Resources area still works
  Run existing TUI tests (`go test ./internal/tui/...`). Manual smoke: launch TUI, verify Resources area still renders list, detail, search, trash toggle, star/unstar. Verify Tab/Shift+Tab cycle still works.
  - Files: — (verification only)
  - Depends: C2, C5
  - Acceptance: All existing TUI tests pass; manual Resources flow works
  - Tests: Existing test suite
  - Specs: — (regression check)

---

## Work Unit D — Web route + sidebar + HTMX OOB

- [ ] D1. RED: GET /today handler test
  Create `internal/web/today_test.go` with `httptest.NewServer`. Seed test DB with todos and resources, call `GET /today`, assert 200 + template content. Test empty state rendering.
  - Files: `internal/web/today_test.go` (NEW)
  - Depends: A5
  - Acceptance: `go test ./internal/web/...` FAILS (handler doesn't exist)
  - Tests: `TestTodayPage_Renders`, `TestTodayPage_ShowsAllSections`, `TestTodayPage_EmptyState`
  - Specs: REQ-TW-01 (/today route); Scenarios: Today page renders all sections

- [ ] D2. GREEN: handler + render today.html
  Create `internal/web/today.go` with `todayPage` handler. Call `todayService.Build(ctx)`, pass sections to `today.html` template. Create `internal/web/templates/today.html` with section rendering, item cards, density truncation, "show all →" links, and empty state (via `today.RenderEmptyState("web")`). Register route in `handlers.go`.
  - Files: `internal/web/today.go` (NEW), `internal/web/templates/today.html` (NEW), `internal/web/handlers.go` (MOD)
  - Depends: D1
  - Acceptance: `go test ./internal/web/...` PASSES; `GET /today` returns 200 with rendered sections
  - Tests: D1 tests now pass
  - Specs: REQ-TW-01, REQ-TW-03 (show-all links), REQ-TW-06 (commonPage isolation)

- [ ] D3. Sidebar "Today" entry with overdue count badge
  Modify `internal/web/templates/layout.html` to add "Today" link as first sidebar entry (before Resources, Todos). Add overdue badge using `CountOverdueTodos` from `commonPage()`. Hide badge when count is 0. Modify `internal/web/handlers.go` `commonPage()` to compute `todoCountOverdue` (reuse existing `countOverdueTodos` query).
  - Files: `internal/web/templates/layout.html` (MOD), `internal/web/handlers.go` (MOD)
  - Depends: D2
  - Acceptance: Sidebar shows "Today" link with overdue badge on all pages; badge hidden when 0
  - Tests: `TestSidebar_TodayEntryWithBadge`, `TestSidebar_BadgeHiddenWhenZero`
  - Specs: REQ-TW-02 (sidebar entry + badge), REQ-TW-05 (sidebar ordering); Scenarios: Sidebar shows Today link with overdue badge, Sidebar hides badge when zero overdue

- [ ] D4. HTMX partials: hx-swap-oob for badge refresh
  Modify `markTodoDone` handler in `internal/web/todos.go` to return an `hx-swap-oob` fragment for the sidebar badge after marking done. Add unique `id` attributes to each section in `today.html`. When a todo is marked done from `/today`, the response includes OOB updates for the affected section and the sidebar badge.
  - Files: `internal/web/todos.go` (MOD), `internal/web/templates/today.html` (MOD)
  - Depends: D2, D3
  - Acceptance: Mark-done from `/today` updates section and sidebar badge via OOB without full reload
  - Tests: Manual (HTMX)
  - Specs: REQ-TW-04 (hx-swap-oob); Scenario: Mark-done from Today refreshes section and badge

- [ ] D5. Show-all links point to existing routes
  Verify "show all →" links in `today.html` match design: Overdue → `/todos?status=open&overdue=true`, Due Today → `/todos?status=open&due=today`, Upcoming → `/todos?status=open&due=upcoming`, Recent → `/resources`. No new routes created.
  - Files: `internal/web/templates/today.html` (MOD)
  - Depends: D2
  - Acceptance: All "show all →" links navigate to existing routes with correct query params
  - Tests: Manual (click-through)
  - Specs: REQ-TW-03; Scenarios: show-all link navigates to filtered todo list, show-all for Recent Resources navigates to resources

---

## Work Unit E — Final verification + CHANGELOG

- [ ] E1. go build ./... clean
  Run `go build ./...` — must exit 0 with no errors.
  - Files: — (verification)
  - Depends: C6, D5
  - Acceptance: Clean build
  - Tests: `go build ./...`
  - Specs: All

- [ ] E2. go test ./... -count=1 -race all green
  Run `go test ./... -count=1 -race` — all tests pass (existing resources + todos + new today).
  - Files: — (verification)
  - Depends: E1
  - Acceptance: All tests pass; no regressions
  - Tests: `go test ./... -count=1 -race`
  - Specs: All (~41 scenarios)

- [ ] E3. go vet ./... clean
  Run `go vet ./...` — must exit 0.
  - Files: — (verification)
  - Depends: E2
  - Acceptance: No vet errors
  - Tests: `go vet ./...`
  - Specs: All

- [ ] E4. make sqlc no drift
  Run `make sqlc` — must produce no diff (no new queries added in phase 3).
  - Files: — (verification)
  - Depends: E3
  - Acceptance: `git diff` shows no changes after `make sqlc`
  - Tests: `make sqlc && git diff --exit-code`
  - Specs: All

- [ ] E5. CHANGELOG entry for phase 3
  Add phase 3 entry to `CHANGELOG.md` documenting: 5 new capabilities (today-view, today-providers, today-empty-state, today-tui, today-web), 3 surfaces (TUI, web, CLI), Provider registry pattern, default landing change, no new migrations.
  - Files: `CHANGELOG.md` (MOD)
  - Depends: E2
  - Acceptance: CHANGELOG has phase-3-today entry
  - Tests: Manual
  - Specs: — (documentation)

- [ ] E6. Update FE/BE tasks markers; final commit
  Update any project tracking docs (if present) to mark phase 3 tasks complete. Commit with `feat(today): add Today cross-domain view with provider registry`. Branch `feat/phase-3-today` ready for PR to `develop`.
  - Files: — (commit)
  - Depends: E5
  - Acceptance: Clean `git log` on `feat/phase-3-today`; ready for PR
  - Tests: Manual
  - Specs: —

---

## Spec Traceability

| Spec ID                | Implementing Tasks                          | Scenarios |
|------------------------|---------------------------------------------|-----------|
| today-view             | A1, A2, A3, A4, A5, B5, B6                  | 10        |
| today-providers        | B1, B2, B3, B4, B5                          | 8         |
| today-empty-state      | A6, C2, D2                                  | 4         |
| today-tui              | C1, C2, C3, C4, C5, C6                      | 7         |
| today-web              | D1, D2, D3, D4, D5                          | 7         |
| todo-tui (AMEND)       | C5, C6                                      | 3         |
| todo-web (AMEND)       | D3                                          | 2         |
| **Total**              |                                             | **41**    |

---

## Commit Plan (~16 commits)

| #  | Work Unit | Commit Message                                                                  | Tasks      |
|----|-----------|-------------------------------------------------------------------------------|------------|
| 1  | A         | `feat(today): add Provider interface, Section/Item types, and section order`  | A1         |
| 2  | A         | `feat(today): implement Registry with Collect and provider error degradation` | A2, A3, A4 |
| 3  | A         | `feat(today): implement Service.Build with density cap and show-all URLs`     | A5         |
| 4  | A         | `feat(today): add empty state renderer for TUI and web surfaces`              | A6         |
| 5  | B         | `feat(today): implement TodosProvider with overdue/due-today/upcoming`        | B1, B2     |
| 6  | B         | `feat(today): implement ResourcesProvider with recent section`                | B3, B4     |
| 7  | B         | `test(today): add full integration test for Service.Build`                    | B5, B6     |
| 8  | C         | `feat(tui): wire areaToday to real Today view with r/n keybindings`           | C1, C2     |
| 9  | C         | `feat(tui): add context-aware status bar hints for Today area`                | C3, C4     |
| 10 | C         | `feat(tui): change default landing to areaToday with KeyLandingSurface`       | C5         |
| 11 | C         | `test(tui): verify Resources area regression after Today wiring`              | C6         |
| 12 | D         | `feat(web): add /today route with handler and today.html template`            | D1, D2     |
| 13 | D         | `feat(web): add Today sidebar entry with overdue count badge`                 | D3         |
| 14 | D         | `feat(web): add hx-swap-oob for section and badge refresh on mark-done`       | D4, D5     |
| 15 | E         | `test: full suite green, vet clean, sqlc verified`                            | E1-E4      |
| 16 | E         | `docs(phase-3): update CHANGELOG and close out`                               | E5, E6     |
