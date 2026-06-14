# Archive Report: v3.x-finance — Finance Domain End-to-End

**Date**: 2026-06-14
**Change**: v3.x-finance
**Status**: Archived and closed
**Artifact Store Mode**: openspec (file-based)

## Executive Summary

The `v3.x-finance` SDD change has been fully implemented and verified. All 8 phases completed, all 56 scenarios validated across 9 capability specs. All tasks marked complete. Spec deltas merged into main openspec specs. Change folder moved to archive.

## Completion Verification

- **All implementation tasks**: 8 phases, all marked [x] complete
- **All tests passing**: `go build ./... ✓`, `go test ./... -race -count=1 ✓` (18 packages), `go vet ./... ✓`, `sqlc` drift ✓
- **Verification report**: All critical and warning items resolved
- **Spec coverage**: 56 scenarios across 9 specs (7 new + 2 amended)

## Specs Merged to Main openspec

| Spec | Action | Details | Main Spec Path |
|------|--------|---------|-----------------|
| finance-service | Created | New capability spec, 12 scenarios | `openspec/specs/finance-service/spec.md` |
| finance-cli | Created | New capability spec, 8 scenarios | `openspec/specs/finance-cli/spec.md` |
| finance-csv-export | Created | New capability spec, 6 scenarios | `openspec/specs/finance-csv-export/spec.md` |
| finance-migration | Created | New capability spec, 6 scenarios | `openspec/specs/finance-migration/spec.md` |
| finance-provider | Created | New capability spec, 7 scenarios | `openspec/specs/finance-provider/spec.md` |
| finance-tui | Created | New capability spec, 6 scenarios | `openspec/specs/finance-tui/spec.md` |
| finance-web | Created | New capability spec, 8 scenarios | `openspec/specs/finance-web/spec.md` |
| today-providers | Amended | Added REQ-TP-07 (FinanceProvider), 6 scenarios | `openspec/specs/today-providers/spec.md` |
| today-view | Amended | Modified REQ-TV-03 (section ordering), 3 scenarios | `openspec/specs/today-view/spec.md` |

## Files Created in Main Specs Directory

```
openspec/specs/finance-service/spec.md
openspec/specs/finance-cli/spec.md
openspec/specs/finance-csv-export/spec.md
openspec/specs/finance-migration/spec.md
openspec/specs/finance-provider/spec.md
openspec/specs/finance-tui/spec.md
openspec/specs/finance-web/spec.md
```

## Files Modified in Main Specs Directory

```
openspec/specs/today-providers/spec.md
openspec/specs/today-view/spec.md
```

## Archive Contents

Archived folder: `openspec/changes/archive/2026-06-14-v3.x-finance/`

Artifacts preserved:
- `proposal.md` — scope, 7 new capabilities, 2 amended capabilities, rationale
- `design.md` — architecture overview, schema, service API, provider design, TUI/web/CLI design
- `tasks.md` — 8 phases, all [x] complete; review workload forecast; 3 chained PR strategies
- `specs/` — 9 spec files (7 new, 2 delta)
  - `finance-service/spec.md`
  - `finance-cli/spec.md`
  - `finance-csv-export/spec.md`
  - `finance-migration/spec.md`
  - `finance-provider/spec.md`
  - `finance-tui/spec.md`
  - `finance-web/spec.md`
  - `today-providers/spec.md` (delta)
  - `today-view/spec.md` (delta)

## Decisions & Tradeoffs

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Domain package shape | Mirror `internal/todos/` exactly | Proven pattern; minimal attacher overhead |
| FTS5 columns | `notes, account` only | 2-column design per finance-migration spec; tag search via JOIN |
| Dynamic filter query | Hand-written `ListFinanceFiltered` | Matches existing pattern; sqlc can't generate dynamic WHERE |
| Orphan tag cleanup | Extend UNION in `DeleteOrphanTags` | One-line SQL change; no Go refactor for 3 domains |
| Currency model | Single currency from `KeyCurrency` at create time | Historical transactions preserve original currency |
| Finance section order | After todos + resources (positions 5–6) | Actionable → informational flow |

## New Capabilities Delivered

