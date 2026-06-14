package calendar

import (
	"context"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/store"
)

// Attacher is the calendar-specific implementation of domain.Attacher.
type Attacher struct {
	q *store.Queries
}

// NewAttacher returns an Attacher backed by the given *store.Queries.
func NewAttacher(q *store.Queries) *Attacher { return &Attacher{q: q} }

// UpsertTag delegates to store.Queries.UpsertTag and returns the id+name pair.
func (a *Attacher) UpsertTag(ctx context.Context, name string) (domain.TagRef, error) {
	t, err := a.q.UpsertTag(ctx, name)
	if err != nil {
		return domain.TagRef{}, err
	}
	return domain.TagRef{ID: t.ID, Name: t.Name}, nil
}

// AttachTagToOwner links the tag to the calendar event identified by OwnerID.
func (a *Attacher) AttachTagToOwner(ctx context.Context, tagID, ownerID int64) error {
	return a.q.AttachTagToCalendar(ctx, store.AttachTagToCalendarParams{
		EventID: ownerID,
		TagID:   tagID,
	})
}

// DeleteOrphanTags removes tag rows that no junction table references.
// Delegates to the shared query which covers resources, todos, finance, and calendar.
func (a *Attacher) DeleteOrphanTags(ctx context.Context) error {
	return a.q.DeleteOrphanTags(ctx)
}
