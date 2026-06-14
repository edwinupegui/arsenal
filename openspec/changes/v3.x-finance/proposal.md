# Proposal: v3.x-finance — Finance Domain End-to-End

## Why

Finance is the first v3.x deferred domain per [ADR-0002 Change 1](../../docs/adr/0002-v3-replan.md). v3.0 shipped Resources + Todos + Today. The Provider registry, shared `domain.WithTags`, typed `configstore`, and TUI area-switcher placeholders are all proven. Finance plugs in without architectural rework — just one new domain following the exact pattern validated by todos.

All 7 product decisions are **locked** (user accepted explore defaults): free-text account, expense+income only, shared categories + tags, metadata-only recurrence, single currency from `KeyCurrency`, 2 FinanceProvider sections, per-transaction CSV export. See [explore.md](./explore.md) for rationale.

## What Changes

### New Capabilities (7)

| Capability | Scope |
|---|---|
| `finance-service` | Domain types, `Service` (Create/Get/Update/SoftDelete/Restore/Purge/List/Export), attacher via `domain.WithTags` |
| `finance-cli` | `arsenal finance` subcommand: `add`, `list`, `show`, `edit`, `rm`, `restore`, `purge`, `export` |
| `finance-tui` | Replace `areaFinance` placeholder with real sub-model (`updateFinance`, keybindings, status bar) |
| `finance-web` | `/finance` route, sidebar entry with count badge, all CRUD + restore/purge handlers + template |
| `finance-provider` | `FinanceProvider` implements `today.Provider`, 2 sections: "This Month's Spending" + "Recent Transactions" |
| `finance-csv-export` | `arsenal finance export --format csv` → stdout or `--output`; columns: date, kind, amount, currency, account, category, notes, tags |
| `finance-migration` | `20260613000000_finance.sql`: `finance_transactions`, `finance_tags`, `finance_fts` (FTS5), sync triggers, indices |

### AMEND Capabilities (2)

| Capability | Change |
|---|---|
| `today-providers` | Registry now includes `FinanceProvider`; section keys `this-month-spending` and `recent-transactions` added |
| `today-view` | `sectionOrder` map in `internal/today/sections.go` extended with finance section keys after existing entries |

> **Note**: `DeleteOrphanTags` UNION extension in `internal/store/queries/tags.sql` is an implementation change, not a spec AMEND. No existing spec covers the attacher's orphan cleanup behavior.

## Impact

### Affected Specs

| Spec | Action |
|---|---|
| `finance-service` | NEW |
| `finance-cli` | NEW |
| `finance-tui` | NEW |
| `finance-web` | NEW |
| `finance-provider` | NEW |
| `finance-csv-export` | NEW |
| `finance-migration` | NEW |
| `today-providers` | AMEND — registry + section keys |
| `today-view` | AMEND — sectionOrder entries |
| `today-empty-state` | — no change |
| `today-tui` | — no change (Finance is a separate TUI area, not a Today sub-view) |
| `today-web` | — no change |
| `todo-lifecycle` | — no change |
| `todo-listing` | — no change |
| `todo-search` | — no change |
| `todo-status` | — no change |
| `todo-tags` | — no change |
| `todo-tui` | — no change |
| `todo-web` | — no change |
| `todo-cli` | — no change |
| `todo-recurrence-placeholder` | — no change |

### New Code Locations

| Path | Purpose |
|---|---|
| `internal/finance/domain.go` | Domain types (`Transaction`, `Kind`, `Recurrence`, `CreateInput`) |
| `internal/finance/attacher.go` | `Attacher` for `domain.WithTags` (mirrors `todos/attacher.go`) |
| `internal/finance/service.go` | `Service` with all lifecycle + export methods |
| `internal/finance/service_test.go` | Integration tests |
| `internal/cli/finance.go` | CLI subcommand tree |
| `internal/tui/finance.go` | TUI sub-model for `areaFinance` |
| `internal/web/finance.go` | HTTP handlers |
| `internal/web/templates/finance.html` | Web templates (list, show, form, partials) |
| `internal/today/providers/finance.go` | `FinanceProvider` |
| `internal/today/providers/finance_test.go` | Provider tests |
| `internal/store/queries/finance.sql` | sqlc queries |
| `internal/migrations/20260613000000_finance.sql` | Migration |

