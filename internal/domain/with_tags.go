package domain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// TagRef is the minimum surface WithTags needs from an upserted tag. Adapters
// translate whatever their sqlc-generated UpsertTag returns into this shape.
type TagRef struct {
	ID   int64
	Name string
}

// Attacher is the store-side contract that every domain (resources, todos,
// finance, calendar) must satisfy to use WithTags. Adapters live next to the
// store they wrap (e.g., internal/resources/attacher.go) and translate the
// generic OwnerID into the domain-specific column name (resource_id,
// todo_id, ...).
//
// IMPORTANT: when the caller is inside a transaction, the adapter passed to
// WithTags must already be bound to that tx. The standard pattern is:
//
//	err := sqliteutil.WithTx(ctx, db, func(tx *sql.Tx) error {
//	    q := store.New(db).WithTx(tx)
//	    att := resources.NewAttacher(q)
//	    if err := domain.WithTags(ctx, db, tx, att, in); err != nil { return err }
//	    // ... other work that also uses q
//	    return nil
//	})
//
type Attacher interface {
	UpsertTag(ctx context.Context, name string) (TagRef, error)
	AttachTagToOwner(ctx context.Context, tagID, ownerID int64) error
	DeleteOrphanTags(ctx context.Context) error
}

// AttachInput describes one attach-tags operation. Tags must already be
// normalized via NormalizeTags.
type AttachInput struct {
	OwnerKind    string // "resource" | "todo" | ... used only in error messages
	OwnerID      int64
	Tags         []string
	PruneOrphans bool
}

// WithTags attaches tags to an owner.
//
// tx is REQUIRED. The caller must open a transaction (typically via
// sqliteutil.WithTx) and pass it here. The adapter passed in MUST already
// be bound to that same tx (typically via store.New(db).WithTx(tx)).
// WithTags does not call WithTx on the adapter itself; the caller is
// responsible for adapter setup. This keeps the helper free of any specific
// sqlc-generated type.
func WithTags(ctx context.Context, db *sql.DB, tx *sql.Tx, att Attacher, in AttachInput) error {
	if tx == nil {
		return errors.New("WithTags: tx is required; if you need a standalone transaction, open one with sqliteutil.WithTx before calling")
	}
	_ = db // db is kept in the signature for backward compatibility with callers

	if len(in.Tags) == 0 && !in.PruneOrphans {
		return nil
	}

	for _, name := range in.Tags {
		tag, err := att.UpsertTag(ctx, name)
		if err != nil {
			return fmt.Errorf("%s: upsert tag %q: %w", in.OwnerKind, name, err)
		}
		if err := att.AttachTagToOwner(ctx, tag.ID, in.OwnerID); err != nil {
			return fmt.Errorf("%s: attach tag %q: %w", in.OwnerKind, name, err)
		}
	}
	if in.PruneOrphans {
		if err := att.DeleteOrphanTags(ctx); err != nil {
			return fmt.Errorf("%s: prune orphan tags: %w", in.OwnerKind, err)
		}
	}
	return nil
}
