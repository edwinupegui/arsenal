package resources

import (
	"context"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/store"
)

// Attacher is the resources-specific implementation of domain.Attacher. It
// translates the generic OwnerID into the resource_id column used by the
// resource_tags join table.
//
// The adapter holds a *store.Queries that is either bound to a *sql.DB
// (standalone use) or to a *sql.Tx (caller-owned transaction). The standard
// pattern in this package is:
//
//	att := resources.NewAttacher(s.q.WithTx(tx))
//	err := domain.WithTags(ctx, s.db, tx, att, in)
type Attacher struct {
	q *store.Queries
}

// NewAttacher returns an Attacher backed by the given *store.Queries.
func NewAttacher(q *store.Queries) *Attacher { return &Attacher{q: q} }

// UpsertTag delegates to store.Queries.UpsertTag and returns the id+name pair
// in the domain.TagRef shape the helper expects.
func (a *Attacher) UpsertTag(ctx context.Context, name string) (domain.TagRef, error) {
	t, err := a.q.UpsertTag(ctx, name)
	if err != nil {
		return domain.TagRef{}, err
	}
	return domain.TagRef{ID: t.ID, Name: t.Name}, nil
}

// AttachTagToOwner links the tag to the resource identified by OwnerID.
func (a *Attacher) AttachTagToOwner(ctx context.Context, tagID, ownerID int64) error {
	return a.q.AttachTag(ctx, store.AttachTagParams{
		ResourceID: ownerID,
		TagID:      tagID,
	})
}

// DeleteOrphanTags removes tag rows that no resource_tags row references.
// Covers resource_tags and todo_tags; extend the UNION when new tag-attachable
// domains are added in phase 3+.
func (a *Attacher) DeleteOrphanTags(ctx context.Context) error {
	return a.q.DeleteOrphanTags(ctx)
}
