package calendar_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestCalendarMigration_CreatesTablesAndIndices(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	tables := []string{
		"calendar_events",
		"calendar_tags",
		"calendar_fts",
	}
	for _, name := range tables {
		var got string
		if err := db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type IN ('table', 'shadow', 'virtual table') AND name = ?", name,
		).Scan(&got); err != nil {
			// Try without 'shadow' (older SQLite)
			if err2 := db.QueryRowContext(ctx,
				"SELECT name FROM sqlite_master WHERE name = ?", name,
			).Scan(&got); err2 != nil {
				t.Fatalf("expected %q to exist: %v", name, err2)
			}
		}
	}

	indices := []string{
		"idx_calendar_start",
		"idx_calendar_deleted",
		"idx_calendar_category",
	}
	for _, name := range indices {
		var got string
		if err := db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?", name,
		).Scan(&got); err != nil {
			t.Fatalf("expected index %q to exist: %v", name, err)
		}
	}
}

func TestCalendarMigration_AllDayCheckConstraint(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, start_at, all_day)
		VALUES ('test', '2026-06-15', 2)
	`)
	if err == nil {
		t.Fatal("expected CHECK constraint error for all_day=2, got nil")
	}
}

func TestCalendarMigration_RecurrenceCheckConstraint(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, start_at, recurrence)
		VALUES ('test', '2026-06-15', 'biweekly')
	`)
	if err == nil {
		t.Fatal("expected CHECK constraint error for recurrence='biweekly', got nil")
	}
}

func TestCalendarMigration_RecurrenceYearlyAllowed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, start_at, all_day, recurrence)
		VALUES ('birthday', '2026-06-15', 1, 'yearly')
	`)
	if err != nil {
		t.Fatalf("expected yearly recurrence to be accepted: %v", err)
	}
}

func TestCalendarMigration_NullableEndAt(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	res, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, start_at, all_day)
		VALUES ('open-ended', '2026-06-15', 1)
	`)
	if err != nil {
		t.Fatalf("insert with NULL end_at: %v", err)
	}
	id, _ := res.LastInsertId()

	var endAt sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT end_at FROM calendar_events WHERE id = ?`, id).Scan(&endAt); err != nil {
		t.Fatalf("select end_at: %v", err)
	}
	if endAt.Valid {
		t.Fatalf("expected end_at to be NULL, got %q", endAt.String)
	}
}

func TestCalendarMigration_DateOnlyStartAt(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	res, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, start_at, all_day)
		VALUES ('all-day event', '2026-06-15', 1)
	`)
	if err != nil {
		t.Fatalf("insert date-only start_at: %v", err)
	}
	id, _ := res.LastInsertId()

	var startAt string
	if err := db.QueryRowContext(ctx, `SELECT start_at FROM calendar_events WHERE id = ?`, id).Scan(&startAt); err != nil {
		t.Fatalf("select start_at: %v", err)
	}
	if startAt != "2026-06-15" {
		t.Fatalf("start_at = %q, want 2026-06-15", startAt)
	}
}

func TestCalendarMigration_FTS5SyncsOnInsert(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, description, location, start_at, all_day)
		VALUES ('team standup', 'daily sync', 'conference room A', '2026-06-15T09:00:00', 0)
	`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	var found int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "standup").Scan(&found); err != nil {
		t.Fatalf("fts search after insert: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 fts match for title after insert, got %d", found)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "sync").Scan(&found); err != nil {
		t.Fatalf("fts search description: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 fts match for description after insert, got %d", found)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "conference").Scan(&found); err != nil {
		t.Fatalf("fts search location: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 fts match for location after insert, got %d", found)
	}
}

func TestCalendarMigration_FTS5SyncsOnUpdate(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	res, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, start_at, all_day)
		VALUES ('old title', '2026-06-15', 1)
	`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, _ := res.LastInsertId()

	if _, err := db.ExecContext(ctx, `UPDATE calendar_events SET title = 'new title' WHERE id = ?`, id); err != nil {
		t.Fatalf("update: %v", err)
	}

	var found int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "old").Scan(&found); err != nil {
		t.Fatalf("fts search old: %v", err)
	}
	if found != 0 {
		t.Fatalf("expected 0 matches for old title, got %d", found)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "new").Scan(&found); err != nil {
		t.Fatalf("fts search new: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 match for new title, got %d", found)
	}
}

