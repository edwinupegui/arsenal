# Exploration: v3.x-finance — Finance Domain End-to-End

## 1. What We Already Know

### Locked-In Decisions from ADR-0002

These are settled and MUST NOT be re-litigated in this change:

- **v3.0 scope = resources + todos + Today only (Change 1)**: Finance is deferred to v3.x as a separate minor release. Calendar is also deferred but gets its own exploration and change.
- **Provider registry (Change 4)**: The Today view is a registry of `Provider` implementations. When Finance ships, it registers a `FinanceProvider` — no change to the rendering layer. The `Provider` interface (`Name() + Sections(ctx)`) is already implemented and proven by `TodosProvider` and `ResourcesProvider`.
- **Per-domain FTS5 + UNION ALL (Change 5)**: Cross-domain search uses `UNION ALL` across per-domain `*_fts` virtual tables. Finance will add `finance_fts` to the union when it ships.
- **One SQLite DB, shared tags/categories (What stays)**: Single migration path, cross-domain transactions via `sqliteutil.WithTx`. Finance gets its own migration file (`2026MMdd000000_finance.sql`).
- **Shared domain helpers (Change 2)**: `domain.WithTags` and `Attacher` interface are validated by todos. Finance will reuse the same pattern with a `finance/attacher.go`.
- **Typed config catalog (Change 3)**: `internal/config/keys.go` already declares `KeyCurrency` with `EnumValues: ["USD", "EUR", "ARS", "BRL", "MXN", "GBP"]` and default `"USD"`. Finance will consume this key.
- **No daemon, no notifications (What stays)**: No background refresh, no OS-level notifications. Finance data is queried on demand.
- **Migrations per domain, ordered by timestamp (ADR-0001 Decision 4)**: Finance owns its own migration file. Disk is source of truth (ADR-0002 Change 8).

### Surface Patterns from Phase 2 and Phase 3

From the shipped codebase:

- **TUI area-switcher is wired**: `internal/tui/app.go` has `areaID` enum with `areaFinance` (placeholder). The `View()` method renders `placeholderView("Finance (coming soon — v3.x)", ...)`. Finance must replace this placeholder with a real sub-model, following the exact pattern of `areaTodos` → `todos.go`.
- **Area dispatch pattern**: `App.Update()` routes by `a.currentArea` — `areaTodos` goes to `updateTodos()`, `areaToday` goes to `updateToday()`. `areaFinance` currently hits the `default` case and returns `nil`. Finance must wire `areaFinance` to `updateFinance()`.
- **Web sidebar**: `layout.html` has Today, Resources, Todos, Trash entries. No Finance link. Finance needs a sidebar entry with a count badge (transaction count or balance).
- **`commonPage()` data source**: Queries `CountResources`, `CountOpenTodos`, `CountOverdueTodos`. Finance sidebar count will need its own query (e.g., `CountFinanceTransactions` or `CurrentMonthBalance`).
- **CLI subcommand registration**: `internal/cli/root.go` registers `newTodoCmd()` and `newTodayCmd()`. Finance will add `newFinanceCmd()` following the same pattern.
- **`domain.WithTags` reuse**: `internal/domain/with_tags.go` defines the `Attacher` interface. Finance needs `internal/finance/attacher.go` (mirror of `todos/attacher.go`).
- **Provider interface**: `internal/today/provider.go` defines `Provider` with `Name() string` and `Sections(ctx) ([]Section, error)`. `internal/today/providers/` has `todos.go` and `resources.go`. Finance will add `providers/finance.go`.
- **Migration filename pattern**: `internal/migrations/` has `20260502000001_init.sql`, `20260502000002_fts5.sql`, `20260608000001_config_table.sql`, `20260608000002_todos.sql`. Finance migration will follow `2026MMdd000000_finance.sql` pattern with a timestamp after the todos migration.

### Patterns Established

