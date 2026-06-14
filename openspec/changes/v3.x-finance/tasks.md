# Tasks: v3.x-finance — Finance Domain End-to-End

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 3000–3600 (source ~2700, tests ~1150, generated ~215) |
| 400-line budget risk | High (7.5–9× over) |
| Chained PRs recommended | Yes |
| Suggested split | 3–4 PRs depending on strategy (see below) |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending user decision |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Migration + sqlc + domain + service + attacher (foundation) | PR 1 | Locks schema + service contract; ~1600 lines with tests |
| 2 | CLI + CSV export + Provider + Today wiring | PR 2 | First user-visible surface; ~900 lines |
| 3 | TUI sub-model + area wiring | PR 3 | ~450 lines; depends on PR 1 |
| 4 | Web handlers + templates + sidebar + docs | PR 4 | ~950 lines; depends on PR 1 |

---

## Phase 1: Migration & Schema

- [x] 1.1 Write `internal/migrations/20260613000000_finance.sql` — `finance_transactions` (13 cols, CHECK kind/recurrence), `finance_tags` junction, `finance_fts` FTS5 (notes, account), 4 indices, `updated_at` trigger, 3 FTS sync triggers. IF NOT EXISTS on tables/indices/triggers. Down section with DROP in reverse order.
  - Files: `internal/migrations/20260613000000_finance.sql` (NEW)
  - Depends: —
  - Acceptance: `goose up` on fresh DB creates all objects; CHECK rejects `kind='transfer'`; FTS sync works on insert/update/delete; indices exist. Specs: finance-migration (all 6 scenarios).
  - Tests: `internal/finance/migration_test.go` — verify schema, constraints, FTS sync (3 scenarios), indices. ~60 LOC.

## Phase 2: sqlc Queries & Store

- [x] 2.1 Write `internal/store/queries/finance.sql` — queries: `CreateFinanceTransaction`, `GetFinanceTransaction`, `ListFinanceTransactions`, `UpdateFinanceTransaction`, `SoftDeleteFinanceTransaction`, `RestoreFinanceTransaction`, `PurgeFinanceTransaction`, `AttachTagToFinance`, `DetachAllTagsFromFinance`, `CountFinanceTransactions`, `ListFinanceByMonth`, `TopCategoriesByMonth`. Add sqlc.yaml overrides for `deleted_at`/`notes` (NullString), `category_id` (NullInt64).
  - Files: `internal/store/queries/finance.sql` (NEW), `sqlc.yaml` (MOD)
  - Depends: 1.1
  - Acceptance: SQL parses; annotations correct. Specs: finance-service (query contract).

- [x] 2.2 Run `sqlc generate` → `internal/store/finance.sql.go`, `models.go`, `querier.go`. Add `ListFinanceFiltered` hand-written dynamic WHERE in `internal/store/list.go` (mirrors `ListTodosFiltered`). Extend `DeleteOrphanTags` in `internal/store/queries/tags.sql` with `UNION SELECT DISTINCT tag_id FROM finance_tags`. Re-run `sqlc generate`.
  - Files: `internal/store/finance.sql.go` (GEN), `internal/store/models.go` (GEN), `internal/store/querier.go` (GEN), `internal/store/list.go` (MOD +60), `internal/store/queries/tags.sql` (MOD +2)
  - Depends: 2.1
  - Acceptance: `go build ./internal/store/...` clean; `make sqlc` no drift. Specs: finance-service (List filter).

## Phase 3: Service & Domain Package

- [x] 3.1 Create `internal/finance/domain.go` — `Kind` (expense/income), `Recurrence` (none/daily/weekly/monthly) enums with `Valid()`, `CreateInput`, `Filter`, `ExportRow` structs.
  - Files: `internal/finance/domain.go` (NEW ~80 LOC)
  - Depends: 2.2
  - Acceptance: `go build ./internal/finance/...` clean. Specs: finance-service (Transaction type).

- [x] 3.2 RED: Write `internal/finance/service_test.go` — table-driven tests: Create happy path (expense+income), Create defaults, invalid kind, invalid recurrence, Get found/not-found, Update changes fields+tags, SoftDelete idempotent, Restore idempotent, Purge cascades tags+FTS, List filter combos (date/kind/tag/trashed), Export resolves categories+tags. Use `newTestDB(t)`.
  - Files: `internal/finance/service_test.go` (NEW ~700 LOC)
  - Depends: 3.1
  - Acceptance: `go test ./internal/finance/...` FAILS (compile errors). Specs: finance-service (all 12 scenarios).

