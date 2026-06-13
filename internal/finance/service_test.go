package finance_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/finance"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/todos"
)

func strPtr(s string) *string { return &s }

func validCreate() finance.CreateInput {
	return finance.CreateInput{
		Date:       "2026-06-13",
		Amount:     42.50,
		Kind:       finance.KindExpense,
		Account:    "checking",
		CategoryID: nil,
		Notes:      "lunch",
		Recurrence: finance.RecurrenceNone,
		Tags:       []string{"work", "  WORK  "}, // dup + whitespace
	}
}

func TestCreate_HappyPath(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	got, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if got.Row.Date != "2026-06-13" {
		t.Errorf("date = %q, want 2026-06-13", got.Row.Date)
	}
	if got.Row.Amount != 42.50 {
		t.Errorf("amount = %v, want 42.50", got.Row.Amount)
	}
	if got.Row.Kind != "expense" {
		t.Errorf("kind = %q, want expense", got.Row.Kind)
	}
	if got.Row.Account != "checking" {
		t.Errorf("account = %q, want checking", got.Row.Account)
	}
	if got.Row.Currency != "USD" {
		t.Errorf("currency = %q, want USD", got.Row.Currency)
	}
	if got.Row.DeletedAt.Valid {
		t.Errorf("deleted_at = %v, want null", got.Row.DeletedAt)
	}

	wantTags := []string{"work"}
	if !equalStrings(got.Tags, wantTags) {
		t.Errorf("tags = %v, want %v", got.Tags, wantTags)
	}

	round, err := svc.Get(ctx, got.Row.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !equalStrings(round.Tags, wantTags) {
		t.Errorf("Get tags = %v, want %v", round.Tags, wantTags)
	}
}

func TestCreate_Income(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	got, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-13",
		Amount:  3000.00,
		Kind:    finance.KindIncome,
		Account: "salary",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.Kind != "income" {
		t.Errorf("kind = %q, want income", got.Row.Kind)
	}
	if got.Row.Amount != 3000.00 {
		t.Errorf("amount = %v, want 3000.00", got.Row.Amount)
	}
}

func TestCreate_Defaults(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	got, err := svc.Create(ctx, finance.CreateInput{
		Amount: 10.00,
		Kind:   finance.KindExpense,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.Date == "" {
		t.Error("expected default date to be set")
	}
	if got.Row.Currency != "USD" {
		t.Errorf("currency = %q, want USD", got.Row.Currency)
	}
	if got.Row.Recurrence != "none" {
		t.Errorf("recurrence = %q, want none", got.Row.Recurrence)
	}
	if got.Row.Account != "" {
		t.Errorf("account = %q, want empty", got.Row.Account)
	}
	if len(got.Tags) != 0 {
		t.Errorf("tags = %v, want empty", got.Tags)
	}
}

func TestCreate_CurrencyFromConfig(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	cs := configstore.New(db)
	if err := cs.Set(ctx, config.KeyCurrency, "ARS"); err != nil {
		t.Fatalf("Set currency: %v", err)
	}

	svc := finance.New(db)
	got, err := svc.Create(ctx, finance.CreateInput{
		Amount: 1000.00,
		Kind:   finance.KindExpense,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.Currency != "ARS" {
		t.Errorf("currency = %q, want ARS", got.Row.Currency)
	}
}

func TestCreate_InvalidKind(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Create(ctx, finance.CreateInput{
		Amount: 10.00,
		Kind:   finance.Kind("transfer"),
	}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreate_InvalidRecurrence(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Create(ctx, finance.CreateInput{
		Amount:     10.00,
		Kind:       finance.KindExpense,
		Recurrence: finance.Recurrence("yearly"),
	}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreate_AmountNotPositive(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Create(ctx, finance.CreateInput{
		Amount: 0,
		Kind:   finance.KindExpense,
	}); err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}
}

func TestCreate_RollbackOnValidation(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	in := validCreate()
	in.Kind = ""
	if _, err := svc.Create(ctx, in); err == nil {
		t.Fatal("expected error, got nil")
	}

	q := store.New(db)
	count, err := q.CountFinanceTransactions(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 transactions, got %d", count)
	}

	tags, err := q.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}
}

func TestGet_Found(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Get(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Row.ID != created.Row.ID {
		t.Errorf("id = %d, want %d", got.Row.ID, created.Row.ID)
	}
	wantTags := []string{"work"}
	if !equalStrings(got.Tags, wantTags) {
		t.Errorf("tags = %v, want %v", got.Tags, wantTags)
	}
}

func TestGet_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Get(ctx, 999); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdate_ChangesAmountAndTags(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Row.ID, finance.CreateInput{
		Date:       "2026-06-14",
		Amount:     100.00,
		Kind:       finance.KindIncome,
		Account:    "savings",
		Notes:      "updated",
		Recurrence: finance.RecurrenceMonthly,
		Tags:       []string{"personal"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Row.Amount != 100.00 {
		t.Errorf("amount = %v, want 100.00", updated.Row.Amount)
	}
	if updated.Row.Kind != "income" {
		t.Errorf("kind = %q, want income", updated.Row.Kind)
	}
	if updated.Row.Date != "2026-06-14" {
		t.Errorf("date = %q, want 2026-06-14", updated.Row.Date)
	}
	if !equalStrings(updated.Tags, []string{"personal"}) {
		t.Errorf("tags = %v, want [personal]", updated.Tags)
	}

	round, err := svc.Get(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if round.Row.Account != "savings" {
		t.Errorf("account = %q, want savings", round.Row.Account)
	}
}

func TestUpdate_TagReplacementPrunesOrphans(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Row.ID, finance.CreateInput{
		Amount: 20.00,
		Kind:   finance.KindExpense,
		Tags:   []string{"ddd", "patterns"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !equalStrings(updated.Tags, []string{"ddd", "patterns"}) {
		t.Errorf("after Update tags = %v", updated.Tags)
	}

	q := store.New(db)
	tags, err := q.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	names := make(map[string]bool, len(tags))
	for _, tag := range tags {
		names[tag.Name] = true
	}
	for _, want := range []string{"ddd", "patterns"} {
		if !names[want] {
			t.Errorf("missing tag %q", want)
		}
	}
	for _, gone := range []string{"work"} {
		if names[gone] {
			t.Errorf("orphan tag %q should have been pruned", gone)
		}
	}
}

func TestUpdate_NonExistentFails(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Update(ctx, 999, finance.CreateInput{
		Amount: 10.00,
		Kind:   finance.KindExpense,
	}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSoftDelete_Active(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	q := store.New(db)
	row, err := q.GetFinanceTransaction(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetFinanceTransaction: %v", err)
	}
	if !row.DeletedAt.Valid {
		t.Fatal("expected deleted_at to be set")
	}
}

func TestSoftDelete_AlreadyDeleted(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete again: %v", err)
	}
}

func TestRestore_SoftDeleted(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	if err := svc.Restore(ctx, created.Row.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	q := store.New(db)
	row, err := q.GetFinanceTransaction(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetFinanceTransaction: %v", err)
	}
	if row.DeletedAt.Valid {
		t.Fatal("expected deleted_at to be NULL after restore")
	}
}

func TestRestore_Active(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Restore(ctx, created.Row.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	q := store.New(db)
	row, err := q.GetFinanceTransaction(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetFinanceTransaction: %v", err)
	}
	if row.DeletedAt.Valid {
		t.Fatal("expected deleted_at to be nil")
	}
}

func TestPurge_AfterSoftDelete(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	if err := svc.Purge(ctx, created.Row.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	q := store.New(db)
	if _, err := q.GetFinanceTransaction(ctx, created.Row.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestPurge_Active(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Purge(ctx, created.Row.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	q := store.New(db)
	if _, err := q.GetFinanceTransaction(ctx, created.Row.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestPurge_RemovesOrphanTags(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	created, err := svc.Create(ctx, finance.CreateInput{
		Amount: 10.00,
		Kind:   finance.KindExpense,
		Tags:   []string{"only-for-this"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Purge(ctx, created.Row.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	q := store.New(db)
	tags, err := q.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected 0 tags after purge, got %d", len(tags))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestList_DefaultActive(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	active, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-10",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "active",
	})
	if err != nil {
		t.Fatalf("Create active: %v", err)
	}
	trashed, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-11",
		Amount:  20.00,
		Kind:    finance.KindExpense,
		Account: "trashed",
	})
	if err != nil {
		t.Fatalf("Create trashed: %v", err)
	}
	if err := svc.SoftDelete(ctx, trashed.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	got, err := svc.List(ctx, finance.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != active.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, active.Row.ID)
	}
}

func TestList_FilterKind(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-10",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "expense",
	}); err != nil {
		t.Fatalf("Create expense: %v", err)
	}
	income, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-11",
		Amount:  100.00,
		Kind:    finance.KindIncome,
		Account: "income",
	})
	if err != nil {
		t.Fatalf("Create income: %v", err)
	}

	kind := "income"
	got, err := svc.List(ctx, finance.Filter{Kind: &kind})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != income.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, income.Row.ID)
	}
}

func TestList_FilterDateRange(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-01",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "early",
	}); err != nil {
		t.Fatalf("Create early: %v", err)
	}
	mid, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-15",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "mid",
	})
	if err != nil {
		t.Fatalf("Create mid: %v", err)
	}

	from := "2026-06-10"
	to := "2026-06-20"
	got, err := svc.List(ctx, finance.Filter{From: &from, To: &to})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != mid.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, mid.Row.ID)
	}
}

func TestList_FilterTag(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	tagged, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-13",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "tagged",
		Tags:    []string{"urgent"},
	})
	if err != nil {
		t.Fatalf("Create tagged: %v", err)
	}
	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-13",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "untagged",
	}); err != nil {
		t.Fatalf("Create untagged: %v", err)
	}

	got, err := svc.List(ctx, finance.Filter{TagName: "urgent"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != tagged.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, tagged.Row.ID)
	}
}

func TestList_FilterCategory(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)
	q := store.New(db)

	cat, err := q.CreateCategory(ctx, store.CreateCategoryParams{Slug: "food", Name: "Food", Icon: "", SortOrder: 1})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	withCat, err := svc.Create(ctx, finance.CreateInput{
		Date:       "2026-06-13",
		Amount:     10.00,
		Kind:       finance.KindExpense,
		CategoryID: &cat.ID,
	})
	if err != nil {
		t.Fatalf("Create with cat: %v", err)
	}
	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-13",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "no cat",
	}); err != nil {
		t.Fatalf("Create no cat: %v", err)
	}

	got, err := svc.List(ctx, finance.Filter{CategorySlug: "food"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != withCat.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, withCat.Row.ID)
	}
}

func TestList_FilterTrashed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	trashed, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-13",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "trashed",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, trashed.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-13",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "active",
	}); err != nil {
		t.Fatalf("Create active: %v", err)
	}

	got, err := svc.List(ctx, finance.Filter{Trashed: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != trashed.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, trashed.Row.ID)
	}
}

