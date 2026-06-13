package finance

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/sqliteutil"
	"github.com/edwinupegui/arsenal/internal/store"
)

// Transaction is the service-layer view of a stored transaction together with
// its resolved tags.
type Transaction struct {
	Row  store.FinanceTransaction
	Tags []string
}

// Service exposes finance transaction lifecycle operations.
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

// Create validates input, opens a transaction, inserts the transaction,
// upserts and attaches tags, and commits.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Transaction, error) {
	if in.Recurrence == "" {
		in.Recurrence = RecurrenceNone
	}
	if in.Date == "" {
		in.Date = s.now().Format("2006-01-02")
	}
	if err := validateCreate(in); err != nil {
		return nil, err
	}
	tags, err := domain.NormalizeTags(in.Tags)
	if err != nil {
		return nil, err
	}

	currency, err := s.currency(ctx)
	if err != nil {
		return nil, err
	}

	var out *Transaction
	err = sqliteutil.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		row, err := q.CreateFinanceTransaction(ctx, store.CreateFinanceTransactionParams{
			Date:        in.Date,
			Amount:      in.Amount,
			Kind:        string(in.Kind),
			Account:     strings.TrimSpace(in.Account),
			CategoryID:  nullableInt64(in.CategoryID),
			Notes:       nullableString(in.Notes),
			Recurrence:  string(in.Recurrence),
			Currency:    currency,
		})
		if err != nil {
			return fmt.Errorf("insert transaction: %w", err)
		}
		att := NewAttacher(q)
		if err := domain.WithTags(ctx, s.db, tx, att, domain.AttachInput{
			OwnerKind:    "finance",
			OwnerID:      row.ID,
			Tags:         tags,
			PruneOrphans: false,
		}); err != nil {
			return err
		}
		out = &Transaction{Row: row, Tags: tags}
		return nil
	})
	return out, err
}

// Update replaces the mutable fields of transaction id. Tag updates are handled
// as detach-all-then-reattach with orphan pruning. Currency is never changed
// by an update; date is preserved when the input leaves it empty.
func (s *Service) Update(ctx context.Context, id int64, in CreateInput) (*Transaction, error) {
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

	var out *Transaction
	err = sqliteutil.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		current, err := q.GetFinanceTransaction(ctx, id)
		if err != nil {
			return fmt.Errorf("get transaction: %w", err)
		}

		date := in.Date
		if date == "" {
			date = current.Date
		}

		row, err := q.UpdateFinanceTransaction(ctx, store.UpdateFinanceTransactionParams{
			Date:        date,
			Amount:      in.Amount,
			Kind:        string(in.Kind),
			Account:     strings.TrimSpace(in.Account),
			CategoryID:  nullableInt64(in.CategoryID),
			Notes:       nullableString(in.Notes),
			Recurrence:  string(in.Recurrence),
			ID:          id,
		})
		if err != nil {
			return fmt.Errorf("update transaction: %w", err)
		}
		if err := q.DetachAllTagsFromFinance(ctx, id); err != nil {
			return fmt.Errorf("detach tags: %w", err)
		}
		att := NewAttacher(q)
		if err := domain.WithTags(ctx, s.db, tx, att, domain.AttachInput{
			OwnerKind:    "finance",
			OwnerID:      id,
			Tags:         tags,
			PruneOrphans: true,
		}); err != nil {
			return err
		}
		out = &Transaction{Row: row, Tags: tags}
		return nil
	})
	return out, err
}

// SoftDelete moves a transaction to the trash by setting deleted_at.
func (s *Service) SoftDelete(ctx context.Context, id int64) error {
	return s.q.SoftDeleteFinanceTransaction(ctx, id)
}

// Restore clears deleted_at on a trashed transaction.
func (s *Service) Restore(ctx context.Context, id int64) error {
	return s.q.RestoreFinanceTransaction(ctx, id)
}

// Purge permanently deletes a transaction and (via FK cascades) its tag links.
// Orphan tags are pruned inside the same transaction.
func (s *Service) Purge(ctx context.Context, id int64) error {
	return sqliteutil.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.PurgeFinanceTransaction(ctx, id); err != nil {
			return fmt.Errorf("purge transaction: %w", err)
		}
		att := NewAttacher(q)
		if err := domain.WithTags(ctx, s.db, tx, att, domain.AttachInput{
			OwnerKind:    "finance",
			OwnerID:      id,
			Tags:         nil,
			PruneOrphans: true,
		}); err != nil {
			return err
		}
		return nil
	})
}

// Get returns a transaction and its tags. Returns sql.ErrNoRows if not found.
func (s *Service) Get(ctx context.Context, id int64) (*Transaction, error) {
	row, err := s.q.GetFinanceTransaction(ctx, id)
	if err != nil {
		return nil, err
	}
	tags, err := s.tagsFor(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Transaction{Row: row, Tags: tags}, nil
}

// List returns transactions matching the filter. When Search is non-empty, it
// delegates to FTS5 via SearchFinance.
func (s *Service) List(ctx context.Context, f Filter) ([]*Transaction, error) {
	if f.Search != "" {
		limit := int64(f.Limit)
		if limit <= 0 {
			limit = 50
		}
		rows, err := s.q.SearchFinance(ctx, f.Search, limit)
		if err != nil {
			return nil, err
		}
		out := make([]*Transaction, len(rows))
		for i, lf := range rows {
			out[i] = &Transaction{Row: lf.Finance, Tags: lf.Tags}
		}
		return out, nil
	}

	rows, err := s.q.ListFinanceFiltered(ctx, store.FinanceListFilter{
		From:         f.From,
		To:           f.To,
		Kind:         f.Kind,
		CategorySlug: f.CategorySlug,
		TagName:      f.TagName,
		Trashed:      f.Trashed,
		Limit:        f.Limit,
		Offset:       f.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*Transaction, len(rows))
	for i, lf := range rows {
		out[i] = &Transaction{Row: lf.Finance, Tags: lf.Tags}
	}
	return out, nil
}

// Export returns rows matching the filter, with category names resolved.
func (s *Service) Export(ctx context.Context, f Filter) ([]ExportRow, error) {
	rows, err := s.q.ListFinanceFiltered(ctx, store.FinanceListFilter{
		From:         f.From,
		To:           f.To,
		Kind:         f.Kind,
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
	for i, lf := range rows {
		category := ""
		if lf.CategoryName.Valid {
			category = lf.CategoryName.String
		}
		notes := ""
		if lf.Finance.Notes.Valid {
			notes = lf.Finance.Notes.String
		}
		out[i] = ExportRow{
			Date:     lf.Finance.Date,
			Kind:     lf.Finance.Kind,
			Amount:   lf.Finance.Amount,
			Currency: lf.Finance.Currency,
			Account:  lf.Finance.Account,
			Category: category,
			Notes:    notes,
			Tags:     lf.Tags,
		}
	}
	return out, nil
}

// tagsFor returns the tag names attached to a transaction, sorted by name.
func (s *Service) tagsFor(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.q.ListTagsForFinance(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	out := make([]string, len(rows))
	for i, t := range rows {
		out[i] = t.Name
	}
	return out, nil
}

// currency returns the configured currency, defaulting to USD.
func (s *Service) currency(ctx context.Context) (string, error) {
	cs := configstore.New(s.db)
	v, err := cs.GetDefault(ctx, config.KeyCurrency)
	if err != nil {
		return "", fmt.Errorf("read currency: %w", err)
	}
	return v, nil
}

func nullableString(s string) sql.NullString {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullableInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}
