package finance_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestMigration_CreatesTablesAndIndices(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	tables := []string{
		"finance_transactions",
		"finance_tags",
		"finance_fts",
	}
	for _, name := range tables {
		var got string
		if err := db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type IN ('table', 'virtual table') AND name = ?", name,
		).Scan(&got); err != nil {
			t.Fatalf("expected %q to exist: %v", name, err)
		}
	}

	indices := []string{
		"idx_finance_date",
		"idx_finance_kind",
		"idx_finance_deleted",
		"idx_finance_category",
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

func TestMigration_KindCheckConstraint(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO finance_transactions (date, amount, kind, account)
		VALUES ('2026-06-13', 1.00, 'transfer', 'x')
	`)
	if err == nil {
		t.Fatal("expected CHECK constraint error for kind='transfer', got nil")
	}
}

func TestMigration_RecurrenceCheckConstraint(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	_, err := db.ExecContext(ctx, `
		INSERT INTO finance_transactions (date, amount, kind, account, recurrence)
		VALUES ('2026-06-13', 1.00, 'expense', 'x', 'yearly')
	`)
	if err == nil {
		t.Fatal("expected CHECK constraint error for recurrence='yearly', got nil")
	}
}

func TestMigration_FTS5Syncs(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	res, err := db.ExecContext(ctx, `
		INSERT INTO finance_transactions (date, amount, kind, account, notes)
		VALUES ('2026-06-13', 10.00, 'expense', 'checking', 'lunch meeting')
	`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, _ := res.LastInsertId()

	var found int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM finance_fts WHERE finance_fts MATCH ?`, "lunch").Scan(&found); err != nil {
		t.Fatalf("search after insert: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 fts match after insert, got %d", found)
	}

	_, err = db.ExecContext(ctx, `
		UPDATE finance_transactions SET notes = 'dinner meeting' WHERE id = ?
	`, id)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM finance_fts WHERE finance_fts MATCH ?`, "lunch").Scan(&found); err != nil {
		t.Fatalf("search old term after update: %v", err)
	}
	if found != 0 {
		t.Fatalf("expected 0 matches for old term, got %d", found)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM finance_fts WHERE finance_fts MATCH ?`, "dinner").Scan(&found); err != nil {
		t.Fatalf("search new term after update: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 match for new term, got %d", found)
	}

	_, err = db.ExecContext(ctx, `DELETE FROM finance_transactions WHERE id = ?`, id)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM finance_fts WHERE finance_fts MATCH ?`, "dinner").Scan(&found); err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if found != 0 {
		t.Fatalf("expected 0 matches after delete, got %d", found)
	}
}

func TestMigration_JunctionLinksTags(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	var tagID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO tags (name) VALUES ('work') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatalf("insert tag: %v", err)
	}

	res, err := db.ExecContext(ctx, `
		INSERT INTO finance_transactions (date, amount, kind, account)
		VALUES ('2026-06-13', 10.00, 'expense', 'checking')
	`)
	if err != nil {
		t.Fatalf("insert transaction: %v", err)
	}
	finID, _ := res.LastInsertId()

	_, err = db.ExecContext(ctx, `INSERT INTO finance_tags (finance_id, tag_id) VALUES (?, ?)`, finID, tagID)
	if err != nil {
		t.Fatalf("insert finance_tags: %v", err)
	}

	var got int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM finance_tags WHERE finance_id = ? AND tag_id = ?`, finID, tagID).Scan(&got); err != nil {
		t.Fatalf("count junction: %v", err)
	}
	if got != 1 {
		t.Fatalf("expected 1 junction row, got %d", got)
	}

	// Cascading delete from transactions removes junction rows.
	_, err = db.ExecContext(ctx, `DELETE FROM finance_transactions WHERE id = ?`, finID)
	if err != nil {
		t.Fatalf("delete transaction: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM finance_tags WHERE finance_id = ?`, finID).Scan(&got); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("count junction after delete: %v", err)
		}
	}
	if got != 0 {
		t.Fatalf("expected 0 junction rows after cascade, got %d", got)
	}
}
