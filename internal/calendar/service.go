package calendar

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/sqliteutil"
	"github.com/edwinupegui/arsenal/internal/store"
)

// Event is the service-layer view of a stored calendar event together with its
// resolved tags.
type Event struct {
	Row  store.CalendarEvent
	Tags []string
}

// Service exposes calendar event lifecycle operations.
type Service struct {
	db  *sql.DB
	q   *store.Queries
	now func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithClock replaces the default time.Now source. Used by tests to pin the
// wall-clock for date-sensitive comparisons.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New builds a Service bound to db.
func New(db *sql.DB, opts ...Option) *Service {
	s := &Service{db: db, q: store.New(db), now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create validates input, opens a transaction, inserts the event, upserts and
// attaches tags, and commits.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Event, error) {
	if in.Recurrence == "" {
		in.Recurrence = RecurrenceNone
	}
	if err := validateCreate(in); err != nil {
		return nil, err
	}
	tags, err := domain.NormalizeTags(in.Tags)
	if err != nil {
		return nil, err
	}

	var out *Event
	err = sqliteutil.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		row, err := q.CreateCalendarEvent(ctx, store.CreateCalendarEventParams{
			Title:       strings.TrimSpace(in.Title),
			Description: nullableStr(in.Description),
			StartAt:     in.StartAt,
			EndAt:       nullableStr(in.EndAt),
			AllDay:      boolToInt(in.AllDay),
			Location:    in.Location,
			CategoryID:  nullableInt64(in.CategoryID),
			Notes:       nullableStr(in.Notes),
			Recurrence:  string(in.Recurrence),
		})
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		att := NewAttacher(q)
		if err := domain.WithTags(ctx, s.db, tx, att, domain.AttachInput{
			OwnerKind:    "calendar",
			OwnerID:      row.ID,
			Tags:         tags,
			PruneOrphans: false,
		}); err != nil {
			return err
		}
		out = &Event{Row: row, Tags: tags}
		return nil
	})
	return out, err
}

// Get returns an event and its tags. Returns sql.ErrNoRows if not found.
func (s *Service) Get(ctx context.Context, id int64) (*Event, error) {
	row, err := s.q.GetCalendarEvent(ctx, id)
	if err != nil {
		return nil, err
	}
	tags, err := s.tagsFor(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Event{Row: row, Tags: tags}, nil
}

// Update replaces the mutable fields of event id. Tag updates are handled as
// detach-all-then-reattach with orphan pruning.
func (s *Service) Update(ctx context.Context, id int64, in CreateInput) (*Event, error) {
	if in.Recurrence == "" {
		in.Recurrence = RecurrenceNone
	}
	if err := validateCreate(in); err != nil {
		return nil, err
	}
	tags, err := domain.NormalizeTags(in.Tags)
	if err != nil {
		return nil, err
	}

	var out *Event
	err = sqliteutil.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		// Verify the event exists before updating.
		if _, err := q.GetCalendarEvent(ctx, id); err != nil {
			return fmt.Errorf("get event: %w", err)
		}
		row, err := q.UpdateCalendarEvent(ctx, store.UpdateCalendarEventParams{
			Title:       strings.TrimSpace(in.Title),
			Description: nullableStr(in.Description),
			StartAt:     in.StartAt,
			EndAt:       nullableStr(in.EndAt),
			AllDay:      boolToInt(in.AllDay),
			Location:    in.Location,
			CategoryID:  nullableInt64(in.CategoryID),
			Notes:       nullableStr(in.Notes),
			Recurrence:  string(in.Recurrence),
			ID:          id,
		})
		if err != nil {
			return fmt.Errorf("update event: %w", err)
		}
		if err := q.DetachAllTagsFromCalendar(ctx, id); err != nil {
			return fmt.Errorf("detach tags: %w", err)
		}
		att := NewAttacher(q)
		if err := domain.WithTags(ctx, s.db, tx, att, domain.AttachInput{
			OwnerKind:    "calendar",
			OwnerID:      id,
			Tags:         tags,
			PruneOrphans: true,
		}); err != nil {
			return err
		}
		out = &Event{Row: row, Tags: tags}
		return nil
	})
	return out, err
}

