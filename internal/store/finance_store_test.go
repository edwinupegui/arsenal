package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
)

func newFinanceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "arsenal.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestFinanceStore_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	db := newFinanceTestDB(t)
	q := store.New(db)

	row, err := q.CreateFinanceTransaction(ctx, store.CreateFinanceTransactionParams{
		Date:       "2026-06-13",
		Amount:     42.50,
		Kind:       "expense",
		Account:    "checking",
		Recurrence: "none",
		Currency:   "USD",
	})
	if err != nil {
		t.Fatalf("CreateFinanceTransaction: %v", err)
	}
	if row.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if row.Kind != "expense" {
		t.Errorf("kind = %q, want expense", row.Kind)
	}
	if row.Currency != "USD" {
		t.Errorf("currency = %q, want USD", row.Currency)
	}

	got, err := q.GetFinanceTransaction(ctx, row.ID)
	if err != nil {
		t.Fatalf("GetFinanceTransaction: %v", err)
	}
	if got.ID != row.ID {
		t.Errorf("id = %d, want %d", got.ID, row.ID)
	}
}
