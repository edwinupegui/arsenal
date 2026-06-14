# Design: v3.x-calendar — Calendar Domain End-to-End

## Architecture Overview

Calendar is the second new domain on the v3.x spine ([ADR-0002](../../docs/adr/0002-v3-replan.md) Change 1), the structural sibling of Finance. It validates nothing new architecturally — it reuses the proven domain pattern that Finance just shipped: a `Service` wrapping sqlc-generated queries, an `Attacher` implementing [`domain.Attacher`](../../internal/domain/with_tags.go), domain types for enums/inputs, a hand-written dynamic filter in [`store/list.go`](../../internal/store/list.go), an FTS5 virtual table with sync triggers, and a `today.Provider`. The only genuinely new surface area over Finance is in the **data semantics**: a datetime `start_at`, a nullable `end_at`, an `all_day` boolean, a `location` column, and an **iCal (RFC 5545) export** that replaces Finance's CSV export.

`CalendarProvider` registers into [`today.Service`](../../internal/today/today.go) alongside `TodosProvider`, `ResourcesProvider`, and `FinanceProvider`, contributing two sections (`events-today`, `events-upcoming`). The [`sectionOrder`](../../internal/today/sections.go) map extends with keys `7` and `8` (after finance's `5/6`); [`showAllURLFor`](../../internal/today/today.go) extends with two cases. The [`DeleteOrphanTags`](../../internal/store/queries/tags.sql) UNION extends to cover `calendar_tags`.

All three surfaces (CLI, TUI, web) are thin adapters over `calendar.Service`. The TUI replaces the `areaCalendar` placeholder in [`app.go`](../../internal/tui/app.go). The web adds `/calendar` routes and a sidebar count badge via a lightweight `COUNT(*)` in `commonPage()`.

All 6 locked product decisions (PQ1–PQ6 from [proposal.md](./proposal.md)) and the 8 technical decisions (T1–T8 from [explore.md](./explore.md)) are reflected below.

## Architecture Decisions

| Decision | Choice | Alternative | Rationale |
|----------|--------|-------------|-----------|
| Domain package shape | Mirror `internal/finance/` exactly: `event.go` (domain types), `attacher.go`, `service.go` | Extract shared domain base | ~40-line attacher is minimal overhead; the finance precedent is proven; no refactor justified for a 2nd consumer |
| `start_at` storage | Single TEXT column; `YYYY-MM-DDTHH:MM:SS` when timed, `YYYY-MM-DD` when `all_day=1` (PQ4) | Two columns (date + time) | One column maps cleanly to iCal DTSTART value type (DATE vs DATE-TIME); branch on `all_day` |
| `end_at` semantics | TEXT nullable; NULL = open-ended; maps to iCal DTEND (PQ1) | `duration_minutes`; both | Natural user input, direct DTEND map, no ambiguity |
| Recurrence | Metadata-only enum `none/daily/weekly/monthly/yearly`, no expansion (PQ2, PQ3) | Provider on-read expansion; materialize rows | Consistent with todos/finance; expansion is a non-breaking future add; `yearly` per ADR-0001 (birthdays/anniversaries) |
| FTS5 columns | `title, description, location` (3 cols) (PQ6) | 2 cols (`title, description`) | `location` is a natural calendar search target |
| iCal export | stdlib only; VCALENDAR + VEVENT; `recurrence`→RRULE; events only (PQ5, T8) | External ics library; include VTODO | No new dependency; VTODO complexity not worth it; domain boundary stays clean |
| Dynamic filter | Hand-written `ListCalendarFiltered` in `store/list.go` | sqlc-generated dynamic query | Matches `ListFinanceFiltered`; sqlc cannot generate dynamic WHERE |
| Orphan tag cleanup | Extend UNION in `DeleteOrphanTags` with `calendar_tags` (T6) | Shared `domain/orphan_tags.go` | One-line SQL change; no Go refactor for a 4th domain |
| Timezone | `today.UserLocation(ctx, db)` in `CalendarProvider`; `start_at` stored as local-time string without offset (T4, ADR-0003) | Store UTC + offset | Single-system-timezone assumption; reuses helper already used by FinanceProvider; spec documents reinterpretation risk |
| FTS5 `IF NOT EXISTS` | Omit guard on `CREATE VIRTUAL TABLE` (T3) | Wrap in try/catch | SQLite does not support it for virtual tables; goose runs once |
| Section ordering | Calendar after finance: `events-today: 7`, `events-upcoming: 8` | Interleave by recency | Matches `today-view` delta; preserves "actionable → informational" flow |

## Layer Diagram

```
cmd/arsenal/main.go
  └─ internal/cli/root.go ─── register newCalendarCmd()
       └─ internal/cli/calendar.go ─── add/list/show/edit/rm/restore/purge/export
            └─ internal/calendar/service.go ─── Create/Get/Update/SoftDelete/Restore/Purge/List/Export
                 ├─ internal/calendar/event.go ─── Recurrence enum, CreateInput, Filter, ExportRow, validateCreate
                 ├─ internal/calendar/attacher.go ─── domain.Attacher for calendar_tags
                 ├─ internal/calendar/ical.go ─── RFC 5545 writer (VCALENDAR/VEVENT/RRULE)
                 ├─ internal/store/list.go ─── ListCalendarFiltered (hand-written dynamic SQL)
                 ├─ internal/store/search.go ─── SearchCalendar (FTS5 MATCH)
                 ├─ internal/store/queries/calendar.sql ─── sqlc queries
                 └─ internal/store/*.sql.go ─── generated code

internal/tui/app.go ─── wire areaCalendar → updateCalendar()
  └─ internal/tui/calendar.go ─── calendarModel, calendarItem, keybindings

internal/web/server.go ─── h.calendarRoutes(r)
  ├─ internal/web/calendar.go ─── CRUD handlers
  ├─ internal/web/templates/calendar.html ─── list/form/detail templates
  └─ internal/web/templates/layout.html ─── sidebar Calendar entry

internal/today/today.go ─── Register(CalendarProvider) + showAllURLFor cases
  ├─ internal/today/providers/calendar.go ─── 2 sections (events-today, events-upcoming)
  └─ internal/today/sections.go ─── sectionOrder +2 keys

internal/migrations/<timestamp>_calendar.sql ─── schema
```

## Schema Design

File: `internal/migrations/<timestamp>_calendar.sql` — the `<timestamp>` is resolved at write time (`YYYYMMDDHHMMSS`, must sort after `20260613000000_finance.sql`); disk is source of truth per ADR-0002 Change 8.

```sql
-- +goose Up
-- Calendar domain (v3.x). Mirrors the finance table shape with calendar-specific
-- fields: start_at (datetime or date-only when all_day), nullable end_at,
-- all_day flag, and location. recurrence is metadata-only (no expansion in v3.x).
-- TIMEZONE NOTE (ADR-0003): start_at/end_at are stored as local-time strings
-- without a timezone offset, consistent with the single-system-timezone model.
-- Changing user_timezone reinterprets historical start_at/end_at values.

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS calendar_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    description TEXT,
    start_at    TEXT    NOT NULL,            -- 'YYYY-MM-DDTHH:MM:SS' (timed) or 'YYYY-MM-DD' (all_day=1)
    end_at      TEXT,                        -- nullable; NULL = open-ended; maps to iCal DTEND
    all_day     INTEGER NOT NULL DEFAULT 0 CHECK (all_day IN (0, 1)),
    location    TEXT    NOT NULL DEFAULT '',
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    notes       TEXT,
    recurrence  TEXT    NOT NULL DEFAULT 'none' CHECK (recurrence IN ('none','daily','weekly','monthly','yearly')),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at  TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS calendar_tags (
    event_id INTEGER NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    tag_id   INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (event_id, tag_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_calendar_start    ON calendar_events(start_at);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_calendar_deleted  ON calendar_events(deleted_at);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_calendar_category ON calendar_events(category_id);
-- +goose StatementEnd

-- Auto-bump updated_at on every UPDATE.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_calendar_updated_at
AFTER UPDATE ON calendar_events
FOR EACH ROW
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE calendar_events
    SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- FTS5 virtual table on title, description, location.
-- Note: CREATE VIRTUAL TABLE does not support IF NOT EXISTS.
-- +goose StatementBegin
CREATE VIRTUAL TABLE calendar_fts USING fts5(
    title,
    description,
    location,
    tokenize='unicode61 remove_diacritics 2'
);
-- +goose StatementEnd

-- Sync triggers for calendar_fts.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_calendar_fts_insert
AFTER INSERT ON calendar_events
BEGIN
    INSERT INTO calendar_fts(rowid, title, description, location)
    VALUES (NEW.id, NEW.title, COALESCE(NEW.description, ''), COALESCE(NEW.location, ''));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_calendar_fts_update
AFTER UPDATE OF title, description, location ON calendar_events
BEGIN
    DELETE FROM calendar_fts WHERE rowid = OLD.id;
    INSERT INTO calendar_fts(rowid, title, description, location)
    VALUES (NEW.id, NEW.title, COALESCE(NEW.description, ''), COALESCE(NEW.location, ''));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_calendar_fts_delete
AFTER DELETE ON calendar_events
BEGIN
    DELETE FROM calendar_fts WHERE rowid = OLD.id;
END;
-- +goose StatementEnd

-- +goose Down
-- Rollback: DROP TRIGGER, TABLE, INDEX in reverse order.
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_calendar_fts_delete;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_calendar_fts_update;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_calendar_fts_insert;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS calendar_fts;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_calendar_updated_at;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_calendar_category;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_calendar_deleted;
-- +goose StatementEnd
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_calendar_start;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS calendar_tags;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS calendar_events;
-- +goose StatementEnd
```

**sqlc.yaml overrides** — append to the existing `overrides` list (mirrors the finance entries). `description`, `end_at`, and `notes` are nullable TEXT; `category_id` nullable INT. `start_at`, `location`, `title`, `all_day` are NOT NULL (no override). `deleted_at` is nullable:

```yaml
- column: "calendar_events.deleted_at"
  go_type:
    import: "database/sql"
    type: "NullString"
- column: "calendar_events.description"
  go_type:
    import: "database/sql"
    type: "NullString"
- column: "calendar_events.end_at"
  go_type:
    import: "database/sql"
    type: "NullString"
- column: "calendar_events.notes"
  go_type:
    import: "database/sql"
    type: "NullString"
- column: "calendar_events.category_id"
  go_type:
    import: "database/sql"
    type: "NullInt64"
```

> **all_day type note**: `all_day INTEGER` generates as `int64` in the `store.CalendarEvent` struct (SQLite has no native bool). The service converts to/from a Go `bool` at the boundary; CLI/web flags surface it as `--all-day` / a checkbox.

## sqlc Query List

File: `internal/store/queries/calendar.sql` (mirrors `finance.sql` query names; `ListCalendarFiltered` and `SearchCalendar` are hand-written in Go).

```sql
-- name: CreateCalendarEvent :one
INSERT INTO calendar_events (
    title, description, start_at, end_at, all_day, location, category_id, notes, recurrence
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetCalendarEvent :one
SELECT * FROM calendar_events WHERE id = ? LIMIT 1;

-- name: ListCalendarEvents :many
SELECT * FROM calendar_events
WHERE deleted_at IS NULL
ORDER BY start_at ASC, created_at DESC, id DESC
LIMIT ? OFFSET ?;

-- name: ListTrashedCalendarEvents :many
SELECT * FROM calendar_events
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id DESC;

-- name: UpdateCalendarEvent :one
UPDATE calendar_events
SET title = ?, description = ?, start_at = ?, end_at = ?, all_day = ?,
    location = ?, category_id = ?, notes = ?, recurrence = ?
WHERE id = ?
RETURNING *;

-- name: SoftDeleteCalendarEvent :exec
UPDATE calendar_events
SET deleted_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND deleted_at IS NULL;

-- name: RestoreCalendarEvent :exec
UPDATE calendar_events
SET deleted_at = NULL
WHERE id = ? AND deleted_at IS NOT NULL;

-- name: PurgeCalendarEvent :exec
DELETE FROM calendar_events WHERE id = ?;

-- name: CountCalendarEvents :one
SELECT COUNT(*) FROM calendar_events WHERE deleted_at IS NULL;

-- name: ListEventsToday :many
-- Events whose start_at falls within [dayStart, dayEnd]. The provider passes
-- bounds formatted for the all_day branch (date) or timed branch (datetime).
SELECT * FROM calendar_events
WHERE deleted_at IS NULL
  AND start_at >= ? AND start_at <= ?
ORDER BY start_at ASC, id ASC;

-- name: ListEventsUpcoming :many
-- Events whose start_at falls within (dayEnd, weekEnd].
SELECT * FROM calendar_events
WHERE deleted_at IS NULL
  AND start_at > ? AND start_at <= ?
ORDER BY start_at ASC, id ASC;

-- name: ListTagsForCalendar :many
SELECT t.*
FROM tags t
JOIN calendar_tags ct ON ct.tag_id = t.id
WHERE ct.event_id = ?
ORDER BY t.name ASC;

-- name: AttachTagToCalendar :exec
INSERT OR IGNORE INTO calendar_tags (event_id, tag_id) VALUES (?, ?);

-- name: DetachAllTagsFromCalendar :exec
DELETE FROM calendar_tags WHERE event_id = ?;

-- ListCalendarFiltered is hand-written in list.go (dynamic WHERE).
-- SearchCalendar is hand-written in search.go (FTS5 MATCH not parseable by sqlc).
```

> **Provider query design note (a decision beyond the confirmed set)**: I model `ListEventsToday`/`ListEventsUpcoming` as simple range queries over `start_at` rather than separate all-day vs timed queries. The provider computes the bounds. For `events-today` the provider passes the **date-prefix range** `['YYYY-MM-DD', 'YYYY-MM-DDT23:59:59']` so that date-only all-day rows (`'YYYY-MM-DD'`) and timed rows on the same day are both captured by string comparison — `'2026-06-15' < '2026-06-15T09:00:00' < '2026-06-15T23:59:59'` holds lexicographically because the date-only value sorts before any datetime on that date. This is the key insight that lets a single column and a single query serve both kinds. See provider logic below.

**DeleteOrphanTags extension** (`internal/store/queries/tags.sql`):

```sql
-- name: DeleteOrphanTags :exec
DELETE FROM tags
WHERE id NOT IN (
    SELECT DISTINCT tag_id FROM resource_tags
    UNION
    SELECT DISTINCT tag_id FROM todo_tags
    UNION
    SELECT DISTINCT tag_id FROM finance_tags
    UNION
    SELECT DISTINCT tag_id FROM calendar_tags
);
```

## Service Go API

```go
// internal/calendar/event.go
package calendar

type Recurrence string
const (
    RecurrenceNone    Recurrence = "none"
    RecurrenceDaily   Recurrence = "daily"
    RecurrenceWeekly  Recurrence = "weekly"
    RecurrenceMonthly Recurrence = "monthly"
    RecurrenceYearly  Recurrence = "yearly" // calendar-specific per ADR-0001
)
func (r Recurrence) Valid() bool          // none/daily/weekly/monthly/yearly
func (r Recurrence) String() string
func AllRecurrences() []Recurrence

// CreateInput captures everything needed to insert or update an event.
type CreateInput struct {
    Title       string
    Description string
    StartAt     string   // 'YYYY-MM-DDTHH:MM:SS' (timed) or 'YYYY-MM-DD' (all-day)
    EndAt       string   // empty = NULL/open-ended
    AllDay      bool
    Location    string
    CategoryID  *int64
    Notes       string
    Recurrence  Recurrence
    Tags        []string
}

// Filter drives the dynamic listing query.
type Filter struct {
    From         *string // start_at lower bound (inclusive); ISO date or datetime
    To           *string // start_at upper bound (inclusive)
    Recurrence   *string // exact match
    CategorySlug string
    TagName      string
    Trashed      bool
    Limit        int
    Offset       int
    Search       string  // FTS5 query; when set, delegates to SearchCalendar
}

// ExportRow is one resolved event for iCal export.
type ExportRow struct {
    ID          int64
    Title       string
    Description string
    StartAt     string
    EndAt       string  // empty when NULL
    AllDay      bool
    Location    string
    Category    string
    Notes       string
    Recurrence  Recurrence
    Tags        []string
    CreatedAt   string  // for DTSTAMP / UID
}

// validateCreate enforces invariants (see Edge Cases section):
//   - Title required (non-empty after trim)
//   - Recurrence required + Valid()
//   - StartAt required + parseable as date (all_day) or datetime (timed)
//   - When AllDay: StartAt must be date-only 'YYYY-MM-DD'; EndAt (if set) date-only
//   - When timed: StartAt must be 'YYYY-MM-DDTHH:MM:SS'; EndAt (if set) datetime
//   - When EndAt set: EndAt >= StartAt (string comparison is valid for same format)
func validateCreate(in CreateInput) error
```

```go
// internal/calendar/service.go
type Event struct {
    Row  store.CalendarEvent
    Tags []string
}

type Service struct { db *sql.DB; q *store.Queries; now func() time.Time }
type Option func(*Service)
func WithClock(now func() time.Time) Option

func New(db *sql.DB, opts ...Option) *Service
func (s *Service) Create(ctx, in CreateInput) (*Event, error)            // validate, tx, insert, attach tags (PruneOrphans:false)
func (s *Service) Get(ctx, id int64) (*Event, error)                     // sql.ErrNoRows if missing
func (s *Service) Update(ctx, id int64, in CreateInput) (*Event, error)  // detach-all + re-attach with PruneOrphans:true
func (s *Service) SoftDelete(ctx, id int64) error
func (s *Service) Restore(ctx, id int64) error
func (s *Service) Purge(ctx, id int64) error                             // DELETE + prune orphans in tx
func (s *Service) List(ctx, f Filter) ([]*Event, error)                  // delegates to ListCalendarFiltered or SearchCalendar
func (s *Service) Export(ctx, f Filter) ([]ExportRow, error)             // resolves category names + tags

// boundary helpers (mirror finance/service.go):
func nullableString(s string) sql.NullString  // empty -> NULL (used for description, end_at, notes)
func nullableInt64(i *int64) sql.NullInt64
func boolToInt(b bool) int64                   // all_day conversion
```

Unlike Finance, Calendar's `Create`/`Update` does **not** read any config value (no currency equivalent). `start_at`/`end_at` are stored verbatim. The `all_day` flag is converted via `boolToInt`.

**Attacher** (`internal/calendar/attacher.go`): mirrors `finance/attacher.go` exactly. `UpsertTag` → `store.UpsertTag`; `AttachTagToOwner` → `AttachTagToCalendar` with `event_id`; `DeleteOrphanTags` → shared query. `OwnerKind` is `"calendar"`.

## iCal Writer (`internal/calendar/ical.go`)

stdlib-only RFC 5545 writer. Entry point:

```go
func WriteICal(w io.Writer, rows []ExportRow) error
```

**Output skeleton:**

```
BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//arsenal//calendar//EN
CALSCALE:GREGORIAN
BEGIN:VEVENT
UID:<id>@arsenal
DTSTAMP:<created_at as UTC basic datetime>
SUMMARY:<escaped title>
DTSTART<value-type>:<formatted start>
DTEND<value-type>:<formatted end>        ; omitted when EndAt empty
DESCRIPTION:<escaped description>          ; omitted when empty
LOCATION:<escaped location>               ; omitted when empty
RRULE:<mapped>                            ; omitted when recurrence=none
CATEGORIES:<escaped category + tags>      ; omitted when none
END:VEVENT
... (one VEVENT per row)
END:VCALENDAR
```

**Datetime formatting — all-day vs timed (decision beyond confirmed set):**

| Case | Stored `start_at` | iCal property | Value |
|------|-------------------|---------------|-------|
| All-day | `2026-06-15` | `DTSTART;VALUE=DATE` | `20260615` |
| All-day end | `2026-06-16` | `DTEND;VALUE=DATE` | `20260616` |
| Timed | `2026-06-15T09:00:00` | `DTSTART` (local, floating) | `20260615T090000` |
| Timed end | `2026-06-15T10:30:00` | `DTEND` | `20260615T103000` |

Timed values are emitted as **floating local time** (no `Z`, no `TZID`), consistent with ADR-0003's single-system-timezone storage model — the strings are local already and arsenal does not persist an offset. `DTSTAMP` uses `created_at` (already stored as `...Z` UTC) reformatted to basic UTC form `YYYYMMDDTHHMMSSZ`. Format conversion strips `-`/`:` separators via `time.Parse` + `Format`, with a string-munge fallback if parse fails.

**RRULE mapping table (T8):**

| `recurrence` | RRULE |
|--------------|-------|
| `none` | (RRULE line omitted) |
| `daily` | `RRULE:FREQ=DAILY` |
| `weekly` | `RRULE:FREQ=WEEKLY` |
| `monthly` | `RRULE:FREQ=MONTHLY` |
| `yearly` | `RRULE:FREQ=YEARLY` |

**RFC 5545 text escaping** (`escapeText`): in `SUMMARY`/`DESCRIPTION`/`LOCATION`/`CATEGORIES` values, escape `\` → `\\`, `;` → `\;`, `,` → `\,`, newline → `\n`. (`CATEGORIES` is comma-separated by spec, so the per-item value is escaped before joining with literal commas.)

**Line folding** (`foldLine`): any content line longer than 75 octets is folded by inserting CRLF followed by a single space, per RFC 5545 §3.1. All lines terminate with CRLF (`\r\n`).

**Edge cases handled by the writer:** empty `rows` → still emits a valid empty VCALENDAR (BEGIN/VERSION/PRODID/END); NULL `end_at` → no DTEND; empty description/location → property omitted.

## CalendarProvider (`internal/today/providers/calendar.go`)

```go
type CalendarProvider struct {
    queries *store.Queries
    db      *sql.DB
    now     func() time.Time
}
type CalendarProviderOption func(*CalendarProvider)
func WithCalendarClock(now func() time.Time) CalendarProviderOption
func NewCalendarProvider(db *sql.DB, opts ...CalendarProviderOption) *CalendarProvider
func (p *CalendarProvider) Name() string { return "calendar" }
func (p *CalendarProvider) Sections(ctx context.Context) ([]today.Section, error)
```

**`Sections` logic — timezone-aware bounds (T4):**

```go
loc, err := today.UserLocation(ctx, p.db)         // *time.Location, UTC fallback
now := p.now().In(loc)
todayDate := now.Format("2006-01-02")             // e.g. "2026-06-15"
dayStart := todayDate                             // captures all-day rows (date-only sorts first)
dayEnd   := todayDate + "T23:59:59"               // captures timed rows up to end of day
weekEnd  := now.AddDate(0, 0, 7).Format("2006-01-02") + "T23:59:59"

// events-today: start_at in [dayStart, dayEnd]
todayRows, _ := p.queries.ListEventsToday(ctx, ListEventsTodayParams{StartAt: dayStart, StartAt_2: dayEnd})
// events-upcoming: start_at in (dayEnd, weekEnd]
upRows, _    := p.queries.ListEventsUpcoming(ctx, ListEventsUpcomingParams{StartAt: dayEnd, StartAt_2: weekEnd})
```

Why this works (the all_day vs timed branch reduces to one comparison): a date-only string `"2026-06-15"` is lexicographically less than `"2026-06-15T00:00:00"` (the `T` byte `0x54` is greater than the absence of a character), so an all-day event on day D satisfies `D >= dayStart && D <= dayEnd`. A timed event `"2026-06-15T09:00:00"` also satisfies it. No `all_day` branch is needed in the SQL — only in the **subtitle rendering**:

```go
func mapCalendarItem(row store.CalendarEvent) today.Item {
    var subtitle string
    if row.AllDay == 1 {
        subtitle = "All day"
    } else {
        // parse start_at/end_at as datetime, render "09:00–10:30" (or "09:00" when end NULL)
        subtitle = formatTimeRange(row.StartAt, row.EndAt)
    }
    if row.Location != "" {
        subtitle += " · " + row.Location
    }
    return today.Item{
        Domain:   "calendar",
        ID:       row.ID,
        Title:    row.Title,
        Subtitle: subtitle,
        URL:      fmt.Sprintf("/calendar/%d", row.ID),
    }
}
```

Empty sections are omitted (`len(rows) == 0` → not appended), matching FinanceProvider. Errors are returned so the registry degrades gracefully.

**Registration**: add `todaySvc.Register(providers.NewCalendarProvider(db))` in `newHandlers()` ([`handlers.go`](../../internal/web/handlers.go)) and `New()` ([`app.go`](../../internal/tui/app.go)), after the finance registration.

**Section ordering** — extend [`sections.go`](../../internal/today/sections.go):

```go
var sectionOrder = map[string]int{
    "overdue":              1,
    "due-today":            2,
    "upcoming":             3,
    "recent":               4,
    "this-month-spending":  5,
    "recent-transactions":  6,
    "events-today":         7,
    "events-upcoming":      8,
}
```

**showAllURLFor** — extend in [`today.go`](../../internal/today/today.go):

```go
case "events-today":    return "/calendar?when=today"
case "events-upcoming": return "/calendar?when=upcoming"
```

## TUI Design (`internal/tui/calendar.go`)

Mirrors [`finance.go`](../../internal/tui/finance.go):

- **`calendarItem`** adapts `calendar.Event` to `list.Item` — `Title()` shows `title`, `Description()` shows `<start_at formatted> · <location | recurrence> · #tags`.
- **`calendarDetailModel`** renders all fields via viewport (mirrors finance detail).
- **State machine**: `calendarViewState` enum (`calendarStateList`, `calendarStateDetail`, `calendarStateTrash`, `calendarStateConfirmDelete`).
- **App fields** (in `app.go`): `calendarService`, `calendarList`, `calendarDetail`, `calendarConfirm`, `calendarShowTrashed`, `calendarState`.
- **`updateCalendar()`**: dispatches by state; keybindings `n` (new), `e` (edit), `d` (soft-delete), `r` (restore in trash), `x` (purge), `j/k` (navigate), `enter` (detail), `Tab` (area switch).
- **`loadCalendarCmd()`**: calls `svc.List(ctx, Filter{Trashed, Limit: 500})`.
- **Status bar** (`status.go`): calendar hints `n new · e edit · d del · Tab switch`.
- **Wire in `app.go`**: replace `placeholderView("Calendar (coming soon — v3.x)", …)` in `View()` with `a.calendarView()`; add `case areaCalendar: return a.updateCalendar(msg)` in `Update()`; add `case areaCalendar: return loadCalendarCmd(…)` in `loadCurrentAreaCmd()`.

## Web Design (`internal/web/calendar.go`)

Mirrors [`finance.go`](../../internal/web/finance.go):

| Method | Path | Handler | HTMX |
|--------|------|---------|------|
| GET | `/calendar` | `listCalendar` | No |
| GET | `/calendar/new` | `newCalendarForm` | No |
| POST | `/calendar` | `createCalendar` | No |
| GET | `/calendar/{id}` | `showCalendar` | No |
| GET | `/calendar/{id}/edit` | `editCalendarForm` | No |
| POST | `/calendar/{id}` | `updateCalendar` | No |
| POST | `/calendar/{id}/delete` | `softDeleteCalendar` | Yes (empty fragment) |
| POST | `/calendar/{id}/restore` | `restoreCalendar` | Yes (card fragment) |
| POST | `/calendar/{id}/purge` | `purgeCalendar` | No (redirect) |

- **Route registration**: `h.calendarRoutes(r)` in [`server.go`](../../internal/web/server.go) after `h.financeRoutes(r)`.
- **Form fields**: title, description, start date + start time (hidden/disabled when all-day), end date + end time, all-day checkbox, location, category select, tags, recurrence select, notes. The handler composes `start_at`/`end_at` strings from the date+time inputs (date-only when all-day).
- **`?when=today` / `?when=upcoming`** query params filter the list (used by `showAllURLFor` links).
- **Sidebar**: add Calendar link in [`layout.html`](../../internal/web/templates/layout.html) between Finance and Trash (both main layout and `sidebar-oob` fragment) with `{{.CalendarCount}}` badge.
- **Header nav**: add `<a href="/calendar">Calendar</a>`.
- **`commonPage()`**: add `CalendarCount int64` to `pageData`; compute via `CountCalendarEvents`. Add `calendarService *calendar.Service` to `Handlers` struct.

## CLI Design (`internal/cli/calendar*.go`)

Mirrors `finance*.go` (split across files: `calendar.go` parent, `calendar_add.go`, `calendar_edit.go`, `calendar_list.go`, `calendar_show.go`, `calendar_rm_restore_purge.go`, `calendar_export.go`).

```go
func newCalendarCmd() *cobra.Command  // parent: "arsenal calendar"
// Subcommands:
//   add     --title --start --end --all-day --location --cat --tag --notes --recurrence [--json]
//   list    --from --to --recurrence --cat --tag --trashed [--json]
//   show    <id> [--json]
//   edit    <id> (same flags as add)
//   rm      <id>
//   restore <id>
//   purge   <id> [--yes]
//   export  --format ical [--output path] [--from --to --cat --tag]
```

- `--start` accepts `YYYY-MM-DD` (implies all-day unless `--end` has a time) or `YYYY-MM-DDTHH:MM`; the CLI normalizes to the storage format and infers/validates `--all-day`.
- `export --format ical`: default `ical`; only `ical` supported in v3.x (no CSV). Writes to stdout or `--output`. Calls `Service.Export()` then `WriteICal()`.
- Register `root.AddCommand(newCalendarCmd())` in [`root.go`](../../internal/cli/root.go) after `newFinanceCmd()`.
- Completions in [`completion.go`](../../internal/cli/completion.go): recurrence values, `--format ical`.

## Attacher Integration

`internal/calendar/attacher.go` mirrors `finance/attacher.go`. `OwnerKind = "calendar"`, `AttachTagToOwner` calls `AttachTagToCalendar(event_id, tag_id)`. The shared `DeleteOrphanTags` UNION is extended (see sqlc section).

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Migration | Tables, indices, FTS5 sync, CHECK constraints (all_day 0/1, recurrence enum incl. yearly) | `newTestDB(t)` + assert schema + insert/search/delete |
| Service | Create/Update/Delete/Restore/Purge/List/Export; all-day vs timed; NULL end_at; end<start rejection | Table-driven, filter combos, tag lifecycle |
| iCal | DTSTART/DTEND value types, RRULE per recurrence, escaping, line folding, empty export, NULL end | Table-driven; parse output lines + assert |
| Provider | events-today / events-upcoming bounds, all-day inclusion, timezone, empty omission, error degradation | `newTestDB(t)` + seed + `WithCalendarClock` |
| TUI | Keybindings, list/detail/trash transitions | Bubbletea teatest |
| Web | Routes, sidebar count, HTMX fragments, form→start_at composition | `httptest.NewServer` + assert |
| CLI | Subcommands, `--json`, `--yes`, `export --format ical` | `cmd.ExecuteContext` + capture stdout |

Strict TDD: tests written before implementation for all new packages. Pattern: `newTestDB(t)` from existing service tests.

## File-by-File Implementation Plan (8 phases)

Mirrors the finance commit cadence so `sdd-tasks` can slice cleanly into chained PRs. Each phase is independently testable and compiles on its own.

**Phase 1 — Migration + schema gen**
- NEW `internal/migrations/<timestamp>_calendar.sql`
- MOD `sqlc.yaml` (5 overrides)
- NEW `internal/calendar/migration_test.go`
- Run `sqlc generate` → produces `store.CalendarEvent`, params, query methods.

**Phase 2 — Service skeleton + domain types**
- NEW `internal/calendar/event.go` (Recurrence enum, CreateInput, Filter, ExportRow, validateCreate)
- NEW `internal/calendar/attacher.go`
- NEW `internal/calendar/service.go` (New, Create, Get, Update, SoftDelete, Restore, Purge; boundary helpers)
- NEW `internal/store/queries/calendar.sql` (all named queries)
- NEW `internal/calendar/service_test.go` (CRUD + tag lifecycle + validation)

**Phase 3 — List, Filter, Search, Export data**
- MOD `internal/store/list.go` (`ListCalendarFiltered`, `CalendarListFilter`, `ListedCalendar`)
- MOD `internal/store/search.go` (`SearchCalendar`)
- ADD to `service.go`: `List`, `Export`
- ADD to `service_test.go`: filter combos, FTS5 search

**Phase 4 — iCal export**
- NEW `internal/calendar/ical.go` (`WriteICal`, formatICalDateTime, mapRRULE, escapeText, foldLine)
- NEW `internal/calendar/ical_test.go`

**Phase 5 — Cross-domain tag cleanup**
- MOD `internal/store/queries/tags.sql` (`DeleteOrphanTags` UNION + `calendar_tags`)
- Run `sqlc generate`
- Verify in `service_test.go` (Purge prunes orphans; finance/todo/resource tags untouched)

**Phase 6 — CLI**
- NEW `internal/cli/calendar.go` + `calendar_add.go` + `calendar_edit.go` + `calendar_list.go` + `calendar_show.go` + `calendar_rm_restore_purge.go` + `calendar_export.go`
- MOD `internal/cli/root.go` (register `newCalendarCmd()`)
- MOD `internal/cli/completion.go`
- NEW `internal/cli/calendar_test.go`

**Phase 7 — TUI**
- NEW `internal/tui/calendar.go`
- MOD `internal/tui/app.go` (fields, `View()`, `Update()`, `loadCurrentAreaCmd()`, provider registration)
- MOD `internal/tui/status.go` (calendar hints)

**Phase 8 — Web + Provider + Today integration**
- NEW `internal/web/calendar.go`
- NEW `internal/web/templates/calendar.html`
- MOD `internal/web/server.go` (`h.calendarRoutes`)
- MOD `internal/web/handlers.go` (routes, `CalendarCount`, `calendarService`, provider registration)
- MOD `internal/web/templates/layout.html` (sidebar + nav)
- NEW `internal/today/providers/calendar.go`
- NEW `internal/today/providers/calendar_test.go`
- MOD `internal/today/sections.go` (2 keys)
- MOD `internal/today/today.go` (2 `showAllURLFor` cases)

> **Suggested PR split** (per proposal scope warning, ~1600–2400 LOC): PR1 = Phases 1–3 (migration + service + filter/search/export data); PR2 = Phases 4–5 (iCal + tag cleanup); PR3 = Phase 6 (CLI); PR4 = Phase 7 (TUI); PR5 = Phase 8 (Web + Provider + Today). `sdd-tasks` produces the official forecast.

## Edge Cases

| Case | Handling |
|------|----------|
| NULL `end_at` | Stored as SQL NULL via `nullableString("")`. iCal omits DTEND. Provider subtitle renders only start time. List/show render "—" or single time. |
| All-day vs timed | `all_day=1` → `start_at` is date-only `YYYY-MM-DD`; `validateCreate` rejects a time component. `all_day=0` → `start_at` must be `YYYY-MM-DDTHH:MM:SS`. iCal switches DTSTART value type accordingly. Provider/SQL needs no branch (lexicographic ordering); only subtitle rendering branches. |
| `end_at` before `start_at` | `validateCreate` rejects when both non-empty and `EndAt < StartAt` (string comparison valid within the same format). Mixed formats (all-day start, timed end) are rejected by the format checks first. |
| Timezone reinterpretation | `start_at` stored as local string without offset (ADR-0003). Changing `KeyUserTimezone` shifts which events the provider counts as "today" without rewriting rows. Documented in migration comment + `calendar-service` spec. Provider always recomputes bounds in current `UserLocation`. |
| Empty title | Rejected by `validateCreate`. |
| Invalid recurrence | Rejected by `validateCreate` + DB CHECK constraint (defense in depth). |
| Empty iCal export | `WriteICal` emits a valid empty VCALENDAR. |
| FTS5 re-run on manual migration | `CREATE VIRTUAL TABLE` lacks `IF NOT EXISTS`; goose tracks applied migrations so this only affects manual re-runs. |

## Migration / Rollout

- Forward-only per ADR-0001 migration policy. First calendar migration — zero existing data.
- `IF NOT EXISTS` on tables/indices/triggers except the FTS5 virtual table (SQLite limitation).
- Rollback: revert commits. If applied with no data, `DROP TABLE IF EXISTS calendar_events` cascades `calendar_tags`; drop `calendar_fts` separately (the `-- +goose Down` block does this in order).
- TUI renders the placeholder until the migration is applied and the area is wired. Web sidebar entry hidden when `CalendarCount == 0` (same as Finance).
- Today view degrades gracefully when `CalendarProvider.Sections()` errors.

## Open Risks

- **1600–2400 LOC** — chained PRs required (see PR split above).
- **Lexicographic date/datetime comparison** — relies on the storage format invariant (`all_day` rows are exactly `YYYY-MM-DD`, timed rows exactly `YYYY-MM-DDTHH:MM:SS`). `validateCreate` is the enforcement point; if it weakens, the provider range query breaks. Tested explicitly.
- **Floating-time iCal** — events export without TZID. Importing into Google/Apple Calendar treats them as floating local time. Acceptable under the single-timezone model; documented in the `calendar-ical-export` spec.
- **Changing `KeyUserTimezone`** reinterprets historical `start_at` — low impact, documented.

## Cross-References

- [ADR-0001](../../docs/adr/0001-v3-scope.md) — spine; Calendar scope (daily routine, simple recurrence incl. yearly, single tz, iCal export)
- [ADR-0002](../../docs/adr/0002-v3-replan.md) — Change 1 (deferred), Change 4 (registry), Change 9 (calendar=iCal)
- [ADR-0003](../../docs/adr/0003-timezone-handling.md) — single-timezone storage for `start_at`
- [proposal.md](./proposal.md) — scope, 7 capabilities, 2 AMENDs, 6 locked decisions
- [explore.md](./explore.md) — Q1–Q6, T1–T8
- [v3.x-finance design](../archive/2026-06-14-v3.x-finance/design.md) — direct structural template