- [x] 3.3 GREEN: Create `internal/finance/attacher.go` (mirror `todos/attacher.go`, ~39 LOC). Create `internal/finance/service.go` — `New()`, `Create` (read KeyCurrency, insert, WithTags), `Get`, `Update` (detach-all+reattach, pruneOrphans), `SoftDelete`, `Restore`, `Purge` (WithTx + cascade), `List` (delegate ListFinanceFiltered), `Export` (resolve category names + tags → ExportRow).
  - Files: `internal/finance/attacher.go` (NEW ~39 LOC), `internal/finance/service.go` (NEW ~230 LOC)
  - Depends: 3.2
  - Acceptance: `go test ./internal/finance/... -race -count=1` PASSES. Specs: finance-service (all scenarios).

- [x] 3.4 REFRACTOR: Verify attacher orphan cleanup covers `finance_tags` via tags.sql UNION from 2.2. Run full suite `go test ./... -race -count=1` — no regressions in resources/todos.
  - Files: — (verification)
  - Depends: 3.3
  - Acceptance: All existing tests green; orphan tags cleaned across all 3 domains.

## Phase 4: Provider & Today Integration

- [x] 4.1 RED+GREEN: Create `internal/today/providers/finance_test.go` — seed transactions, assert "this-month-spending" total+top3, "recent-transactions" limit 5, empty omission, timezone-aware month, error degradation, item mapping (Domain/Title/Subtitle/URL). Create `internal/today/providers/finance.go` — `FinanceProvider` with `Name()="finance"`, `Sections(ctx)` using `UserLocation(ctx,db)` for timezone.
  - Files: `internal/today/providers/finance.go` (NEW ~130 LOC), `internal/today/providers/finance_test.go` (NEW ~140 LOC)
  - Depends: 3.3
  - Acceptance: `go test ./internal/today/providers/... -race` PASSES. Specs: finance-provider (all 7 scenarios), today-providers (REQ-TP-07).

- [x] 4.2 Wire provider: register `FinanceProvider` in `internal/web/handlers.go` `newHandlers()` and `internal/tui/app.go` `New()`. Extend `internal/today/sections.go` `sectionOrder` with `"this-month-spending": 5, "recent-transactions": 6`. Extend `showAllURLFor` in `internal/today/today.go`.
  - Files: `internal/today/sections.go` (MOD +4), `internal/today/today.go` (MOD +4), `internal/web/handlers.go` (MOD +2), `internal/tui/app.go` (MOD +2)
  - Depends: 4.1
  - Acceptance: Today view shows finance sections when data exists; sections omitted when empty; ordering correct. Specs: today-view (REQ-TV-03 modified).

## Phase 5: CLI

- [x] 5.1 Create `internal/cli/finance.go` (parent, ~25 LOC) + `finance_add.go` (~100), `finance_list.go` (~120), `finance_show.go` (~80), `finance_edit.go` (~120), `finance_rm_restore_purge.go` (~160). Flags per design: `--date/--amount/--kind/--account/--cat/--tag/--notes/--recurrence/--json`. Purge requires `--yes` or TTY.
  - Files: `internal/cli/finance*.go` (NEW 6 files ~605 LOC)
  - Depends: 3.3
  - Acceptance: `arsenal finance add/list/show/edit/rm/restore/purge` work; `--json` valid; invalid kind errors. Specs: finance-cli (all 8 scenarios).

- [x] 5.2 Create `internal/cli/finance_export.go` (~120 LOC) — `export` subcommand, `--format csv`, `--output path`, filter flags. `encoding/csv` writer, RFC 4180 escaping. Register `newFinanceCmd()` in `internal/cli/root.go` (+3). Add completions in `internal/cli/completion.go` (+15).
  - Files: `internal/cli/finance_export.go` (NEW ~120 LOC), `internal/cli/root.go` (MOD +3), `internal/cli/completion.go` (MOD +15)
  - Depends: 5.1, 3.3 (Export method)
  - Acceptance: CSV header correct; tags comma-separated in quoted cell; `--output` writes file; empty export = header only. Specs: finance-csv-export (all 6 scenarios).

