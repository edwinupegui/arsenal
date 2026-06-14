# Design: v3.x-finance — Finance Domain End-to-End

## Architecture Overview

Finance is the first new domain built on the v3.x spine defined in [ADR-0002](../../docs/adr/0002-v3-replan.md). It validates that the Provider registry (Change 4), `domain.WithTags` (Change 2), typed config catalog (Change 3), and TUI area-switcher (Change 6) work for a domain designed independently — not just for todos and resources which were co-designed in phases 2–3. The technical approach mirrors [`internal/todos/`](../../internal/todos/) exactly: a `Service` struct wrapping sqlc-generated queries, an `Attacher` implementing [`domain.Attacher`](../../internal/domain/with_tags.go), and domain types for enums and input shapes. All 7 locked product decisions from [proposal.md](./proposal.md) and [explore.md](./explore.md) are reflected in the schema and service contracts below.

The `FinanceProvider` registers into [`today.Service`](../../internal/today/today.go) alongside `TodosProvider` and `ResourcesProvider`, contributing two sections ("this-month-spending", "recent-transactions"). The [`sectionOrder`](../../internal/today/sections.go) map extends with two new keys. The data layer adds one migration, one sqlc query file, and one hand-written dynamic WHERE builder in [`store/list.go`](../../internal/store/list.go) following the `ListTodosFiltered` pattern. The `DeleteOrphanTags` UNION in [`tags.sql`](../../internal/store/queries/tags.sql) extends to cover `finance_tags`.

All three surfaces (CLI, TUI, web) are thin adapters over `finance.Service`. The TUI replaces the `areaFinance` placeholder in [`app.go`](../../internal/tui/app.go) (line 442). The web adds `/finance` routes and a sidebar count badge via a lightweight `COUNT(*)` in `commonPage()`. See the 9 spec files in [`specs/`](./specs/) for requirement-level contracts.

## Architecture Decisions

| Decision | Choice | Alternative | Rationale |
|----------|--------|-------------|-----------|
| Domain package shape | Mirror `internal/todos/` exactly | Extract shared domain base | 40-line attacher is minimal overhead; keeps domain boundary explicit ([phase-2 design](../archive/2026-06-11-phase-2-todos/design.md)) |
| FTS5 columns | `notes, account` only (no tags column) | Tags as 3rd FTS column (todos pattern) | [finance-migration spec](./specs/finance-migration/spec.md) specifies 2 columns; tag search via SQL JOIN is sufficient |
| Dynamic filter query | Hand-written `ListFinanceFiltered` in `store/list.go` | sqlc-generated dynamic query | Matches `ListTodosFiltered`/`ListResourcesFiltered` pattern; sqlc can't generate dynamic WHERE |
| Orphan tag cleanup | Extend UNION in `DeleteOrphanTags` | Move to shared `domain/orphan_tags.go` | One-line SQL change; no Go refactor needed for 3 domains |
| Currency denormalization | Read `KeyCurrency` at create time, store on row | Re-read config on every query | Historical transactions preserve original currency; matches [finance-service spec](./specs/finance-service/spec.md) |
| Section ordering | Finance after todos + resources | Interleave by recency | Matches [today-view delta](./specs/today-view/spec.md); "actionable → informational" flow |
| FTS5 IF NOT EXISTS | Omit guard (unsupported by SQLite) | Wrap in try/catch | `CREATE VIRTUAL TABLE` doesn't support `IF NOT EXISTS`; goose runs once; re-runs fail safely on duplicate |

## Layer Diagram

