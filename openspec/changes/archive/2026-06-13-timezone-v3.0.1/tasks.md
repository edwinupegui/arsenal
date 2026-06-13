# Tasks: Timezone fix (ADR-0003)

## Phase 1: Config & Helper

- [x] 1.1 Add `KeyUserTimezone` to `internal/config/keys.go` (default `"UTC"`, TypeString, no enum)
- [x] 1.2 Add `configstore` tests for `KeyUserTimezone` default (`internal/configstore/keys_test.go`)
- [x] 1.3 Add `UserLocation` helper in `internal/today/user_location.go`
- [x] 1.4 Add helper tests in `internal/today/user_location_test.go` (unset=UTC, valid=loc, invalid=UTC+logs)

## Phase 2: Call Sites

- [x] 2.1 Replace `time.Now().UTC()` in `internal/today/providers/todos.go` via `UserLocation`
- [x] 2.2 Replace `time.Now().UTC()` in `internal/todos/service.go` via `UserLocation`
- [x] 2.3 Replace `time.Now().UTC()` in `internal/web/todos.go` (overdue badge + list filter)
- [x] 2.4 Replace `date('now')` in `internal/web/handlers.go` with Go-bound parameter via `UserLocation`

## Phase 3: Acceptance Tests

- [x] 3.1 Add overdue timezone test in `internal/todos/service_test.go`
- [x] 3.2 Add overdue badge timezone test in `internal/web/todos_test.go`

## Phase 4: Docs

- [x] 4.1 Remove "Timezone handling" from `CHANGELOG.md` Known limitations

---

**Review Workload Forecast**
- Estimated changed lines: ~50 LOC + ~6 tests
- 400-line budget risk: Low
- Chained PRs recommended: No
- Decision needed before apply: No
- Mode: commit directly to `develop` (small isolated fix)
