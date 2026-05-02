// Package resources is the application service layer over the sqlc store.
// It owns input validation, tag resolution, and transactional consistency
// for create/update/delete flows. Both `arsenal add` and `arsenal migrate`
// route through this package so the business rules live in one place.
package resources

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/store"
)

// Service exposes resource lifecycle operations. It is safe to share across
// goroutines — the underlying *sql.DB is concurrency-safe and each method
// owns its own transaction.
type Service struct {
	db *sql.DB
	q  *store.Queries
}

// New builds a Service bound to db. Callers are responsible for the DB lifecycle.
func New(db *sql.DB) *Service {
	return &Service{db: db, q: store.New(db)}
}

// Resource is the service-layer view of a stored resource together with its
// resolved tags. Callers shouldn't need to touch the sqlc structs.
type Resource struct {
	Row  store.Resource
	Tags []string
}

// CreateInput captures everything needed to insert a new resource.
type CreateInput struct {
	Title       string
	URL         string
	Description string
	Type        domain.ResourceType
	Language    domain.Language
	CategoryID  *int64 // optional FK; nil = uncategorized
	Notes       string
	Favorite    bool
	Tags        []string
}

// ImportInput extends CreateInput with the timestamps the migrate command
// must preserve when copying rows from a legacy database.
type ImportInput struct {
	CreateInput
	CreatedAt string  // ISO-8601 string (matches column type)
	UpdatedAt string  // ISO-8601; usually equal to CreatedAt for legacy rows
	DeletedAt *string // soft-delete timestamp; nil means active
}

// UpdateInput captures the mutable fields of an existing resource.
type UpdateInput struct {
	Title       string
	URL         string
	Description string
	Type        domain.ResourceType
	Language    domain.Language
	CategoryID  *int64
	Notes       string
	Tags        []string
}

// Create validates input, opens a transaction, inserts the resource,
// upserts and attaches tags, and commits. On any error the transaction is
// rolled back and no rows are written.
func (s *Service) Create(ctx context.Context, in CreateInput) (Resource, error) {
	if err := validateCreate(in); err != nil {
		return Resource{}, err
	}
	tags, err := domain.NormalizeTags(in.Tags)
	if err != nil {
		return Resource{}, err
	}

	var out Resource
	err = withTx(ctx, s.db, func(q *store.Queries) error {
		row, err := q.CreateResource(ctx, store.CreateResourceParams{
			Title:       strings.TrimSpace(in.Title),
			Url:         strings.TrimSpace(in.URL),
			Description: nullableString(in.Description),
			Type:        string(in.Type),
			Language:    string(in.Language),
			CategoryID:  nullableInt64(in.CategoryID),
			Notes:       nullableString(in.Notes),
			Favorite:    boolToInt64(in.Favorite),
		})
		if err != nil {
			return fmt.Errorf("insert resource: %w", err)
		}
		if err := attachTags(ctx, q, row.ID, tags); err != nil {
			return err
		}
		out = Resource{Row: row, Tags: tags}
		return nil
	})
	return out, err
}

// Import is the migrate-only variant of Create that preserves the original
// created_at / updated_at / deleted_at values from a legacy row.
func (s *Service) Import(ctx context.Context, in ImportInput) (Resource, error) {
	if err := validateCreate(in.CreateInput); err != nil {
		return Resource{}, err
	}
	if strings.TrimSpace(in.CreatedAt) == "" {
		return Resource{}, errors.New("created_at is required for Import")
	}
	if strings.TrimSpace(in.UpdatedAt) == "" {
		in.UpdatedAt = in.CreatedAt
	}
	tags, err := domain.NormalizeTags(in.Tags)
	if err != nil {
		return Resource{}, err
	}

	var out Resource
	err = withTx(ctx, s.db, func(q *store.Queries) error {
		row, err := q.CreateResourceWithTimestamps(ctx, store.CreateResourceWithTimestampsParams{
			Title:       strings.TrimSpace(in.Title),
			Url:         strings.TrimSpace(in.URL),
			Description: nullableString(in.Description),
			Type:        string(in.Type),
			Language:    string(in.Language),
			CategoryID:  nullableInt64(in.CategoryID),
			Notes:       nullableString(in.Notes),
			Favorite:    boolToInt64(in.Favorite),
			CreatedAt:   in.CreatedAt,
			UpdatedAt:   in.UpdatedAt,
			DeletedAt:   nullableStringPtr(in.DeletedAt),
		})
		if err != nil {
			return fmt.Errorf("insert resource: %w", err)
		}
		if err := attachTags(ctx, q, row.ID, tags); err != nil {
			return err
		}
		out = Resource{Row: row, Tags: tags}
		return nil
	})
	return out, err
}