1. **finance-service** — Domain types, Service (Create/Get/Update/SoftDelete/Restore/Purge/List/Export), attacher
2. **finance-cli** — `arsenal finance` subcommands (add/list/show/edit/rm/restore/purge/export)
3. **finance-csv-export** — Per-transaction CSV export with filter support
4. **finance-migration** — `20260613000000_finance.sql` (tables, indices, FTS5, triggers)
5. **finance-provider** — Today view integration (2 sections: this-month-spending, recent-transactions)
6. **finance-tui** — TUI area replacing placeholder (list/detail, keybindings, status bar)
7. **finance-web** — Web routes + sidebar count badge + templates

## Amended Capabilities

1. **today-providers** — Registry now includes FinanceProvider (REQ-TP-07 added)
2. **today-view** — Section ordering extended with finance sections (REQ-TV-03 modified)

## Implementation Summary

- **Lines of code**: ~3000–3600 (source ~2700, tests ~1150, generated ~215)
- **New packages**: `internal/finance/` (domain, service, attacher, tests)
- **New CLI package extensions**: `internal/cli/finance*.go` (6 files)
- **New TUI model**: `internal/tui/finance.go`
- **New web handlers**: `internal/web/finance.go` + `finance.html` template
- **New provider**: `internal/today/providers/finance.go` + tests
- **New migration**: `internal/migrations/20260613000000_finance.sql`
- **Modified files**: ~8 (tui/app.go, web/handlers.go, cli/root.go, today/sections.go, etc.)

## Testing & Validation

All tests passing (18 packages):
- `go build ./...` ✓
- `go test ./... -race -count=1` ✓
- `go vet ./...` ✓
- `sqlc` drift check ✓

Strict TDD applied throughout:
- Service tests cover Create/Update/Delete/Restore/Purge/List/Export scenarios
- Provider tests cover section logic, timezone handling, error degradation
- TUI tests cover keybindings, state transitions, detail view
- Web tests cover routes, sidebar counts, HTMX fragments
- CLI tests cover subcommand execution, flags, JSON output, CSV format

## Known Decisions & Notes

1. **FTS5 `CREATE VIRTUAL TABLE` lacks `IF NOT EXISTS`** — Re-running the migration manually will fail on the FTS5 statement. Goose tracks applied migrations, so this only affects manual re-runs.

2. **400-line budget exceeded** — Change size ~3000–3600 LOC (7.5–9× over budget). Chained PRs required as per proposal. Recommended strategy: layer-first (3 PRs): (1) migration + sqlc + domain + service ~1600 lines, (2) provider + today + CLI + CSV export ~900 lines, (3) TUI + web + docs + verification ~1400 lines.

3. **Shared test fixtures** — `finance`, `today/providers`, and `web` packages all seed finance data. Low duplication; no extraction needed.

4. **commonPage() latency** — Finance count query is lightweight (`COUNT(*)` per existing pattern). No performance concerns identified.

## Archive Validation Checklist

- [x] Main specs updated with 7 new capability specs
- [x] Main specs updated with 2 amended capability deltas
- [x] Change folder moved to archive with date prefix (2026-06-14)
- [x] Archive contains all SDD artifacts (proposal, design, tasks, specs)
- [x] Archived tasks.md shows no unchecked implementation tasks
- [x] Active changes directory no longer has this change
- [x] No CRITICAL issues remain in verification report

## Cross-References

- **Proposal**: `openspec/changes/archive/2026-06-14-v3.x-finance/proposal.md`
- **Design**: `openspec/changes/archive/2026-06-14-v3.x-finance/design.md`
- **Tasks**: `openspec/changes/archive/2026-06-14-v3.x-finance/tasks.md`
- **Specs**: `openspec/changes/archive/2026-06-14-v3.x-finance/specs/`
- **ADR-0001**: `docs/adr/0001-v3-scope.md` (spine)
- **ADR-0002**: `docs/adr/0002-v3-replan.md` (provider registry, Change 1)
- **Phase 2 (todos) precedent**: `openspec/changes/archive/2026-06-11-phase-2-todos/`
- **Phase 3 (today) precedent**: `openspec/changes/archive/2026-06-11-phase-3-today/`

## SDD Cycle Complete

The change has been fully planned (proposal), specified (9 specs), designed (architecture), tasked (8 phases), implemented (all surface code + service + migration), verified (all tests + coverage), and archived. Ready for team review and merge via the chained PR strategy.

No further work required. The next change is ready to start.