- Services own business logic, store owns queries, TUI/web/CLI are thin adapters.
- HTMX fragments for in-place updates (mark-done, star, soft-delete).
- Confirm modal pattern: `data-confirm` attribute on forms, JS intercepts submit.
- Status bar in TUI shows area name + key hints.
- Strict TDD: tests written before implementation for all new packages.
- `newTestDB(t)` pattern from resources/todos tests for integration tests.

---

## 2. Product Decisions That Need User Input

These are UX/product questions the user MUST answer before spec or design begins.

### Q1: Account Model

**Question**: Does Finance support multiple accounts (checking, savings, credit card, cash) or a single account pool?

**Recommended default**: **Free-text account field (no account table)** — a simple `account` text column on the `finance_transactions` table. Users type "banco nación", "tarjeta visa", "efectivo" etc. No FK, no account registry. Matches ADR-0001's original scope: "`account` (free-text)".

**Alternatives**:
- *Multi-account with FK to `accounts` table*: Normalized, enforceable, enables per-account balance queries. Cost: new table, new service, new CLI surface (`arsenal finance account add/list`), TUI filter, web filter. Overkill for v3.x personal scale.
- *Enum-based accounts*: Predefined set ("checking", "savings", "credit", "cash"). Simpler than FK but still constrains the user. No advantage over free-text at personal scale.

**Why it matters**: The account model determines schema complexity, query patterns, CLI flags, TUI filters, and web sidebar sections. Free-text is the fastest path to a working Finance domain. Multi-account is a v4+ feature per ADR-0001 ("Not in scope: multi-account accounting").

### Q2: Transaction Types

**Question**: Just expenses + income? Or also transfers between accounts, refunds, adjustments?

**Recommended default**: **Expense + Income only** — two values for the `kind` field. Matches ADR-0001's original scope: "`kind` (expense/income)". Transfers require double-entry bookkeeping (two rows per transfer, balance reconciliation), which is a v4+ concern.

**Alternatives**:
- *Expense + Income + Transfer*: Transfer creates two rows (debit from source, credit to destination). Requires `transfer_id` FK to link pairs. Doubles the row count for transfers. Adds complexity to balance queries. Overkill for personal finance at <10k rows.
- *Expense + Income + Refund + Adjustment*: Four types. Refund is just a negative expense (or positive income). Adjustment is a balance correction. Both can be modeled as income/expense with notes. Adding types without adding double-entry is cosmetic.

**Why it matters**: Transaction types drive the schema's `kind` CHECK constraint, the service's validation, the CLI's `--kind` flag values, and the Today view's section logic ("overdue bills" vs "recent income"). Two types keep it simple.

### Q3: Categories vs Tags

**Question**: Finance has natural categories (food, transport, services). Does it use the existing `categories` table (shared with resources/todos) or its own? Tags on top, or exclusive?

**Recommended default**: **Shared `categories` table + tags via `domain.WithTags`** — exactly the same pattern as todos. Finance transactions get a `category_id` FK to the shared `categories` table, and tags attach via the `finance_tags` junction table. The existing `DeleteOrphanTags` UNION in `resources/attacher.go` must be extended to cover `finance_tags`.

**Alternatives**:
- *Separate `finance_categories` table*: Isolates finance categories from resource/todo categories. Cost: duplicate category management UI, duplicate CLI commands, separate migration. No benefit at personal scale — the user wants "alimentación" to appear everywhere.
- *Categories only, no tags*: Finance transactions have categories but no tags. Simpler, but loses the cross-domain tag namespace. A tag like "urgente" can't apply to both a todo and an expense. Breaks the shared-tags design from ADR-0001.

**Why it matters**: Category/tag strategy affects schema (FK relationships), the attacher pattern, orphan cleanup, FTS5 indexing, and the web sidebar. The shared model is already proven by todos and costs nothing extra.

### Q4: Recurrence

**Question**: How do recurring transactions work — auto-create next occurrence on mark-paid, or just template the user duplicates manually?

**Recommended default**: **Metadata-only placeholder** — the `recurrence` field is stored and displayed (like todos), but no auto-expansion happens. This matches the phase-2 decision for todos: "recurrence is metadata only; no auto-expansion." ADR-0001 says "simple recurrence only, no RRULE" for calendar, and the same constraint applies to finance.