// SoftDelete moves an event to the trash by setting deleted_at.
func (s *Service) SoftDelete(ctx context.Context, id int64) error {
	return s.q.SoftDeleteCalendarEvent(ctx, id)
}

// Restore clears deleted_at on a trashed event.
func (s *Service) Restore(ctx context.Context, id int64) error {
	return s.q.RestoreCalendarEvent(ctx, id)
}

// Purge permanently deletes an event and (via FK cascades) its tag links.
// Orphan tags are pruned inside the same transaction.
func (s *Service) Purge(ctx context.Context, id int64) error {
	return sqliteutil.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.PurgeCalendarEvent(ctx, id); err != nil {
			return fmt.Errorf("purge event: %w", err)
		}
		att := NewAttacher(q)
		if err := domain.WithTags(ctx, s.db, tx, att, domain.AttachInput{
			OwnerKind:    "calendar",
			OwnerID:      id,
			Tags:         nil,
			PruneOrphans: true,
		}); err != nil {
			return err
		}
		return nil
	})
}

// List returns events matching the filter. When Search is non-empty, it
// delegates to FTS5 via SearchCalendar.
func (s *Service) List(ctx context.Context, f Filter) ([]*Event, error) {
	if f.Search != "" {
		limit := int64(f.Limit)
		if limit <= 0 {
			limit = 50
		}
		rows, err := s.q.SearchCalendar(ctx, f.Search, limit)
		if err != nil {
			return nil, err
		}
		out := make([]*Event, len(rows))
		for i, lc := range rows {
			out[i] = &Event{Row: lc.Calendar, Tags: lc.Tags}
		}
		return out, nil
	}

	rows, err := s.q.ListCalendarFiltered(ctx, store.CalendarListFilter{
		From:         f.From,
		To:           f.To,
		Recurrence:   f.Recurrence,
		CategorySlug: f.CategorySlug,
		TagName:      f.TagName,
		Trashed:      f.Trashed,
		Limit:        f.Limit,
		Offset:       f.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*Event, len(rows))
	for i, lc := range rows {
		out[i] = &Event{Row: lc.Calendar, Tags: lc.Tags}
	}
	return out, nil
}

// Export returns rows matching the filter, with category names resolved.
func (s *Service) Export(ctx context.Context, f Filter) ([]ExportRow, error) {
	rows, err := s.q.ListCalendarFiltered(ctx, store.CalendarListFilter{
		From:         f.From,
		To:           f.To,
		Recurrence:   f.Recurrence,
		CategorySlug: f.CategorySlug,
		TagName:      f.TagName,
		Trashed:      f.Trashed,
		Limit:        f.Limit,
		Offset:       f.Offset,
	})
	if err != nil {
		return nil, err
	}

	out := make([]ExportRow, len(rows))
	for i, lc := range rows {
		category := ""
		if lc.CategoryName.Valid {
			category = lc.CategoryName.String
		}
		description := ""
		if lc.Calendar.Description.Valid {
			description = lc.Calendar.Description.String
		}
		endAt := ""
		if lc.Calendar.EndAt.Valid {
			endAt = lc.Calendar.EndAt.String
		}
		notes := ""
		if lc.Calendar.Notes.Valid {
			notes = lc.Calendar.Notes.String
		}
		out[i] = ExportRow{
			ID:          lc.Calendar.ID,
			Title:       lc.Calendar.Title,
			Description: description,
			StartAt:     lc.Calendar.StartAt,
			EndAt:       endAt,
			AllDay:      lc.Calendar.AllDay == 1,
			Location:    lc.Calendar.Location,
			Category:    category,
			Notes:       notes,
			Recurrence:  Recurrence(lc.Calendar.Recurrence),
			Tags:        lc.Tags,
			CreatedAt:   lc.Calendar.CreatedAt,
		}
	}
	return out, nil
}

// tagsFor returns the tag names attached to an event, sorted by name.
func (s *Service) tagsFor(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.q.ListTagsForCalendar(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	out := make([]string, len(rows))
	for i, t := range rows {
		out[i] = t.Name
	}
	return out, nil
}

// nullableStr converts an empty string to a SQL NULL NullString.
func nullableStr(s string) sql.NullString {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullableInt64 converts a nil *int64 to a SQL NULL NullInt64.
func nullableInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}