```
cmd/arsenal/main.go
  └─ internal/cli/root.go ─── register newFinanceCmd()
       └─ internal/cli/finance.go ─── add/list/show/edit/rm/restore/purge/export
            └─ internal/finance/service.go ─── Create/Get/Update/SoftDelete/Restore/Purge/List/Export
                 ├─ internal/finance/domain.go ─── Kind, Recurrence, CreateInput, Filter
                 ├─ internal/finance/attacher.go ─── domain.Attacher for finance_tags
                 ├─ internal/store/list.go ─── ListFinanceFiltered (hand-written dynamic SQL)
                 ├─ internal/store/queries/finance.sql ─── sqlc queries
                 └─ internal/store/*.sql.go ─── generated code

internal/tui/app.go ─── wire areaFinance → updateFinance()
  └─ internal/tui/finance.go ─── financeModel, financeItem, keybindings

internal/web/server.go ─── h.financeRoutes(r)
  ├─ internal/web/finance.go ─── CRUD handlers
  ├─ internal/web/templates/finance.html ─── list/form/detail templates
  └─ internal/web/templates/layout.html ─── sidebar Finance entry

internal/today/today.go ─── Register(FinanceProvider)
  ├─ internal/today/providers/finance.go ─── 2 sections
  └─ internal/today/sections.go ─── sectionOrder +2 keys

internal/migrations/20260613000000_finance.sql ─── schema
```

## Schema Design

File: `internal/migrations/20260613000000_finance.sql`

```sql
-- +goose Up
-- Finance domain (v3.x). Mirrors the todos table shape: amount, kind, soft-delete
-- timestamp, optional category_id FK, and a FTS5 virtual table.

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS finance_transactions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    date        TEXT    NOT NULL,
    amount      REAL    NOT NULL,
    kind        TEXT    NOT NULL CHECK (kind IN ('expense', 'income')),
    account     TEXT    NOT NULL DEFAULT '',
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    notes       TEXT,
    recurrence  TEXT    NOT NULL DEFAULT 'none' CHECK (recurrence IN ('none','daily','weekly','monthly')),
    currency    TEXT    NOT NULL DEFAULT 'USD',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at  TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS finance_tags (
    finance_id INTEGER NOT NULL REFERENCES finance_transactions(id) ON DELETE CASCADE,
    tag_id     INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (finance_id, tag_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_finance_date     ON finance_transactions(date);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_finance_kind     ON finance_transactions(kind);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_finance_deleted  ON finance_transactions(deleted_at);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_finance_category ON finance_transactions(category_id);
-- +goose StatementEnd

-- Auto-bump updated_at on every UPDATE.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_finance_updated_at
AFTER UPDATE ON finance_transactions
FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE finance_transactions
    SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- FTS5 virtual table on notes and account.
-- Note: CREATE VIRTUAL TABLE does not support IF NOT EXISTS in all SQLite builds.
-- +goose StatementBegin
CREATE VIRTUAL TABLE finance_fts USING fts5(
    notes,
    account,
    tokenize='unicode61 remove_diacritics 2'
);
-- +goose StatementEnd

-- Sync triggers for finance_fts.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_finance_fts_insert
AFTER INSERT ON finance_transactions
BEGIN
    INSERT INTO finance_fts(rowid, notes, account)
    VALUES (NEW.id, COALESCE(NEW.notes, ''), COALESCE(NEW.account, ''));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_finance_fts_update
AFTER UPDATE OF notes, account ON finance_transactions
BEGIN
    DELETE FROM finance_fts WHERE rowid = OLD.id;
    INSERT INTO finance_fts(rowid, notes, account)
    VALUES (NEW.id, COALESCE(NEW.notes, ''), COALESCE(NEW.account, ''));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_finance_fts_delete
AFTER DELETE ON finance_transactions
BEGIN
    DELETE FROM finance_fts WHERE rowid = OLD.id;
END;
-- +goose StatementEnd

-- +goose Down
-- Rollback: DROP TRIGGER, TABLE, INDEX in reverse order.
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_finance_fts_delete;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_finance_fts_update;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_finance_fts_insert;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS finance_fts;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_finance_updated_at;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_finance_category;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_finance_deleted;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_finance_kind;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_finance_date;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS finance_tags;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS finance_transactions;
-- +goose StatementEnd
```

**sqlc.yaml overrides** — add to existing `overrides` list:

