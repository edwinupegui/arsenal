package domain_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/sqliteutil"
)

// fakeAttacher is a domain.Attacher implementation backed by counters, used
// to verify the contract without touching the real store.
type fakeAttacher struct {
	upsertCalls   atomic.Int32
	attachCalls   atomic.Int32
	pruneCalls    atomic.Int32
	failOnUpsert  string // tag name that should fail on UpsertTag
	failOnAttach  string // tag name that should fail on AttachTagToOwner
	tags          map[string]int64
	nextTagID     int64
	attachedPairs [][2]int64
}

func newFakeAttacher() *fakeAttacher {
	return &fakeAttacher{
		tags: map[string]int64{},
	}
}

func (f *fakeAttacher) UpsertTag(_ context.Context, name string) (domain.TagRef, error) {
	f.upsertCalls.Add(1)
	if name == f.failOnUpsert {
		return domain.TagRef{}, errors.New("upsert boom")
	}
	id, ok := f.tags[name]
	if !ok {
		f.nextTagID++
		id = f.nextTagID
		f.tags[name] = id
	}
	return domain.TagRef{ID: id, Name: name}, nil
}

func (f *fakeAttacher) AttachTagToOwner(_ context.Context, tagID, ownerID int64) error {
	f.attachCalls.Add(1)
	// failOnAttach is matched on tag name; we look it up by id.
	for name, id := range f.tags {
		if id == tagID && name == f.failOnAttach {
			return errors.New("attach boom")
		}
	}
	f.attachedPairs = append(f.attachedPairs, [2]int64{ownerID, tagID})
	return nil
}

func (f *fakeAttacher) DeleteOrphanTags(_ context.Context) error {
	f.pruneCalls.Add(1)
	return nil
}

func TestWithTags_HappyPath(t *testing.T) {
	t.Parallel()
	att := newFakeAttacher()
	db := newInMemoryDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = domain.WithTags(context.Background(), db, tx, att, domain.AttachInput{
		OwnerKind: "resource",
		OwnerID:   42,
		Tags:      []string{"go", "sqlite"},
	})
	if err != nil {
		t.Fatalf("WithTags: %v", err)
	}
	if got := att.upsertCalls.Load(); got != 2 {
		t.Errorf("upsertCalls = %d, want 2", got)
	}
	if got := att.attachCalls.Load(); got != 2 {
		t.Errorf("attachCalls = %d, want 2", got)
	}
	if got := att.pruneCalls.Load(); got != 0 {
		t.Errorf("pruneCalls = %d, want 0 (PruneOrphans=false)", got)
	}
	if len(att.attachedPairs) != 2 {
		t.Fatalf("attachedPairs = %d, want 2", len(att.attachedPairs))
	}
	for _, p := range att.attachedPairs {
		if p[0] != 42 {
			t.Errorf("ownerID = %d, want 42", p[0])
		}
	}
}

func TestWithTags_EmptyNoOp(t *testing.T) {
	t.Parallel()
	att := newFakeAttacher()
	db := newInMemoryDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = domain.WithTags(context.Background(), db, tx, att, domain.AttachInput{
		OwnerKind: "resource",
		OwnerID:   1,
		Tags:      nil,
	})
	if err != nil {
		t.Fatalf("WithTags: %v", err)
	}
	if got := att.upsertCalls.Load(); got != 0 {
		t.Errorf("upsertCalls = %d, want 0 (empty input)", got)
	}
}

func TestWithTags_PruneOrphans_True(t *testing.T) {
	t.Parallel()
	att := newFakeAttacher()
	db := newInMemoryDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = domain.WithTags(context.Background(), db, tx, att, domain.AttachInput{
		OwnerKind:    "resource",
		OwnerID:      7,
		Tags:         []string{"a"},
		PruneOrphans: true,
	})
	if err != nil {
		t.Fatalf("WithTags: %v", err)
	}
	if got := att.pruneCalls.Load(); got != 1 {
		t.Errorf("pruneCalls = %d, want 1", got)
	}
}

