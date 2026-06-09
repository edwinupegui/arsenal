# Tasks: phase-2-todos — Todos End-to-End

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1770 (design agent estimate) |
| 400-line budget risk | High (4.4× over) |
| Chained PRs recommended | No — size:exception approved |
| Suggested split | Single PR from `feat/phase-2-todos` → `main` |
| Delivery strategy | size:exception (engram obs 1452) |
| Chain strategy | size:exception |
| Commit plan | 6 work units, ~23 commits, each independently revertable |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size:exception
400-line budget risk: High

**Justification**: Foundation + first-feature phase mirrors the phase 1.5 pattern. Chained PRs add overhead not worth it for v3.0 phases. Maintainer approved.

---

## Work Unit A — Foundation: domain types and sqlc queries

### A1. Verify migration schema contract [x]
Validate `internal/migrations/20260608000002_todos.sql` matches design contract: 13 columns, CHECK constraints, 5 indexes, FTS5 virtual table, 5 sync triggers, `v_todo_tags` view.
- **Files**: `internal/migrations/20260608000002_todos.sql` (read-only verify)
- **Depends**: —
- **Acceptance**: Schema matches design §Data Model; all CHECK constraints present
- **Tests**: None (read-only verification)
- **Specs**: todo-lifecycle, todo-recurrence-placeholder