### Modified Code Locations

| Path | Change |
|---|---|
| `internal/store/queries/tags.sql` | Extend `DeleteOrphanTags` UNION: add `SELECT DISTINCT tag_id FROM finance_tags` |
| `internal/store/db.go` | Register finance queries |
| `internal/tui/app.go` | Wire `areaFinance` → `updateFinance()`; replace placeholder; add keybinding dispatch |
| `internal/tui/status.go` | Finance context hints in status bar |
| `internal/web/handlers.go` | Register `/finance/*` routes; extend `commonPage()` with finance count |
| `internal/web/templates/layout.html` | Sidebar Finance entry with count badge; header nav entry |
| `internal/cli/root.go` | Register `newFinanceCmd()` |
| `internal/cli/completion.go` | Finance completions |
| `internal/today/sections.go` | Add `this-month-spending` and `recent-transactions` to `sectionOrder` |

## Migrations

One new migration: `internal/migrations/20260613000000_finance.sql`. Creates:
- `finance_transactions` — columns: `id`, `date`, `amount` (REAL), `kind` (CHECK: expense/income), `account` (TEXT), `category_id` (FK→categories), `notes`, `recurrence` (CHECK: none/daily/weekly/monthly), `currency` (TEXT), `created_at`, `updated_at`, `deleted_at`
- `finance_tags` — junction table (FKs to `finance_transactions` + `tags`)
- `finance_fts` — FTS5 virtual table on `notes`, `account`, tags; sync triggers for insert/update/delete
- Indices: `idx_finance_date`, `idx_finance_kind`, `idx_finance_deleted`, `idx_finance_category`, `idx_finance_account`

Forward-only per ADR-0001 migration_policy. This is the **first** finance migration — zero existing data to lose. The migration uses `IF NOT EXISTS` guards on tables and triggers for safety.

## Rollout / Rollback

- **Feature flag**: Finance is opt-in by domain. No other domain depends on finance tables. TUI renders placeholder until migration is applied. Web sidebar entry hidden when no finance data exists.
- **Rollback**: Revert commits. If migration was applied but no user data exists, `DROP TABLE IF EXISTS finance_transactions, finance_tags, finance_fts` is safe. If user data exists, rolling back the migration requires a down-migration (documented in the migration file as a comment). Since this is v3.x fresh — no prior finance data — the risk is minimal.
- **Cross-domain isolation**: The Today view gracefully degrades when `FinanceProvider.Sections()` returns an error (same pattern as existing provider error handling per `today-view` REQ-TV-06).

## Scope Warning

**Forecast**: 1500–2200 LOC across 4 surfaces (CLI, TUI, Web, Provider) + service + migration + tests. This is **3.75–5.5× over the 400-line review budget guard**. This change WILL require chained PRs.

The orchestrator will present chain-strategy options after `sdd-tasks` produces the official forecast. Recommended split: domain-first (migration + service + attacher → CLI → TUI → Web → Provider → CSV export), each slice independently testable.

## Open Questions

None. All 7 product decisions accepted. No ambiguities found beyond what the exploration already resolved.

## References

- [ADR-0001: v3 scope](../../docs/adr/0001-v3-scope.md) — spine (single DB, shared tags, sub-areas, no daemon)
- [ADR-0002: v3 replan](../../docs/adr/0002-v3-replan.md) — Change 1 (deferred to v3.x), Change 4 (Provider registry), Change 5 (FTS5 UNION ALL)
- [explore.md](./explore.md) — full exploration, 7 product decisions, technical decisions T1-T8
- [phase-2-todos proposal](../archive/2026-06-11-phase-2-todos/proposal.md) — structural precedent (another end-to-end domain)
- [phase-3-today proposal](../archive/2026-06-11-phase-3-today/proposal.md) — precedent for cross-domain integration + provider AMENDs