// Update replaces the mutable fields of resource id. Tag updates are handled
// as detach-all-then-reattach for simplicity — diffing 30-tag sets is not
// worth the complexity at this scale.
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (Resource, error) {
	if err := validateUpdate(in); err != nil {
		return Resource{}, err
	}
	tags, err := domain.NormalizeTags(in.Tags)
	if err != nil {
		return Resource{}, err
	}

	var out Resource
	err = withTx(ctx, s.db, func(q *store.Queries) error {
		row, err := q.UpdateResource(ctx, store.UpdateResourceParams{
			Title:       strings.TrimSpace(in.Title),
			Url:         strings.TrimSpace(in.URL),
			Description: nullableString(in.Description),
			Type:        string(in.Type),
			Language:    string(in.Language),
			CategoryID:  nullableInt64(in.CategoryID),
			Notes:       nullableString(in.Notes),
			ID:          id,
		})
		if err != nil {
			return fmt.Errorf("update resource: %w", err)
		}
		if err := q.DetachAllTagsFromResource(ctx, id); err != nil {
			return fmt.Errorf("detach tags: %w", err)
		}
		if err := attachTags(ctx, q, id, tags); err != nil {
			return err
		}
		// Drop any tag rows that no longer reference any resource. Keeps the
		// `tags` table tidy without scheduling a separate sweep.
		if err := q.DeleteOrphanTags(ctx); err != nil {
			return fmt.Errorf("prune orphan tags: %w", err)
		}
		out = Resource{Row: row, Tags: tags}
		return nil
	})
	return out, err
}

// SoftDelete moves a resource to the trash by setting deleted_at.
func (s *Service) SoftDelete(ctx context.Context, id int64) error {
	return s.q.SoftDeleteResource(ctx, id)
}

// Restore clears deleted_at on a trashed resource.
func (s *Service) Restore(ctx context.Context, id int64) error {
	return s.q.RestoreResource(ctx, id)
}

// Purge permanently deletes a resource and (via FK cascades) its tag links.
// Triggers fire DELETE on resources_fts so the search index stays in sync.
func (s *Service) Purge(ctx context.Context, id int64) error {
	return withTx(ctx, s.db, func(q *store.Queries) error {
		if err := q.PurgeResource(ctx, id); err != nil {
			return fmt.Errorf("purge resource: %w", err)
		}
		if err := q.DeleteOrphanTags(ctx); err != nil {
			return fmt.Errorf("prune orphan tags: %w", err)
		}
		return nil
	})
}

// SetFavorite flips the favorite flag.
func (s *Service) SetFavorite(ctx context.Context, id int64, fav bool) error {
	return s.q.SetFavorite(ctx, store.SetFavoriteParams{
		Favorite: boolToInt64(fav),
		ID:       id,
	})
}

// Get returns a resource and its tags. Returns sql.ErrNoRows if not found.
func (s *Service) Get(ctx context.Context, id int64) (Resource, error) {
	row, err := s.q.GetResource(ctx, id)
	if err != nil {
		return Resource{}, err
	}
	tags, err := s.tagsFor(ctx, id)
	if err != nil {
		return Resource{}, err
	}
	return Resource{Row: row, Tags: tags}, nil
}

// tagsFor returns the tag names attached to a resource, sorted by name.
func (s *Service) tagsFor(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.q.ListTagsForResource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out, nil
}

// --- internals ---------------------------------------------------------------

func validateCreate(in CreateInput) error {
	if err := domain.ValidateTitle(in.Title); err != nil {
		return err
	}
	if err := domain.ValidateURL(in.URL); err != nil {
		return err
	}
	if !in.Type.Valid() {
		return fmt.Errorf("invalid type %q", in.Type)
	}
	if in.Language == "" {
		// Empty language is a programmer bug, not a user one — surface it.
		return errors.New("language is required")
	}
	if !in.Language.Valid() {
		return fmt.Errorf("invalid language %q", in.Language)
	}
	return nil
}

func validateUpdate(in UpdateInput) error {
	return validateCreate(CreateInput{
		Title: in.Title, URL: in.URL, Description: in.Description,
		Type: in.Type, Language: in.Language, CategoryID: in.CategoryID,
		Notes: in.Notes, Tags: in.Tags,
	})
}

// attachTags upserts each tag and links it to resourceID. Tags must already
// be normalized by domain.NormalizeTags.
func attachTags(ctx context.Context, q *store.Queries, resourceID int64, tags []string) error {
	for _, name := range tags {
		tag, err := q.UpsertTag(ctx, name)
		if err != nil {
			return fmt.Errorf("upsert tag %q: %w", name, err)
		}
		if err := q.AttachTag(ctx, store.AttachTagParams{
			ResourceID: resourceID,
			TagID:      tag.ID,
		}); err != nil {
			return fmt.Errorf("attach tag %q: %w", name, err)
		}
	}
	return nil
}

// withTx runs fn inside a transaction, committing on success and rolling
// back (best-effort) on any error.
func withTx(ctx context.Context, db *sql.DB, fn func(*store.Queries) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	q := store.New(db).WithTx(tx)
	if err := fn(q); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func nullableString(s string) sql.NullString {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullableStringPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return nullableString(*s)
}

func nullableInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