```yaml
- column: "finance_transactions.deleted_at"
  go_type:
    import: "database/sql"
    type: "NullString"
- column: "finance_transactions.notes"
  go_type:
    import: "database/sql"
    type: "NullString"
- column: "finance_transactions.category_id"
  go_type:
    import: "database/sql"
    type: "NullInt64"
```

## Service Go API

```go
// internal/finance/domain.go
package finance

type Kind string
const (KindExpense Kind = "expense"; KindIncome Kind = "income")

type Recurrence string
const (
    RecurrenceNone    Recurrence = "none"
    RecurrenceDaily   Recurrence = "daily"
    RecurrenceWeekly  Recurrence = "weekly"
    RecurrenceMonthly Recurrence = "monthly"
)

type CreateInput struct {
    Date       string   // YYYY-MM-DD; defaults to today when empty
    Amount     float64
    Kind       Kind
    Account    string
    CategoryID *int64
    Notes      string
    Recurrence Recurrence
    Tags       []string
}

type Filter struct {
    From           *string // ISO date lower bound
    To             *string // ISO date upper bound
    Kind           *string
    CategorySlug   string
    TagName        string
    Trashed        bool
    Limit, Offset  int
}

type ExportRow struct {
    Date, Kind, Currency, Account, Category, Notes string
    Amount                                          float64
    Tags                                            []string
}
```

```go
// internal/finance/service.go
type Transaction struct {
    Row  store.FinanceTransaction
    Tags []string
}

type Service struct { db *sql.DB; q *store.Queries; now func() time.Time }

func New(db *sql.DB, opts ...Option) *Service
func (s *Service) Create(ctx, in CreateInput) (*Transaction, error)   // validates, reads KeyCurrency, inserts, attaches tags
func (s *Service) Get(ctx, id int64) (*Transaction, error)
func (s *Service) Update(ctx, id int64, in CreateInput) (*Transaction, error) // detach-all + re-attach with prune
func (s *Service) SoftDelete(ctx, id int64) error
func (s *Service) Restore(ctx, id int64) error
func (s *Service) Purge(ctx, id int64) error                          // DELETE + prune orphans in tx
func (s *Service) List(ctx, f Filter) ([]*Transaction, error)         // delegates to ListFinanceFiltered or SearchFinance
func (s *Service) Export(ctx, f Filter) ([]ExportRow, error)          // resolves category names + tags
```

**Attacher** (`internal/finance/attacher.go`): mirrors [`todos/attacher.go`](../../internal/todos/attacher.go) — `UpsertTag` delegates to `store.UpsertTag`, `AttachTagToOwner` calls `AttachTagToFinance`, `DeleteOrphanTags` delegates to shared query.

## FinanceProvider

```go
// internal/today/providers/finance.go
type FinanceProvider struct {
    queries *store.Queries
    db      *sql.DB
    now     func() time.Time
}

func NewFinanceProvider(db *sql.DB) *FinanceProvider
func (p *FinanceProvider) Name() string { return "finance" }
func (p *FinanceProvider) Sections(ctx context.Context) ([]today.Section, error)
// Returns up to 2 sections:
//   "this-month-spending" — total expenses + top 3 categories for current month
//   "recent-transactions"  — 5 most recent non-trashed transactions
// Uses today.UserLocation(ctx, db) for timezone-aware month boundaries.
// Item mapping: Domain="finance", Title=account+" ("+kind+")", Subtitle=formatted amount, URL="/finance/{id}"
```

**Registration**: add `todaySvc.Register(providers.NewFinanceProvider(db))` in `newHandlers()` ([`handlers.go`](../../internal/web/handlers.go):34) and `New()` ([`app.go`](../../internal/tui/app.go):169).

**Section ordering** — extend [`sections.go`](../../internal/today/sections.go):

```go
var sectionOrder = map[string]int{
    "overdue":              1,
    "due-today":            2,
    "upcoming":             3,
    "recent":               4,
    "this-month-spending":  5,
    "recent-transactions":  6,
}
```

**showAllURLFor** — extend in [`today.go`](../../internal/today/today.go):

```go
case "this-month-spending": return "/finance?kind=expense"
case "recent-transactions": return "/finance"
```