## Phase 6: TUI

- [x] 6.1 Create `internal/tui/finance.go` (~340 LOC) — `financeItem` (list.Item), `financeDetailModel` (viewport), state machine (`financeStateList/Detail/Trash/ConfirmDelete`), `updateFinance()` dispatcher, `loadFinanceCmd()`, keybindings (n/e/d/r/x/j/k/enter/Tab), status bar hints.
  - Files: `internal/tui/finance.go` (NEW ~340 LOC)
  - Depends: 3.3
  - Acceptance: Direct `Model.Update()` tests for keybindings pass. Specs: finance-tui (keybindings, detail).

- [x] 6.2 Wire `areaFinance` in `internal/tui/app.go`: replace `placeholderView("Finance…")` in `View()` (~line 442), add `case areaFinance: return a.updateFinance(msg)` in `Update()` (~line 308), add `case areaFinance: return loadFinanceCmd(…)` in `loadCurrentAreaCmd()` (~line 551). Status bar shows "Finance" + hints.
  - Files: `internal/tui/app.go` (MOD +15)
  - Depends: 6.1
  - Acceptance: Placeholder gone; Tab cycles to Finance; list renders; `j/k` navigate; `enter` opens detail. Specs: finance-tui (all 6 scenarios).

## Phase 7: Web

- [x] 7.1 Create `internal/web/finance.go` (~420 LOC) — 9 handlers: `listFinance`, `newFinanceForm`, `createFinance`, `showFinance`, `editFinanceForm`, `updateFinance`, `softDeleteFinance` (HTMX empty fragment), `restoreFinance` (HTMX card), `purgeFinance` (redirect). Add `financeVM` to `internal/web/viewmodel.go` (+30). Register routes via `h.financeRoutes(r)` in `internal/web/handlers.go` (+15).
  - Files: `internal/web/finance.go` (NEW ~420 LOC), `internal/web/viewmodel.go` (MOD +30), `internal/web/handlers.go` (MOD +15)
  - Depends: 3.3
  - Acceptance: All 9 routes return correct status; HTMX fragments swap correctly. Specs: finance-web (lifecycle routes).

- [x] 7.2 Create `internal/web/templates/finance.html` (~230 LOC) — list (card-based, filter controls), show (detail), form (create/edit reuse), card fragment (HTMX swap), empty state.
  - Files: `internal/web/templates/finance.html` (NEW ~230 LOC)
  - Depends: 7.1
  - Acceptance: All views render; filter controls work; empty state shown when no data. Specs: finance-web (list, show, empty).

- [x] 7.3 Add Finance sidebar entry in `internal/web/templates/layout.html` (+10) — link between Todos and Trash, count badge via `{{if gt .FinanceCount 0}}`. Extend `commonPage()` in `internal/web/handlers.go` with `FinanceCount` via single `COUNT(*)` query (+5). Add header nav entry.
  - Files: `internal/web/templates/layout.html` (MOD +10), `internal/web/handlers.go` (MOD +5)
  - Depends: 7.1
  - Acceptance: Badge shows count when >0, hidden when 0; count updates after HTMX actions. Specs: finance-web (sidebar badge).

- [x] 7.4 Write `internal/web/finance_test.go` (~100 LOC) — `httptest.NewServer` tests: GET /finance list, POST /finance create+redirect, GET /finance/{id} show, POST /finance/{id}/delete HTMX fragment, sidebar badge presence.
  - Files: `internal/web/finance_test.go` (NEW ~100 LOC)
  - Depends: 7.1–7.3
  - Acceptance: `go test ./internal/web/... -race` PASSES. Specs: finance-web (all 8 scenarios).

## Phase 8: Final Wiring & Docs

- [ ] 8.1 Run `go build ./...` clean. Run `go test ./... -race -count=1` — all green. Run `golangci-lint run ./...` clean. Run `make sqlc` — no drift.
  - Files: — (verification)
  - Depends: 7.4
  - Acceptance: All checks pass; no regressions in resources/todos/today.

- [ ] 8.2 Update `CHANGELOG.md` — add `[Unreleased] ### Added` section: 7 new finance capabilities, 2 AMENDs, migration, 4 surfaces. Update `docs/CONTINUE.md` resume guide with finance domain context.
  - Files: `CHANGELOG.md` (MOD ~20), `docs/CONTINUE.md` (MOD ~15)
  - Depends: 8.1
  - Acceptance: CHANGELOG documents finance domain; resume guide updated.