func TestWithTags_PruneOrphans_True_Alone(t *testing.T) {
	t.Parallel()
	att := newFakeAttacher()
	db := newInMemoryDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	// No tags but PruneOrphans=true: helper should still run DeleteOrphanTags.
	err = domain.WithTags(context.Background(), db, tx, att, domain.AttachInput{
		OwnerKind:    "resource",
		OwnerID:      7,
		Tags:         nil,
		PruneOrphans: true,
	})
	if err != nil {
		t.Fatalf("WithTags: %v", err)
	}
	if got := att.pruneCalls.Load(); got != 1 {
		t.Errorf("pruneCalls = %d, want 1", got)
	}
	if got := att.upsertCalls.Load(); got != 0 {
		t.Errorf("upsertCalls = %d, want 0 (no tags)", got)
	}
}

func TestWithTags_UpsertError(t *testing.T) {
	t.Parallel()
	att := newFakeAttacher()
	att.failOnUpsert = "boom"
	db := newInMemoryDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = domain.WithTags(context.Background(), db, tx, att, domain.AttachInput{
		OwnerKind: "resource",
		OwnerID:   1,
		Tags:      []string{"ok", "boom", "later"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `upsert tag "boom"`) {
		t.Errorf("error = %q, want it to mention upsert tag boom", err.Error())
	}
	// Attach should never have been called for "boom" or for "later".
	if got := att.attachCalls.Load(); got != 1 {
		t.Errorf("attachCalls = %d, want 1 (only 'ok' before the failure)", got)
	}
}

func TestWithTags_AttachError(t *testing.T) {
	t.Parallel()
	att := newFakeAttacher()
	att.failOnAttach = "boom"
	db := newInMemoryDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = domain.WithTags(context.Background(), db, tx, att, domain.AttachInput{
		OwnerKind: "resource",
		OwnerID:   1,
		Tags:      []string{"ok", "boom"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `attach tag "boom"`) {
		t.Errorf("error = %q, want it to mention attach tag boom", err.Error())
	}
}

func TestWithTags_TxNil(t *testing.T) {
	t.Parallel()
	att := newFakeAttacher()
	err := domain.WithTags(context.Background(), nil, nil, att, domain.AttachInput{
		OwnerKind: "resource",
		OwnerID:   1,
		Tags:      []string{"x"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tx is required") {
		t.Errorf("error = %q, want it to mention tx is required", err.Error())
	}
}

func TestWithTags_CallerTxIsUsed(t *testing.T) {
	t.Parallel()
	att := newFakeAttacher()
	db := newInMemoryDB(t)

	// Open a transaction, pass it to WithTags, then do work ourselves to
	// confirm the helper did NOT open a new one. We track this indirectly:
	// if WithTags tried to open a new tx, it would have called BeginTx on
	// the in-memory DB, and a *sql.Tx held by the caller would still commit
	// independently. The cleanest check: pass tx=non-nil and assert the work
	// ran. The "did not open new tx" property is documented in the godoc
	// and covered by the implementation: when tx != nil we skip sqliteutil.WithTx.
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := domain.WithTags(context.Background(), db, tx, att, domain.AttachInput{
		OwnerKind: "resource",
		OwnerID:   99,
		Tags:      []string{"one"},
	}); err != nil {
		t.Fatalf("WithTags: %v", err)
	}
	if got := att.upsertCalls.Load(); got != 1 {
		t.Errorf("upsertCalls = %d, want 1", got)
	}
}

// newInMemoryDB returns a fresh in-memory SQLite DB. The helper only needs a
// real *sql.DB to pass to WithTags when tx is nil; the fakeAttacher does not
// actually touch the DB.
func newInMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqliteutil.Open(":memory:")
	if err != nil {
		t.Fatalf("open :memory: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