## TUI Design

`internal/tui/finance.go` mirrors [`todos.go`](../../internal/tui/todos.go):

- **`financeItem`** adapts `finance.Transaction` to `list.Item` — `Title()` shows `account ±amount currency`, `Description()` shows `kind · date · #tags`.
- **`financeDetailModel`** renders all fields via viewport (mirrors `todoDetailModel`).
- **State machine**: `financeViewState` enum (`financeStateList`, `financeStateDetail`, `financeStateTrash`, `financeStateConfirmDelete`).
- **App fields**: `financeService`, `financeList`, `financeDetail`, `financeConfirm`, `financeShowTrashed`, `financeState`.
- **`updateFinance()`**: dispatches by state; keybindings `n` (new), `e` (edit), `d` (soft-delete), `r` (restore in trash), `x` (purge), `j/k` (navigate), `enter` (detail), `Tab` (area switch).
- **`loadFinanceCmd()`**: calls `svc.List(ctx, Filter{Trashed, Limit: 500})`.
- **Status bar**: `keyStyle.Render("n")+" new  "+keyStyle.Render("e")+" edit  "+keyStyle.Render("d")+" del  "+keyStyle.Render("Tab")+" switch`.
- **Wire in `app.go`**: replace `placeholderView("Finance…")` in `View()` (line 442), add `case areaFinance: return a.updateFinance(msg)` in `Update()` (line 308), add `case areaFinance: return loadFinanceCmd(…)` in `loadCurrentAreaCmd()` (line 551).

## Web Design

`internal/web/finance.go` mirrors [`todos.go`](../../internal/web/todos.go):

| Method | Path | Handler | HTMX |
|--------|------|---------|------|
| GET | `/finance` | `listFinance` | No |
| GET | `/finance/new` | `newFinanceForm` | No |
| POST | `/finance` | `createFinance` | No |
| GET | `/finance/{id}` | `showFinance` | No |
| GET | `/finance/{id}/edit` | `editFinanceForm` | No |
| POST | `/finance/{id}` | `updateFinance` | No |
| POST | `/finance/{id}/delete` | `softDeleteFinance` | Yes (empty fragment) |
| POST | `/finance/{id}/restore` | `restoreFinance` | Yes (card fragment) |
| POST | `/finance/{id}/purge` | `purgeFinance` | No (redirect) |

**Route registration**: `h.financeRoutes(r)` in [`server.go`](../../internal/web/server.go) after `h.todoRoutes(r)`.

**Sidebar**: add Finance link in [`layout.html`](../../internal/web/templates/layout.html) between Todos and Trash (both in main layout and `sidebar-oob` fragment):

```html
<a href="/finance" class="sidebar-link {{if eq .Nav "finance"}}is-active{{end}}">
  <span class="sidebar-link-label">Finance</span>
  {{if gt .FinanceCount 0}}<span class="sidebar-link-count">{{.FinanceCount}}</span>{{end}}
</a>
```

**`commonPage()`**: add `FinanceCount int64` to `pageData`; compute via `SELECT COUNT(*) FROM finance_transactions WHERE deleted_at IS NULL` (same lightweight pattern as `CountOpenTodos`). Add `financeService *finance.Service` to `Handlers` struct.

**Header nav**: add `<a href="/finance">Finance</a>` in the `<nav>` block.

## CLI Design

`internal/cli/finance.go` mirrors [`todo.go`](../../internal/cli/todo.go):

```go
func newFinanceCmd() *cobra.Command  // parent: "arsenal finance"
// Subcommands:
//   add     --date --amount --kind --account --cat --tag --notes --recurrence [--json]
//   list    --from --to --kind --cat --tag --trashed [--json]
//   show    <id> [--json]
//   edit    <id> (same flags as add)
//   rm      <id>
//   restore <id>
//   purge   <id> [--yes]
//   export  --format csv [--output path] [--from --to --kind --cat --tag]
```

Register `root.AddCommand(newFinanceCmd())` in [`root.go`](../../internal/cli/root.go) after `newTodoCmd()`.