---

## Slice Strategies

Three viable strategies for splitting this change into chained PRs. All exceed 400 lines per PR — this is unavoidable given ~1150 LOC of tests that must ship with their code (strict TDD).

### Strategy A: Layer-first (3 PRs, stacked-to-main)

| PR | Scope | Est. lines | Review focus |
|----|-------|-----------|--------------|
| 1 | Phases 1–3: migration + sqlc + domain + service + attacher + tests | ~1600 | Schema correctness, service contract, tag lifecycle |
| 2 | Phases 4–5: provider + today + CLI + CSV export | ~900 | CLI surface, Today sections, CSV format |
| 3 | Phases 6–8: TUI + web + docs + verification | ~1400 | TUI keybindings, web routes, templates, sidebar |

- **Pros**: Foundation locked independently; PR 1 is the highest-risk review (schema is irreversible); each PR builds on a tested base.
- **Cons**: PR 1 has no user-visible output (only `go test` validates); PR 3 is still large.
- **Working binary at boundary**: PR 1 → `go test` green; PR 2 → `arsenal finance` CLI works; PR 3 → all surfaces complete.

### Strategy B: Surface-first (4 PRs, stacked-to-main)

| PR | Scope | Est. lines | Review focus |
|----|-------|-----------|--------------|
| 1 | Phases 1–2 + 3.1: migration + sqlc + domain types (no service logic) | ~500 | Schema, generated code, types |
| 2 | Phase 3 + 5: service + attacher + CLI + CSV (headless, full CLI works) | ~1600 | Service contract, CLI flags, CSV format |
| 3 | Phases 4 + 6: provider + today + TUI | ~850 | Today sections, TUI keybindings |
| 4 | Phase 7–8: web + sidebar + docs | ~950 | Web routes, templates, sidebar badge |

- **Pros**: PR 1 is small and reviewable; PR 2 delivers first user value (CLI); each PR after PR 1 has a working surface.
- **Cons**: PR 2 is still large (service + tests + CLI); 4 PRs means more merge overhead.
- **Working binary at boundary**: PR 1 → migration applies; PR 2 → CLI works; PR 3 → TUI works; PR 4 → all surfaces complete.

### Strategy C: Vertical-slice (3 PRs, feature-branch-chain)

| PR | Scope | Est. lines | Review focus |
|----|-------|-----------|--------------|
| 1 | Phases 1–3 + 5.1: migration + sqlc + service + CLI basic (add/list/show) | ~1800 | Foundation + minimum CLI |
| 2 | Phases 4–5.2 + 6: provider + CLI remaining + TUI | ~1100 | Today + full CLI + TUI |
| 3 | Phases 7–8: web + docs + verification | ~950 | Web surface |

- **Pros**: PR 1 delivers a working `arsenal finance add/list/show` end-to-end; balanced review focus; feature-branch-chain keeps integration clean.
- **Cons**: PR 1 is the largest; feature-branch-chain requires tracker branch discipline.
- **Working binary at boundary**: PR 1 → basic CLI works; PR 2 → TUI + Today work; PR 3 → all surfaces complete.

### Recommendation

**Strategy A (Layer-first, stacked-to-main)** is recommended because:
1. Schema + service is the highest-risk, most-irreversible work — locking it in PR 1 gives confidence.
2. Stacked-to-main fits the existing `develop`-based flow from phase 2 and phase 3.
3. 3 PRs balances review overhead vs. slice size.
4. Each PR has a clear architectural boundary (data → headless → UI).

If the team prefers smaller PR 1 and doesn't mind 4 PRs, **Strategy B** is a solid alternative.

---

## Spec Traceability

| Spec | Implementing Phases | Scenarios |
|------|-------------------|-----------|
| finance-migration | 1 | 6 |
| finance-service | 2, 3 | 12 |
| finance-cli | 5 | 8 |
| finance-csv-export | 5 | 6 |
| finance-tui | 6 | 6 |
| finance-web | 7 | 8 |
| finance-provider | 4 | 7 |
| today-providers (AMEND) | 4 | 6 |
| today-view (AMEND) | 4 | 3 |
| **Total** | | **56** |