func TestCalendarMigration_FTS5SyncsOnDelete(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	res, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, start_at, all_day)
		VALUES ('deleted event', '2026-06-15', 1)
	`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, _ := res.LastInsertId()

	if _, err := db.ExecContext(ctx, `DELETE FROM calendar_events WHERE id = ?`, id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var found int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "deleted").Scan(&found); err != nil {
		t.Fatalf("fts search after delete: %v", err)
	}
	if found != 0 {
		t.Fatalf("expected 0 matches after delete, got %d", found)
	}
}

func TestCalendarMigration_FTS5LocationSync(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	res, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, location, start_at, all_day)
		VALUES ('offsite', 'mountain cabin', '2026-06-15', 1)
	`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, _ := res.LastInsertId()

	var found int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "mountain").Scan(&found); err != nil {
		t.Fatalf("fts search location: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 match for location, got %d", found)
	}

	// Update location and verify sync
	if _, err := db.ExecContext(ctx, `UPDATE calendar_events SET location = 'beach house' WHERE id = ?`, id); err != nil {
		t.Fatalf("update location: %v", err)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "mountain").Scan(&found); err != nil {
		t.Fatalf("fts search old location: %v", err)
	}
	if found != 0 {
		t.Fatalf("expected 0 matches for old location, got %d", found)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "beach").Scan(&found); err != nil {
		t.Fatalf("fts search new location: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 match for new location, got %d", found)
	}
}

func TestCalendarMigration_JunctionLinksTags(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	var tagID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO tags (name) VALUES ('work') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatalf("insert tag: %v", err)
	}

	res, err := db.ExecContext(ctx, `
		INSERT INTO calendar_events (title, start_at, all_day)
		VALUES ('team lunch', '2026-06-15', 1)
	`)
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	eventID, _ := res.LastInsertId()

	if _, err := db.ExecContext(ctx, `INSERT INTO calendar_tags (event_id, tag_id) VALUES (?, ?)`, eventID, tagID); err != nil {
		t.Fatalf("insert calendar_tags: %v", err)
	}

	var got int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_tags WHERE event_id = ? AND tag_id = ?`, eventID, tagID).Scan(&got); err != nil {
		t.Fatalf("count junction: %v", err)
	}
	if got != 1 {
		t.Fatalf("expected 1 junction row, got %d", got)
	}

	// Cascading delete from events removes junction rows.
	if _, err := db.ExecContext(ctx, `DELETE FROM calendar_events WHERE id = ?`, eventID); err != nil {
		t.Fatalf("delete event: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_tags WHERE event_id = ?`, eventID).Scan(&got); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("count junction after delete: %v", err)
		}
	}
	if got != 0 {
		t.Fatalf("expected 0 junction rows after cascade, got %d", got)
	}
}

func TestCalendarMigration_RerunTableStmtsSafe(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	// IF NOT EXISTS guards mean re-running the CREATE TABLE stmts is a no-op.
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS calendar_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT,
		start_at TEXT NOT NULL,
		end_at TEXT,
		all_day INTEGER NOT NULL DEFAULT 0 CHECK (all_day IN (0, 1)),
		location TEXT NOT NULL DEFAULT '',
		category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
		notes TEXT,
		recurrence TEXT NOT NULL DEFAULT 'none' CHECK (recurrence IN ('none','daily','weekly','monthly','yearly')),
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
		deleted_at TEXT
	)`)
	if err != nil {
		t.Fatalf("re-run CREATE TABLE IF NOT EXISTS: %v", err)
	}

	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS calendar_tags (
		event_id INTEGER NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
		tag_id   INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
		PRIMARY KEY (event_id, tag_id)
	)`)
	if err != nil {
		t.Fatalf("re-run CREATE TABLE IF NOT EXISTS calendar_tags: %v", err)
	}

	_, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_calendar_start ON calendar_events(start_at)`)
	if err != nil {
		t.Fatalf("re-run CREATE INDEX IF NOT EXISTS: %v", err)
	}
}