**Alternatives**:
- *Auto-create on mark-paid*: When a recurring expense is marked as paid, the next occurrence is auto-created with the same amount/category/account. Requires a `next_due` computed field and a "mark paid" action. More useful, but adds service complexity and edge cases (what if amount changes? what if the user skips a month?).
- *Template + manual duplicate*: The user creates a "template" transaction and manually duplicates it each month. No automation, no edge cases. The template is just a regular transaction with a `recurrence` field.

**Why it matters**: Recurrence drives the `recurrence` CHECK constraint, the Today view's "upcoming bills" section logic, and whether the service needs a "mark paid" action vs just "create". Metadata-only is the safest v3.x scope.

### Q5: Currency Handling

**Question**: Single currency per user, or multi-currency with conversion?

**Recommended default**: **Single currency from `KeyCurrency` config** — all transactions use the currency set in `configstore`. The `currency` column on `finance_transactions` is populated from config at create time (denormalized for display). ADR-0002 Change 3 already declares `KeyCurrency` with `EnumValues: ["USD", "EUR", "ARS", "BRL", "MXN", "GBP"]` and default `"USD"`.

**Alternatives**:
- *Multi-currency with manual conversion*: Each transaction has its own currency. Balance queries require a conversion table or manual rates. Adds a `currency` FK, rate storage, and conversion logic. Overkill for personal use.
- *Multi-currency with live rates*: Fetch exchange rates from an API. Requires network access, rate caching, and offline fallback. Violates the local-first design.

**Why it matters**: Currency handling determines whether the schema needs a `currency` column per transaction (single-currency: yes, for display) or a `currency_id` FK with a rates table (multi-currency: no, not in v3.x). Single currency is the v3.x scope per ADR-0001 ("Not in scope: multi-currency in same DB").

### Q6: FinanceProvider in Today View

**Question**: What sections does the Finance provider contribute to the Today view?

**Recommended default**: **2 sections: "This Month's Spending" and "Recent Transactions"** — the first shows a summary of the current month's expenses (total + top 3 categories), the second shows the 5 most recent transactions. Both match the Today view's "what's actionable today" framing.

**Alternatives**:
- *Overdue bills + This month's spending + Recent transactions*: "Overdue bills" requires a `due_date` on transactions (recurring bills with a next-due date). This adds a column and query complexity. Defer to v4 when recurrence auto-expansion ships.
- *Budget alerts + This month's spending*: "Budget alerts" requires a budgets table and threshold logic. Explicitly out of scope per ADR-0002.
- *This week's spending + This month's spending*: Two time-range summaries. Redundant — "this month" subsumes "this week."

**Why it matters**: The FinanceProvider's sections determine what data the Today view renders for finance. Two sections keep the provider lightweight and avoid overloading the dashboard. The section keys must be added to `sectionOrder` in `internal/today/sections.go`.

### Q7: CSV Export Shape

**Question**: Per-transaction CSV (date, account, category, amount, note)? Or monthly summary? Or both via separate commands?

**Recommended default**: **Per-transaction CSV** — one row per transaction with columns: `date, kind, amount, currency, account, category, notes, tags`. Export via `arsenal finance export --format csv`. Monthly summary can be derived from the per-transaction CSV by the user's spreadsheet tool.

**Alternatives**:
- *Monthly summary CSV*: Aggregated by month + category. Less data, but loses individual transaction detail. The user can't reconcile or audit.
- *Both via separate commands*: `arsenal finance export --transactions` and `arsenal finance export --summary`. Two commands, two code paths, two templates. More work for marginal benefit.

**Why it matters**: CSV export is the primary data portability feature for finance. Per-transaction is the most flexible format — users can pivot, filter, and chart in any spreadsheet tool. ADR-0001 already committed to "Export: CSV (more useful than markdown for finance)."

---

## 3. Technical Decisions Still to Make

These are ONLY decidable AFTER the product questions above are resolved.

### T1: Schema Design (tables, indices, FK relationships)