func TestList_SortOrder(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-01",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "old",
	}); err != nil {
		t.Fatalf("Create old: %v", err)
	}
	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-03",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "new",
	}); err != nil {
		t.Fatalf("Create new: %v", err)
	}

	got, err := svc.List(ctx, finance.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Row.Account != "new" || got[1].Row.Account != "old" {
		t.Errorf("order = %v, want [new, old]", []string{got[0].Row.Account, got[1].Row.Account})
	}
}

func TestList_Pagination(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	for i := 0; i < 5; i++ {
		if _, err := svc.Create(ctx, finance.CreateInput{
			Date:    "2026-06-13",
			Amount:  10.00,
			Kind:    finance.KindExpense,
			Account: "x",
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	got, err := svc.List(ctx, finance.Filter{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestExport(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)
	q := store.New(db)

	cat, err := q.CreateCategory(ctx, store.CreateCategoryParams{Slug: "food", Name: "Food", Icon: "", SortOrder: 1})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:       "2026-06-13",
		Amount:     42.50,
		Kind:       finance.KindExpense,
		Account:    "checking",
		CategoryID: &cat.ID,
		Notes:      "lunch",
		Tags:       []string{"work"},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Export(ctx, finance.Filter{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	row := got[0]
	if row.Date != "2026-06-13" {
		t.Errorf("date = %q, want 2026-06-13", row.Date)
	}
	if row.Kind != "expense" {
		t.Errorf("kind = %q, want expense", row.Kind)
	}
	if row.Amount != 42.50 {
		t.Errorf("amount = %v, want 42.50", row.Amount)
	}
	if row.Currency != "USD" {
		t.Errorf("currency = %q, want USD", row.Currency)
	}
	if row.Account != "checking" {
		t.Errorf("account = %q, want checking", row.Account)
	}
	if row.Category != "Food" {
		t.Errorf("category = %q, want Food", row.Category)
	}
	if row.Notes != "lunch" {
		t.Errorf("notes = %q, want lunch", row.Notes)
	}
	if !equalStrings(row.Tags, []string{"work"}) {
		t.Errorf("tags = %v, want [work]", row.Tags)
	}
}

func TestExport_WithFilter(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-10",
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "a",
		Tags:    []string{"x"},
	}); err != nil {
		t.Fatalf("Create a: %v", err)
	}
	if _, err := svc.Create(ctx, finance.CreateInput{
		Date:    "2026-06-11",
		Amount:  20.00,
		Kind:    finance.KindIncome,
		Account: "b",
		Tags:    []string{"y"},
	}); err != nil {
		t.Fatalf("Create b: %v", err)
	}

	kind := "income"
	got, err := svc.Export(ctx, finance.Filter{Kind: &kind})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Kind != "income" {
		t.Errorf("kind = %q, want income", got[0].Kind)
	}
	if !equalStrings(got[0].Tags, []string{"y"}) {
		t.Errorf("tags = %v, want [y]", got[0].Tags)
	}
}

func TestList_SearchNotes(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Create(ctx, finance.CreateInput{
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "a",
		Notes:   "monthly grocery run",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, finance.CreateInput{
		Amount:  20.00,
		Kind:    finance.KindExpense,
		Account: "b",
		Notes:   "subscription renewal",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, finance.Filter{Search: "grocery"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.Account != "a" {
		t.Errorf("account = %q, want a", got[0].Row.Account)
	}
}

func TestList_SearchAccount(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	if _, err := svc.Create(ctx, finance.CreateInput{
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "banco nación",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, finance.CreateInput{
		Amount:  20.00,
		Kind:    finance.KindExpense,
		Account: "cash",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, finance.Filter{Search: "nación"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.Account != "banco nación" {
		t.Errorf("account = %q, want banco nación", got[0].Row.Account)
	}
}

func TestList_SearchExcludesTrashed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	trashed, err := svc.Create(ctx, finance.CreateInput{
		Amount:  10.00,
		Kind:    finance.KindExpense,
		Account: "a",
		Notes:   "searchable note",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, trashed.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, err := svc.Create(ctx, finance.CreateInput{
		Amount:  20.00,
		Kind:    finance.KindExpense,
		Account: "b",
		Notes:   "searchable note",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, finance.Filter{Search: "searchable"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.Account != "b" {
		t.Errorf("account = %q, want b", got[0].Row.Account)
	}
}

func TestExport_Empty(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := finance.New(db)

	got, err := svc.Export(ctx, finance.Filter{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestWithClock(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	fixed := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	svc := finance.New(db, finance.WithClock(func() time.Time { return fixed }))

	got, err := svc.Create(ctx, finance.CreateInput{
		Amount: 10.00,
		Kind:   finance.KindExpense,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.Date != "2026-06-15" {
		t.Errorf("date = %q, want 2026-06-15", got.Row.Date)
	}
}

func TestOrphanCleanup_CoversAllDomains(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	resSvc := resources.New(db)
	todoSvc := todos.New(db)
	finSvc := finance.New(db)

	res, err := resSvc.Create(ctx, resources.CreateInput{
		Title:    "res",
		URL:      "https://example.com/res",
		Type:     domain.TypeArticle,
		Language: domain.LangEN,
		Tags:     []string{"res-only"},
	})
	if err != nil {
		t.Fatalf("Create resource: %v", err)
	}
	todo, err := todoSvc.Create(ctx, todos.CreateInput{
		Title: "todo",
		Tags:  []string{"todo-only"},
	})
	if err != nil {
		t.Fatalf("Create todo: %v", err)
	}
	fin, err := finSvc.Create(ctx, finance.CreateInput{
		Amount: 10.00,
		Kind:   finance.KindExpense,
		Tags:   []string{"finance-only"},
	})
	if err != nil {
		t.Fatalf("Create finance: %v", err)
	}

	if err := resSvc.Purge(ctx, res.Row.ID); err != nil {
		t.Fatalf("Purge resource: %v", err)
	}
	if err := todoSvc.Purge(ctx, todo.Row.ID); err != nil {
		t.Fatalf("Purge todo: %v", err)
	}
	if err := finSvc.Purge(ctx, fin.Row.ID); err != nil {
		t.Fatalf("Purge finance: %v", err)
	}

	q := store.New(db)
	tags, err := q.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected 0 orphan tags across all domains, got %v", tags)
	}
}