### A2. Write failing tests for domain enums [x]
Create `internal/todos/domain_test.go` with table-driven tests for `Priority.Valid()`, `Status.Valid()`, `Recurrence.Valid()`, `AllPriorities()`, `AllStatuses()`, `AllRecurrences()`, and `String()` methods.
- **Files**: `internal/todos/domain_test.go` (NEW)
- **Depends**: A1
- **Acceptance**: `go test ./internal/todos/...` FAILS (package doesn't exist yet)
- **Tests**: ~12 cases: valid/invalid values for each enum, All*() slice lengths
- **Specs**: todo-lifecycle (Priority/Status/Recurrence validation)

### A3. Implement domain enums [x]
Create `internal/todos/domain.go` with `Priority`, `Status`, `Recurrence` typed string enums, `Valid()`, `String()`, `All*()` methods, and `CreateInput` / `ListFilter` structs per design §Key Contracts.
- **Files**: `internal/todos/domain.go` (NEW)
- **Depends**: A2
- **Acceptance**: `go test ./internal/todos/...` PASSES (green)
- **Tests**: Tests from A2 now pass
- **Specs**: todo-lifecycle, todo-recurrence-placeholder

### A4. Write sqlc queries [x]
Create `internal/store/queries/todos.sql` with all 16 queries: `CreateTodo`, `GetTodo`, `ListTodos`, `ListTrashedTodos`, `UpdateTodo`, `SoftDeleteTodo`, `RestoreTodo`, `PurgeTodo`, `MarkTodoDone`, `MarkTodoOpen`, `ListTodosByStatus`, `ListTodosDueBefore`, `ListTodosDueBetween`, `CountOpenTodos`, `ListTodosFiltered`, `SearchTodos`. Also add `DetachAllTagsFromTodo` and `AttachTagToTodo` for the attacher.
- **Files**: `internal/store/queries/todos.sql` (NEW)
- **Depends**: A1
- **Acceptance**: SQL file parses; all 16+ queries present with correct annotations
- **Tests**: None (sqlc validation in A5)
- **Specs**: todo-lifecycle, todo-listing, todo-search, todo-tags

### A5. Run sqlc code generation [x]
Run `make sqlc` to regenerate `internal/store/todos.sql.go` and `internal/store/models.go`. Verify `go build ./internal/store/...` compiles.
- **Files**: `internal/store/todos.sql.go` (REGEN), `internal/store/models.go` (REGEN), `internal/store/querier.go` (REGEN)
- **Depends**: A4
- **Acceptance**: `make sqlc` exits 0; `go build ./internal/store/...` succeeds; `Todo` struct present in models
- **Tests**: `go build ./...` clean
- **Specs**: All (infrastructure)

### A6. Smoke test CreateTodo + read-back [x]
Write `internal/todos/smoke_test.go` that opens a fresh DB (via `newTestDB`), calls `store.Queries.CreateTodo`, reads it back with `GetTodo`, asserts fields match.
- **Files**: `internal/todos/smoke_test.go` (NEW)
- **Depends**: A3, A5
- **Acceptance**: `go test ./internal/todos/...` PASSES; validates sqlc regeneration works end-to-end
- **Tests**: 1 integration test: create → read-back → assert
- **Specs**: todo-lifecycle (Create with all fields, Create with defaults)

---

## Work Unit B — Service layer (strict TDD)

### B1. RED: Create happy path + tag dedup tests [x]
Write failing `internal/todos/service_test.go` with: `TestCreate_HappyPath` (all fields + tags), `TestCreate_Defaults`, `TestCreate_EmptyTitle`, `TestCreate_TitleTooLong`, `TestCreate_InvalidPriority`, `TestCreate_InvalidRecurrence`. Use `newTestDB(t)` pattern from `resources/service_test.go`.
- **Files**: `internal/todos/service_test.go` (NEW)
- **Depends**: A3, A5
- **Acceptance**: `go test ./internal/todos/...` FAILS with compile errors (Service doesn't exist)
- **Tests**: 6 test functions, ~10 scenarios
- **Scenarios covered**: 6 — lifecycle:1,2,3,4; recurrence:3; tags:1,2
- **Specs**: todo-lifecycle (Create), todo-recurrence-placeholder (invalid), todo-tags (normalization)

### B2. GREEN: Implement Service.Create + attacher [x]
Create `internal/todos/attacher.go` (mirror of `resources/attacher.go`, ~40 lines, implements `domain.Attacher` with `todo_id`-scoped `AttachTagToOwner`, delegates `DeleteOrphanTags` to shared query). Create `internal/todos/service.go` with `New()`, `Create()` using `sqliteutil.WithTx` + `domain.WithTags(pruneOrphans=false)`.
- **Files**: `internal/todos/attacher.go` (NEW), `internal/todos/service.go` (NEW)
- **Depends**: B1
- **Acceptance**: `go test ./internal/todos/...` PASSES (all B1 tests green)
- **Tests**: B1 tests now pass
- **Scenarios covered**: 6 — lifecycle:1,2; tags:1,2,3; recurrence:1,2
- **Specs**: todo-lifecycle (Create), todo-tags (attach, shared namespace), todo-recurrence-placeholder (persistence)

### B3. RED+GREEN: Get + Update with tag replacement [x]
Tests: `TestGet_Found`, `TestGet_NotFound`, `TestUpdate_ChangesPriority`, `TestUpdate_TagReplacementPrunesOrphans`, `TestUpdate_NonExistentFails`. Implement `Service.Get()` and `Service.Update()` (detach-all-then-reattach pattern, `pruneOrphans=true`).
- **Files**: `internal/todos/service_test.go` (MOD), `internal/todos/service.go` (MOD)
- **Depends**: B2
- **Acceptance**: All new + existing tests pass
- **Tests**: 5 test functions
- **Scenarios covered**: 5 — lifecycle:5,6,7
- **Specs**: todo-lifecycle (Update, Get), todo-tags (prune orphans)

### B4. RED+GREEN: SoftDelete, Restore, Purge [x]
Tests: `TestSoftDelete_Active`, `TestSoftDelete_AlreadyDeleted`, `TestRestore_SoftDeleted`, `TestRestore_Active`, `TestPurge_AfterSoftDelete`, `TestPurge_Active`. Implement `Service.SoftDelete()`, `Service.Restore()`, `Service.Purge()` (Purge uses `WithTx` + orphan prune).
- **Files**: `internal/todos/service_test.go` (MOD), `internal/todos/service.go` (MOD)
- **Depends**: B3
- **Acceptance**: All new + existing tests pass
- **Tests**: 6 test functions
- **Scenarios covered**: 5 — lifecycle:8,9,10,11,12,13
- **Specs**: todo-lifecycle (SoftDelete, Restore, Purge)

### B5. RED+GREEN: MarkDone + MarkOpen idempotency [x]
Tests: `TestMarkDone_OpenToDone`, `TestMarkDone_AlreadyDone` (no-op: done_at/updated_at unchanged), `TestMarkOpen_DoneToOpen`, `TestMarkOpen_AlreadyOpen` (no-op). Implement `Service.MarkDone()` and `Service.MarkOpen()`.
- **Files**: `internal/todos/service_test.go` (MOD), `internal/todos/service.go` (MOD)
- **Depends**: B4
- **Acceptance**: All new + existing tests pass; idempotency verified
- **Tests**: 4 test functions
- **Scenarios covered**: 4 — status:1,2,3,4
- **Specs**: todo-status (MarkDone, MarkOpen)

### B6. RED+GREEN: List with all filter combinations [x]
Tests: `TestList_DefaultOpen`, `TestList_FilterDone`, `TestList_FilterPriority`, `TestList_FilterCategory`, `TestList_FilterTag`, `TestList_FilterOverdue`, `TestList_FilterDueBefore`, `TestList_FilterTrashed`, `TestList_SortOrder`, `TestList_Pagination`. Implement `Service.List()` with dynamic WHERE builder delegating to `ListTodosFiltered`/`ListTrashedTodos`.
- **Files**: `internal/todos/service_test.go` (MOD), `internal/todos/service.go` (MOD)
- **Depends**: B5
- **Acceptance**: All filter combinations pass; sort order verified
- **Tests**: 10 test functions
- **Scenarios covered**: 10 — listing:1,2,3,4,5,6,7,8,9,10
- **Specs**: todo-listing (all requirements)

### B7. RED+GREEN: SearchTodos (FTS5) [x]
Tests: `TestSearch_TitlePrefix`, `TestSearch_Description`, `TestSearch_Notes`, `TestSearch_TagNames`, `TestSearch_SpecialCharsNoCrash`, `TestSearch_ExcludesTrashed`, `TestSearch_EmptyQuery`. Implement `SearchTodos` method in `internal/store/search.go` (reuse `buildFTSQuery`/`stripFTSSpecials`). Add `Service.List` search delegation when `ListFilter.Search != ""`.
- **Files**: `internal/todos/service_test.go` (MOD), `internal/store/search.go` (MOD)
- **Depends**: B6
- **Acceptance**: FTS5 search works across title/description/notes/tags; special chars don't crash; trashed excluded
- **Tests**: 7 test functions
- **Scenarios covered**: 7 — search:1,2,3,4,5,6,7
- **Specs**: todo-search (all requirements)

### B8. RED+GREEN: Rollback on validation failure [x]
Test: `TestCreate_RollbackOnValidation` — attempt create with invalid title inside a tx, verify no row inserted, verify tag rows not inserted. This validates `sqliteutil.WithTx` rollback semantics.
- **Files**: `internal/todos/service_test.go` (MOD)
- **Depends**: B7
- **Acceptance**: Validation failure produces zero rows in `todos` and `tags`
- **Tests**: 1 test function
- **Scenarios covered**: 1 — lifecycle:3 (reinforced)
- **Specs**: todo-lifecycle (reject empty title — no row inserted)

---

## Work Unit C — CLI

### C1. Parent command + add subcommand [x]
Create `internal/cli/todo.go` with `newTodoCmd()` parent and `newTodoAddCmd()` subcommand. Flags: `--priority`, `--due`, `--cat`, `--tag` (repeatable), `--notes`, `--recurrence`, `--desc`, `--json`. Title from first positional arg.
- **Files**: `internal/cli/todo.go` (NEW)
- **Depends**: B2
- **Acceptance**: `arsenal todo add "test" --priority high` creates a todo; `arsenal todo add` (no title) errors
- **Tests**: Manual (CLI integration tests optional per design)
- **Scenarios covered**: 2 — cli:1,2
- **Specs**: todo-cli (Add)

### C2. List subcommand with filters [x]
Add `newTodoListCmd()` with flags: `--status`, `--priority`, `--overdue`, `--cat`, `--tag`, `--trashed`, `--due-before`, `--limit`, `--offset`, `--json`. Text and JSON output modes.
- **Files**: `internal/cli/todo.go` (MOD)
- **Depends**: B6
- **Acceptance**: `arsenal todo list --priority high --overdue` filters correctly; `--json` outputs valid JSON array
- **Tests**: Manual
- **Scenarios covered**: 2 — cli:3,4
- **Specs**: todo-cli (List)

### C3. Show, done, open, edit subcommands [x]
Add `newTodoShowCmd()`, `newTodoDoneCmd()`, `newTodoOpenCmd()`, `newTodoEditCmd()`. Show prints full detail. Done/Open transition status. Edit uses same flags as add (with `--title`).
- **Files**: `internal/cli/todo.go` (MOD)
- **Depends**: B5
- **Acceptance**: `show 42` prints detail; `done 5` transitions; `open 5` transitions; `edit 5 --title "new"` updates
- **Tests**: Manual
- **Scenarios covered**: 6 — cli:5,6,7,8,13
- **Specs**: todo-cli (Show, Done, Open, Edit)

### C4. Rm, restore, purge subcommands [x]
Add `newTodoRmCmd()`, `newTodoRestoreCmd()`, `newTodoPurgeCmd()`. Purge requires `--yes` flag or interactive confirmation; fails in non-interactive mode without `--yes`.
- **Files**: `internal/cli/todo.go` (MOD)
- **Depends**: B4
- **Acceptance**: `rm 5` soft-deletes; `restore 5` restores; `purge 5 --yes` hard-deletes; `purge 5` (no TTY, no --yes) errors
- **Tests**: Manual
- **Scenarios covered**: 4 — cli:9,10,11,12
- **Specs**: todo-cli (Rm, Restore, Purge)

### C5. Wire into root + completions [x]
Add `root.AddCommand(newTodoCmd())` in `internal/cli/root.go`. Add todo completions in `internal/cli/completion.go`: priority values, status values, recurrence values, todo IDs, category slugs.
- **Files**: `internal/cli/root.go` (MOD), `internal/cli/completion.go` (MOD)
- **Depends**: C1, C2, C3, C4
- **Acceptance**: `arsenal todo` appears in root help; `<TAB>` completes priority/status/recurrence values
- **Tests**: Manual
- **Scenarios covered**: 1 — cli:14
- **Specs**: todo-cli (Shell completions)

---

## Work Unit D — TUI

### D1. Add currentArea enum to app.go
Add `areaID` type and constants (`areaToday`, `areaResources`, `areaTodos`, `areaFinance`, `areaCalendar`) to `internal/tui/app.go`. Add `currentArea` field to `App` struct. Default: `areaResources`.
- **Files**: `internal/tui/app.go` (MOD)
- **Depends**: B2
- **Acceptance**: App struct has `currentArea`; default is `areaResources`
- **Tests**: Existing resources TUI tests still pass
- **Scenarios covered**: 1 — tui:1
- **Specs**: todo-tui (Area enum)

### D2. Implement key routing for area switching
Add `Tab`/`Shift+Tab` cycle (forward/backward with wrap-around) and `1`-`5` direct jump to `App.Update()`. Status bar renders current area name + key hints.
- **Files**: `internal/tui/app.go` (MOD)
- **Depends**: D1
- **Acceptance**: Tab from Resources → Todos; Shift+Tab from Todos → Resources; `3` jumps to Todos; Calendar+Tab wraps to Today
- **Tests**: Direct `Model.Update()` tests for Tab/Shift+Tab/number keys
- **Scenarios covered**: 4 — tui:2,3,4,5
- **Specs**: todo-tui (Tab cycling, direct jump)

### D3. Add placeholder renderers for Today/Finance/Calendar
Add placeholder `View()` branches for non-functional areas: Today → "Today (coming soon — phase 3)", Finance → "Finance (coming soon — v3.x)", Calendar → "Calendar (coming soon — v3.x)".
- **Files**: `internal/tui/app.go` (MOD)
- **Depends**: D2
- **Acceptance**: Switching to Today/Finance/Calendar shows placeholder text
- **Tests**: Direct `View()` output assertions
- **Scenarios covered**: 2 — tui:6,7
- **Specs**: todo-tui (Placeholder areas)

### D4. Implement todos sub-model
Create `internal/tui/todos.go` with todo sub-model: scrollable list, filter by status, search box, keybindings (`x`=done/open, `d`=soft-delete, `r`=restore in trash, `enter`=detail, `n`=new form). Uses `todos.Service` for all operations.
- **Files**: `internal/tui/todos.go` (NEW)
- **Depends**: D2, B8
- **Acceptance**: Todo area shows scrollable list; `x` marks done; `d` deletes; `r` restores; `enter` shows detail
- **Tests**: Direct `Model.Update()` tests for keybindings
- **Scenarios covered**: 5 — tui:8,9,10,11,12
- **Specs**: todo-tui (Todo sub-model), todo-status, todo-lifecycle

### D5. Status bar shows current area
Update `App.View()` status bar to display current area name + area-switching key hints (`Tab`/`Shift+Tab`/`1-5`).
- **Files**: `internal/tui/app.go` (MOD)
- **Depends**: D2
- **Acceptance**: Status bar changes from "Resources" to "Todos" on area switch
- **Tests**: View output assertion
- **Scenarios covered**: 1 — tui:13
- **Specs**: todo-tui (Status bar)

### D6. Verify Resources area still works
Run existing TUI tests and manual smoke to confirm the `app.go` refactor didn't break the Resources area (list, detail, search, trash toggle, star/unstar).
- **Files**: — (verification only)
- **Depends**: D4, D5
- **Acceptance**: `go test ./internal/tui/...` passes; manual Resources flow works
- **Tests**: Existing test suite
- **Scenarios covered**: 0 (regression check)
- **Specs**: — (regression)

---

## Work Unit E — Web

### E1. Define routes in todos.go
Create `internal/web/todos.go` with route registration function. 11 routes per design §Web route contract: GET `/todos`, GET `/todos/new`, POST `/todos`, GET `/todos/{id}`, GET `/todos/{id}/edit`, POST `/todos/{id}`, POST `/todos/{id}/done`, POST `/todos/{id}/open`, POST `/todos/{id}/delete`, POST `/todos/{id}/restore`, POST `/todos/{id}/purge`. Wire into `internal/web/handlers.go` router setup.
- **Files**: `internal/web/todos.go` (NEW), `internal/web/handlers.go` (MOD)
- **Depends**: B8
- **Acceptance**: Routes registered; `GET /todos` returns 200 (even if handler is stub)
- **Tests**: Manual (route registration)
- **Scenarios covered**: 0 (routing infrastructure)
- **Specs**: todo-web (all routes)

### E2. Implement list + new + create handlers
Implement `listTodos` (GET `/todos` with filter query params), `newTodoForm` (GET `/todos/new`), `createTodo` (POST `/todos` with validation + redirect).
- **Files**: `internal/web/todos.go` (MOD)
- **Depends**: E1, E6
- **Acceptance**: `GET /todos` renders card list; `GET /todos/new` renders form; `POST /todos` creates + redirects; empty title re-renders form with error
- **Tests**: Manual
- **Scenarios covered**: 3 — web:1,2,3,4
- **Specs**: todo-web (List route, Create routes)

### E3. Implement show + edit + update handlers
Implement `showTodo` (GET `/todos/{id}`), `editTodoForm` (GET `/todos/{id}/edit`), `updateTodo` (POST `/todos/{id}` with redirect).
- **Files**: `internal/web/todos.go` (MOD)
- **Depends**: E2
- **Acceptance**: `GET /todos/42` renders detail; `GET /todos/42/edit` renders form; `POST /todos/42` updates + redirects
- **Tests**: Manual
- **Scenarios covered**: 2 — web:5,6
- **Specs**: todo-web (Show and edit routes)

### E4. Implement status transition handlers (HTMX)
Implement `markTodoDone` (POST `/todos/{id}/done`) and `markTodoOpen` (POST `/todos/{id}/open`). Return HTML card fragments for HTMX in-place swap.
- **Files**: `internal/web/todos.go` (MOD)
- **Depends**: E3
- **Acceptance**: POST done returns card fragment with done styling; POST open returns card with open styling
- **Tests**: Manual (HTMX)
- **Scenarios covered**: 2 — web:7,8
- **Specs**: todo-web (Status transition routes), todo-status

### E5. Implement delete/restore/purge handlers
Implement `softDeleteTodo` (POST, returns empty fragment for card removal), `restoreTodo` (POST, returns card fragment), `purgeTodo` (POST, redirect to `/todos`). Purge shows confirmation dialog.
- **Files**: `internal/web/todos.go` (MOD)
- **Depends**: E4
- **Acceptance**: Soft-delete removes card via HTMX; restore re-renders card; purge requires confirmation then redirects
- **Tests**: Manual (HTMX)
- **Scenarios covered**: 3 — web:9,10,11
- **Specs**: todo-web (Delete/restore/purge routes)

### E6. Add todoVM to viewmodel.go
Add `todoVM` struct mirroring `resourceVM` pattern: unwraps `sql.NullString`/`NullInt64`, resolves tag names, category slugs, formatted dates. Add `toTodoVM()` converter.
- **Files**: `internal/web/viewmodel.go` (MOD)
- **Depends**: A5
- **Acceptance**: `todoVM` has all fields needed by templates; `toTodoVM()` maps store rows correctly
- **Tests**: Unit test for `toTodoVM()` (tag resolution, null handling)
- **Scenarios covered**: 1 — web:12
- **Specs**: todo-web (View model)

### E7. Create todos.html templates
Create `internal/web/templates/todos.html` with: list view (card-based, filter controls), show view (detail), form view (create/edit reuse with `kind=todo`), card fragment (for HTMX swaps). Recurrence displayed in all views.
- **Files**: `internal/web/templates/todos.html` (NEW)
- **Depends**: E6
- **Acceptance**: All views render correctly; recurrence shown; filter controls functional
- **Tests**: Manual (visual)
- **Scenarios covered**: 4 — recurrence:4,5,6,7
- **Specs**: todo-web (templates), todo-recurrence-placeholder (display)

### E8. Update layout.html sidebar
Add "Todos" link to sidebar in `internal/web/templates/layout.html` with badge counts: open count, overdue count. Add `CountOpenTodos` query to `commonPage()`. Add `todoCountOpen` / `todoCountOverdue` to `pageData`.
- **Files**: `internal/web/templates/layout.html` (MOD), `internal/web/handlers.go` (MOD)
- **Depends**: E2, A5
- **Acceptance**: Sidebar shows "Todos (12 open, 3 overdue)"; counts update after HTMX actions
- **Tests**: Manual
- **Scenarios covered**: 2 — web:13,14
- **Specs**: todo-web (Sidebar integration)

### E9. HTMX partials for card swap
Ensure HTMX `hx-swap` attributes on todo cards work: mark-done/open swap outerHTML with updated card; soft-delete removes card; restore re-renders card. Sidebar counts refresh via `hx-swap-oob`.
- **Files**: `internal/web/templates/todos.html` (MOD), `internal/web/todos.go` (MOD)
- **Depends**: E4, E5, E8
- **Acceptance**: All HTMX transitions work: card swap, card removal, sidebar count update
- **Tests**: Manual (HTMX)
- **Scenarios covered**: 0 (HTMX wiring, covered by web:7-11)
- **Specs**: todo-web (HTMX behavior)

---

## Work Unit F — Final verification

### F1. Build verification
Run `go build ./...` — must exit 0 with no errors.
- **Files**: — (verification)
- **Depends**: C5, D6, E9
- **Acceptance**: Clean build
- **Specs**: All

### F2. Full test suite
Run `go test ./...` — all tests green (existing resources + new todos + domain + store).
- **Files**: — (verification)
- **Depends**: F1
- **Acceptance**: All tests pass; no regressions
- **Specs**: All (82 scenarios)

### F3. Lint verification
Run `golangci-lint run ./...` — must exit 0.
- **Files**: — (verification)
- **Depends**: F2
- **Acceptance**: No lint errors
- **Specs**: All

### F4. Sqlc verification
Run `make sqlc` — must produce no diff (generated code is committed).
- **Files**: — (verification)
- **Depends**: F3
- **Acceptance**: `git diff` shows no changes after `make sqlc`
- **Specs**: All

### F5. Manual CLI smoke test
Run through all CLI commands: `todo add`, `todo list` (with all filter flags), `todo show`, `todo done`, `todo open`, `todo edit`, `todo rm`, `todo restore`, `todo purge --yes`. Verify `--json` output.
- **Files**: — (manual)
- **Depends**: F1
- **Acceptance**: All commands work as specified
- **Scenarios covered**: 13 — cli:1-14 (manual)
- **Specs**: todo-cli (all)

### F6. Manual TUI smoke test
Launch TUI, verify: Tab/Shift+Tab cycle areas, 1-5 jump, Todos area shows list, mark done with `x`, delete with `d`, restore with `r` in trash, detail with `enter`. Verify Resources area still works. Verify placeholders for Today/Finance/Calendar.
- **Files**: — (manual)
- **Depends**: F1
- **Acceptance**: All TUI interactions work
- **Scenarios covered**: 10 — tui:1-13 (manual)
- **Specs**: todo-tui (all)

### F7. Manual web smoke test
Open browser: `GET /todos` (list), `GET /todos/new` (create form), create a todo, `GET /todos/{id}` (show), edit, mark done via HTMX, soft-delete, restore, purge. Verify sidebar counts update.
- **Files**: — (manual)
- **Depends**: F1
- **Acceptance**: All web flows work; HTMX transitions smooth; sidebar counts correct
- **Scenarios covered**: 12 — web:1-14 (manual)
- **Specs**: todo-web (all)

### F8. Update CHANGELOG
Add phase 2 entry to `CHANGELOG.md` (or release notes path) documenting: 9 new capabilities, 3 surfaces, migration applied, area-switcher prototype.
- **Files**: `CHANGELOG.md` (MOD)
- **Depends**: F2
- **Acceptance**: CHANGELOG has phase-2-todos entry
- **Specs**: — (documentation)

### F9. Final commit
Commit with `docs(phase-2): update CHANGELOG and close out`. Branch `feat/phase-2-todos` ready for PR to `main`.
- **Files**: — (commit)
- **Depends**: F8
- **Acceptance**: Clean `git log` on `feat/phase-2-todos`; ready for PR
- **Specs**: —

---

## Spec Traceability

| Spec ID | Implementing Tasks | Scenarios |
|---------|-------------------|-----------|
| todo-lifecycle | A2, A3, A4, A6, B1-B4, B8 | 13 |
| todo-status | B5, D4, E4 | 4 |
| todo-listing | A4, B6, C2, D4, E2 | 10 |
| todo-search | A4, B7 | 7 |
| todo-tags | A4, B1, B2, B3 | 5 |
| todo-cli | C1-C5, F5 | 13 |
| todo-tui | D1-D6, F6 | 10 |
| todo-web | E1-E9, F7 | 12 |
| todo-recurrence-placeholder | A2, A3, B1, B2, B6, E7, F5-F7 | 7 |
| **Total** | | **82** |

## Scenario Coverage by Task

| Task | Scenarios | Spec Areas |
|------|-----------|------------|
| B1 | 6 | lifecycle, recurrence, tags |
| B2 | 6 | lifecycle, tags, recurrence |
| B3 | 5 | lifecycle, tags |
| B4 | 5 | lifecycle |
| B5 | 4 | status |
| B6 | 10 | listing |
| B7 | 7 | search |
| B8 | 1 | lifecycle |
| C1 | 2 | cli |
| C2 | 2 | cli |
| C3 | 6 | cli |
| C4 | 4 | cli |
| C5 | 1 | cli |
| D1-D5 | 13 | tui |
| E2-E8 | 12 | web, recurrence |
| F5-F7 | 35 | cli, tui, web (manual) |

## Commit Plan (~23 commits)

| # | Work Unit | Commit Message | Tasks |
|---|-----------|---------------|-------|
| 1 | A | `feat(todos): add domain enums, sqlc queries, and smoke test` | A1-A6 |
| 2 | B | `feat(todos): implement Service.Create with tag attachment` | B1-B2 |
| 3 | B | `feat(todos): implement Get and Update with tag replacement` | B3 |
| 4 | B | `feat(todos): implement SoftDelete, Restore, and Purge` | B4 |
| 5 | B | `feat(todos): implement MarkDone and MarkOpen with idempotency` | B5 |
| 6 | B | `feat(todos): implement List with all filter combinations` | B6 |
| 7 | B | `feat(todos): implement FTS5 search via SearchTodos` | B7 |
| 8 | B | `test(todos): add rollback verification test` | B8 |
| 9 | C | `feat(cli): add todo parent command and add subcommand` | C1 |
| 10 | C | `feat(cli): add todo list with all filter flags` | C2 |
| 11 | C | `feat(cli): add todo show, done, open, and edit subcommands` | C3 |
| 12 | C | `feat(cli): add todo rm, restore, and purge subcommands` | C4 |
| 13 | C | `feat(cli): wire todo command and add shell completions` | C5 |
| 14 | D | `feat(tui): add area enum and key routing for area switching` | D1-D2 |
| 15 | D | `feat(tui): add placeholder renderers for Today/Finance/Calendar` | D3 |
| 16 | D | `feat(tui): implement todos sub-model with list and actions` | D4 |
| 17 | D | `feat(tui): update status bar and verify Resources regression` | D5-D6 |
| 18 | E | `feat(web): add todo routes and view model` | E1, E6 |
| 19 | E | `feat(web): implement list, create, show, and edit handlers` | E2-E3 |
| 20 | E | `feat(web): implement HTMX status transitions and delete/restore/purge` | E4-E5, E9 |
| 21 | E | `feat(web): add todos templates and sidebar integration` | E7-E8 |
| 22 | F | `test: full suite green, lint clean, sqlc verified` | F1-F4 |
| 23 | F | `docs(phase-2): update CHANGELOG and close out` | F8-F9 |