**Depends on**: Q1 (account model), Q2 (transaction types), Q3 (categories/tags), Q5 (currency).

The `finance_transactions` table needs columns for: `id`, `date` (ISO date), `amount` (REAL), `kind` (expense/income CHECK), `account` (text, nullable), `category_id` (FK → categories, nullable), `notes` (text, nullable), `recurrence` (none/daily/weekly/monthly CHECK), `currency` (text, from config), `created_at`, `updated_at`, `deleted_at`.

Indices: `idx_finance_date`, `idx_finance_kind`, `idx_finance_deleted`, `idx_finance_category`.

Junction tables: `finance_tags` (same pattern as `todo_tags`).

FTS5: `finance_fts` virtual table on `notes, account, tags` with sync triggers.

The `categories` table may need a `domain` column or the existing shared model holds. Current `categories` table has no domain scoping — categories are global. This is fine for personal use.

### T2: Service Surface (Go API)

**Depends on**: Q1-Q5 (all schema decisions).

Mirrors `internal/todos/service.go`. Methods: `Create`, `Get`, `Update`, `SoftDelete`, `Restore`, `Purge`, `List` (with filter), `Export` (CSV). The `CreateInput` struct needs: `Date`, `Amount`, `Kind`, `Account`, `CategoryID`, `Notes`, `Recurrence`, `Tags`.

### T3: FinanceProvider Interface Contract

**Depends on**: Q6 (Today view sections).

Follows `internal/today/providers/todos.go` pattern. `FinanceProvider` implements `today.Provider` with `Name() = "finance"` and `Sections(ctx)` returning 2 sections. Must be registered in `today.Service` alongside existing providers.

### T4: TUI area-finance Design

**Depends on**: T1-T2 (schema and service).

Mirrors `internal/tui/todos.go`. The `areaFinance` placeholder in `app.go` is replaced with `updateFinance()` and `financeModel`. Keybindings: `x` (mark paid?), `d` (soft-delete), `enter` (detail), `n` (new transaction). Status bar hints update for finance context.

### T5: Web /finance Route + Sidebar Entry

**Depends on**: T1-T2 (schema and service).

Mirrors `/todos` routes. New file `internal/web/finance.go` with handlers: `listFinance`, `newFinanceForm`, `createFinance`, `showFinance`, `editFinanceForm`, `updateFinance`, `softDeleteFinance`, `restoreFinance`, `purgeFinance`. Sidebar entry in `layout.html` with count badge. `commonPage()` extended with finance count.

### T6: CLI `arsenal finance` Subcommand

**Depends on**: T2 (service surface).

Mirrors `internal/cli/todo.go`. Parent command `newFinanceCmd()` with subcommands: `add`, `list`, `show`, `edit`, `rm`, `restore`, `purge`, `export`. Flags: `--date`, `--amount`, `--kind`, `--account`, `--cat`, `--tag`, `--notes`, `--recurrence`, `--json`.

### T7: CSV Export Format and Command

**Depends on**: Q7 (CSV shape).

`arsenal finance export --format csv` writes to stdout or a file (`--output`). Columns: `date,kind,amount,currency,account,category,notes,tags`. The service method `Export(ctx, filter) ([]TransactionRow, error)` returns data; the CLI adapter formats as CSV.

### T8: Migration Filename

**Depends on**: T1 (schema).