## CSV Export Format

```
date,kind,amount,currency,account,category,notes,tags
2026-06-10,expense,1500.00,ARS,banco nación,alimentación,"compras del supermercado","comida,hogar"
2026-06-09,income,50000.00,ARS,banco nación,sueldo,sueldo mensual,
```

- Header row mandatory. UTF-8, comma-delimited.
- RFC 4180: fields with commas, quotes, or newlines are double-quoted; internal quotes doubled.
- Tags comma-separated within cell (quoted when multiple).
- Empty export outputs header row only.
- `Service.Export()` returns `[]ExportRow`; CLI adapter formats via `encoding/csv`.

## Attacher Integration

Extend `DeleteOrphanTags` in [`tags.sql`](../../internal/store/queries/tags.sql):

```sql
-- name: DeleteOrphanTags :exec
DELETE FROM tags
WHERE id NOT IN (
    SELECT DISTINCT tag_id FROM resource_tags
    UNION
    SELECT DISTINCT tag_id FROM todo_tags
    UNION
    SELECT DISTINCT tag_id FROM finance_tags
);
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Migration | Tables, indices, FTS5 sync, CHECK constraints | `newTestDB(t)` + assert schema + insert/search/delete |
| Service | Create/Update/Delete/Restore/Purge/List/Export | Table-driven, filter combos, tag lifecycle, currency from config |
| Provider | Section construction, timezone, empty omission, error degradation | `newTestDB(t)` + seed data + assert sections |
| TUI | Keybindings, list/detail transitions | Bubbletea teatest |
| Web | Routes, sidebar count, HTMX fragments | `httptest.NewServer` + assert template output |
| CLI | Subcommand execution, `--json`, `--yes` | `cmd.ExecuteContext` + capture stdout |
| CSV | Format, escaping, filters, empty export | Table-driven with `encoding/csv` reader verification |

Strict TDD: tests written before implementation for all new packages. Pattern: `newTestDB(t)` from [`resources/service_test.go`](../../internal/resources/service_test.go).

## Migration / Rollout

- Forward-only per ADR-0001 migration policy.
- `IF NOT EXISTS` on tables, indices, triggers (except FTS5 virtual table — SQLite limitation).
- No data migration (finance tables don't exist in v3.0.1).
- Rollback: revert commits. `DROP TABLE IF EXISTS finance_transactions` cascades `finance_tags`. FTS5 dropped separately.
- For users upgrading from v3.0.1, goose applies the migration once on next `arsenal` invocation.

## Open Risks

- **1500–2200 LOC forecast** — chained PRs required. Recommended split: (1) migration + service + attacher, (2) CLI + CSV export, (3) TUI, (4) web + templates, (5) provider + today integration.
- **FTS5 `CREATE VIRTUAL TABLE` lacks `IF NOT EXISTS`** — re-running the migration will fail on the FTS5 statement. Goose tracks applied migrations, so this only affects manual re-runs.
- **`commonPage()` latency** — finance count must stay a single `COUNT(*)` query. If the sidebar grows more counts, consider batching.
- **Shared test fixtures** — `finance`, `today/providers`, and `web` packages all need seeded finance data. Consider extracting `testutil/finance.go` if duplication exceeds 3 call sites.

## Cross-References

- [ADR-0001](../../docs/adr/0001-v3-scope.md) — spine (single DB, shared tags, no daemon)
- [ADR-0002](../../docs/adr/0002-v3-replan.md) — Changes 1–6 (registry, helpers, config, TUI)
- [proposal.md](./proposal.md) — scope, 7 capabilities, 2 AMENDs
- [explore.md](./explore.md) — 7 product decisions (Q1–Q7), 8 technical decisions (T1–T8)
- [phase-2-todos design](../archive/2026-06-11-phase-2-todos/design.md) — closest structural precedent
- [phase-3-today design](../archive/2026-06-11-phase-3-today/design.md) — provider registry precedent
- 9 spec files in [`specs/`](./specs/)