Following the pattern `2026MMdd000000_finance.sql`. The next available timestamp after `20260608000002_todos.sql` should be `20260613000000_finance.sql` (today's date). The migration creates: `finance_transactions`, `finance_tags`, `finance_fts`, sync triggers, and the `v_finance_tags` aggregating view.

---

## 4. Out of Scope (v3.x Finance, this change)

These items are explicitly excluded from the Finance change. Do NOT design or implement them.

- **Calendar domain** — its own exploration, proposal, and change.
- **Multi-currency conversion** — single currency from `KeyCurrency` only.
- **Budgets and budget alerts** — no budget table, no threshold logic.
- **Investment tracking** — no portfolio, no stock/crypto.
- **Reconciliation with bank statements** — no import, no matching.
- **Mobile/web app** — CLI + TUI + local web server only.
- **iCal/calendar export** — that's Calendar's responsibility.
- **Recurrence auto-spawning** — metadata-only placeholder; auto-expansion deferred to v4.
- **FinanceProvider for Calendar** — Calendar gets its own provider in its own change.
- **Transfer transactions** — double-entry bookkeeping is v4+.
- **Multi-account with FK** — free-text account field only.
- **Live exchange rates** — single currency, no network dependency.
- **Overdue bills section in Today** — requires `due_date` on transactions; defer to v4 when recurrence auto-expansion ships.
- **Budget alerts section in Today** — requires budgets table; out of scope.
- **Daemon / background refresh** — no goroutines, no timers, no OS notifications.

---

## 5. Risks & Open Questions

### R1: Schema Migration Is Irreversible

The `finance_transactions` table, `finance_tags` junction, and `finance_fts` virtual table are created in a goose migration. Once applied, the migration cannot be rolled back without manual SQL. **Mitigation**: design the schema carefully in the spec phase. No destructive changes. The migration is additive only.

### R2: Multi-Account Queries Are 2× Cost

Even with free-text accounts, listing by account requires a full scan or an index on the `account` column. At <10k rows this is negligible. **Mitigation**: add `idx_finance_account` index. If the user has >100 distinct accounts, the index helps. If <10, it's unused overhead — acceptable.

### R3: CSV Export Needs Explicit Format Spec

The column order, delimiter (comma vs semicolon for locale), encoding (UTF-8), and header row must be specified before implementation. **Mitigation**: define the CSV format in the spec phase. Default: UTF-8, comma-delimited, header row included.

### R4: TUI area-finance Placeholder Must Not Break Area Switcher

The `areaFinance` placeholder is currently wired in `app.go` (line 442: `body = placeholderView("Finance (coming soon — v3.x)", ...)`). Replacing it with a real sub-model must not break the Tab/Shift+Tab cycling or the 1-5 jump. **Mitigation**: follow the exact pattern of `areaTodos` — add `updateFinance()`, `financeModel`, `financeViewState`, and wire into the `Update()` and `View()` switch statements.

### R5: `commonPage()` Expansion Risk

Adding finance counts to `commonPage()` (sidebar badge) could add latency to every web page if the query is expensive. **Mitigation**: use a lightweight `COUNT(*)` query (same pattern as `CountOpenTodos`). The Today handler already computes its own data independently; the finance handler should do the same.

### R6: Attacher Orphan Cleanup Must Cover `finance_tags`

The existing `DeleteOrphanTags` in `resources/attacher.go` already covers `todo_tags` in its UNION. It must be extended to cover `finance_tags` too. **Mitigation**: add `finance_tags` to the UNION in the same PR. Low risk — one-line change in the SQL.

### R7: `KeyCurrency` EnumValues May Need Expansion

The current `KeyCurrency` catalog has `["USD", "EUR", "ARS", "BRL", "MXN", "GBP"]`. If the user needs a currency not in this list, the config validation will reject it. **Mitigation**: confirm with user during Q5 resolution. Add more currencies if needed before spec phase.

---

## 6. Recommended Path Forward

```
sdd-explore (this document)
    ↓
sdd-propose  →  User answers Q1-Q7, confirms out-of-scope
    ↓
sdd-spec     →  New specs for finance/{service,cli,tui,web} + AMEND for today-providers
    ↓
sdd-design   →  Schema, service, provider, TUI/web rendering, CSV format
    ↓
sdd-tasks    →  Task breakdown with 400-line budget guard
    ↓
sdd-apply    →  Implementation in work units
    ↓
sdd-verify   →  Tests prove specs match implementation
    ↓
sdd-archive  →  Merge deltas into main specs
```

**What the orchestrator should tell the user**: "Before we design the Finance domain, I need answers to 7 product questions about account model, transaction types, categories, recurrence, currency, Today view sections, and CSV export. Each has a recommended default — you can accept all defaults or override any."

---

## 7. Forecast: Change Size

| Metric | Estimate |
|--------|----------|
| Estimated LOC | 1500–2200 (service ~400, CLI ~300, TUI ~250, web ~400, provider ~100, tests ~300, migration ~100, config/sidebar ~50) |
| Surface count | 3 (CLI, TUI, web) + 1 provider + 1 export |
| New files | ~12: `finance/domain.go`, `finance/attacher.go`, `finance/service.go`, `finance/service_test.go`, `cli/finance.go`, `tui/finance.go`, `web/finance.go`, `web/templates/finance.html`, `today/providers/finance.go`, `today/providers/finance_test.go`, `store/queries/finance.sql`, `migrations/20260613000000_finance.sql` |
| Modified files | ~8: `tui/app.go`, `web/handlers.go`, `web/templates/layout.html`, `cli/root.go`, `cli/completion.go`, `today/sections.go`, `resources/attacher.go` (orphan cleanup), `config/keys.go` (if currency list changes) |
| 400-line budget risk | High (3.75–5.5× over) — requires chained PRs or size exception |
| Spec count | ~5 new specs (finance-lifecycle, finance-cli, finance-tui, finance-web, finance-export) + 1 AMEND (today-providers) |
| Scenario count | ~60–80 (extrapolating from phase-2's 82 scenarios) |

---

## 8. Exploration: v3.x-finance

### Current State
The TUI has a placeholder `areaFinance` that renders "Finance (coming soon — v3.x)". The web has no Finance route or sidebar entry. The `KeyCurrency` config key exists with 6 currency options. The Today view has a Provider registry ready for a `FinanceProvider`. The `domain.WithTags` helper and `Attacher` interface are proven by todos. No finance tables, queries, or service code exist.

### Affected Areas
- `internal/tui/app.go` — Wire `areaFinance` to `updateFinance()`, replace placeholder
- `internal/tui/today.go` — No change needed (Today is independent)
- `internal/web/handlers.go` — New `/finance` route, `commonPage()` sidebar Finance entry
- `internal/web/templates/layout.html` — Finance link in sidebar + header nav
- `internal/finance/` (new package) — domain types, service, attacher, tests
- `internal/store/queries/finance.sql` — sqlc queries for finance
- `internal/migrations/20260613000000_finance.sql` — schema migration
- `internal/today/providers/finance.go` — FinanceProvider implementation
- `internal/today/sections.go` — Add finance section keys to `sectionOrder`
- `internal/cli/finance.go` — CLI finance subcommand
- `internal/cli/root.go` — Register `newFinanceCmd()`
- `internal/resources/attacher.go` — Extend `DeleteOrphanTags` UNION to cover `finance_tags`
- `internal/config/keys.go` — Possibly expand `KeyCurrency` EnumValues

### Approaches

1. **Domain-first (recommended)** — Build the migration, sqlc queries, domain types, and service first. Then wire CLI, TUI, web, and provider on top. Matches the phase-2 pattern exactly.
   - Pros: Proven pattern, testable in isolation, service validates before UI work
   - Cons: More upfront code before first visible result
   - Effort: Medium

2. **UI-first** — Build the TUI placeholder-to-real + web route with mock data, then fill in the service. Faster first visible result.
   - Pros: Validates UX before committing to schema
   - Cons: Refactoring cost when the real service lands, risk of designing the API around the UI
   - Effort: Low initially, Medium overall (refactor tax)

### Recommendation
Domain-first. The phase-2-todos pattern is proven and the service layer is the foundation for all three surfaces. The UI-first approach saves a day upfront but costs two days of refactoring when the schema and service land.

### Risks
- Schema migration is irreversible — design carefully
- CSV export needs an explicit format spec before implementation
- The TUI area-finance placeholder replacement must not break the area switcher
- `commonPage()` expansion could add latency if not kept lightweight
- Attacher orphan cleanup must cover `finance_tags` to avoid orphaned tag rows

### Ready for Proposal
Yes — pending user answers to Q1-Q7 (product questions). The orchestrator should present the 7 questions with recommended defaults and ask the user to accept or override.
