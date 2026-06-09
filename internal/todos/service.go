package todos

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/sqliteutil"
	"github.com/edwinupegui/arsenal/internal/store"
)

// Todo is the service-layer view of a stored todo together with its resolved tags.
type Todo struct {
	Row  store.Todo
	Tags []string
}

// Service exposes todo lifecycle operations.
type Service struct {
	db *sql.DB
	q  *store.Queries
}

// New builds a Service bound to db.
func New(db *sql.DB) *Service {
	return &Service{db: db, q: store.New(db)}
}

// Create validates input, opens a transaction, inserts the todo, upserts and
// attaches tags, and commits.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Todo, error) {
	if in.Priority == "" {
		in.Priority = PriorityMed
	}
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

	var out *Todo
	err = sqliteutil.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		row, err := q.CreateTodo(ctx, store.CreateTodoParams{
			Title:       strings.TrimSpace(in.Title),
			Description: nullableStringPtr(in.Description),
			Priority:    string(in.Priority),
			Status:      "open",
			DueDate:     in.DueDate,
			CategoryID:  in.CategoryID,
			Notes:       nullableStringPtr(in.Notes),
			Recurrence:  string(in.Recurrence),
		})
		if err != nil {
			return fmt.Errorf("insert todo: %w", err)
		}
		att := NewAttacher(q)
		if err := domain.WithTags(ctx, s.db, tx, att, domain.AttachInput{
			OwnerKind:    "todo",
			OwnerID:      row.ID,
			Tags:         tags,
			PruneOrphans: false,
		}); err != nil {
			return err
		}
		out = &Todo{Row: row, Tags: tags}
		return nil
	})
	return out, err
}

// Update replaces the mutable fields of todo id. Tag updates are handled as
// detach-all-then-reattach with orphan pruning.
func (s *Service) Update(ctx context.Context, id int64, in CreateInput) (*Todo, error) {
	if err := validateCreate(in); err != nil {
		return nil, err
	}
	tags, err := domain.NormalizeTags(in.Tags)
	if err != nil {
		return nil, err
	}

	var out *Todo
	err = sqliteutil.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		// Preserve current status — Update is not allowed to change status.
		current, err := q.GetTodo(ctx, id)
		if err != nil {
			return fmt.Errorf("get todo: %w", err)
		}
		row, err := q.UpdateTodo(ctx, store.UpdateTodoParams{
			Title:       strings.TrimSpace(in.Title),
			Description: nullableStringPtr(in.Description),
			Priority:    string(in.Priority),
			Status:      current.Status,
			DueDate:     in.DueDate,
			CategoryID:  in.CategoryID,
			Notes:       nullableStringPtr(in.Notes),
			Recurrence:  string(in.Recurrence),
			ID:          id,
		})
		if err != nil {
			return fmt.Errorf("update todo: %w", err)
		}
		if err := q.DetachAllTagsFromTodo(ctx, id); err != nil {
			return fmt.Errorf("detach tags: %w", err)
		}
		att := NewAttacher(q)
		if err := domain.WithTags(ctx, s.db, tx, att, domain.AttachInput{
			OwnerKind:    "todo",
			OwnerID:      id,
			Tags:         tags,
			PruneOrphans: true,
		}); err != nil {
			return err
		}
		out = &Todo{Row: row, Tags: tags}
		return nil
	})
	return out, err
}

// SoftDelete moves a todo to the trash by setting deleted_at.
func (s *Service) SoftDelete(ctx context.Context, id int64) error {
	return s.q.SoftDeleteTodo(ctx, id)
}

// Restore clears deleted_at on a trashed todo.
func (s *Service) Restore(ctx context.Context, id int64) error {
	return s.q.RestoreTodo(ctx, id)
}

// Purge permanently deletes a todo and (via FK cascades) its tag links.
// Orphan tags are pruned inside the same transaction.
func (s *Service) Purge(ctx context.Context, id int64) error {
	return sqliteutil.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.PurgeTodo(ctx, id); err != nil {
			return fmt.Errorf("purge todo: %w", err)
		}
		att := NewAttacher(q)
		if err := domain.WithTags(ctx, s.db, tx, att, domain.AttachInput{
			OwnerKind:    "todo",
			OwnerID:      id,
			Tags:         nil,
			PruneOrphans: true,
		}); err != nil {
			return err
		}
		return nil
	})
}

// Get returns a todo and its tags. Returns sql.ErrNoRows if not found.
func (s *Service) Get(ctx context.Context, id int64) (*Todo, error) {
	row, err := s.q.GetTodo(ctx, id)
	if err != nil {
		return nil, err
	}
	tags, err := s.tagsFor(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Todo{Row: row, Tags: tags}, nil
}

// tagsFor returns the tag names attached to a todo, sorted by name.
func (s *Service) tagsFor(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.name FROM tags t
		JOIN todo_tags tt ON tt.tag_id = t.id
		WHERE tt.todo_id = ?
		ORDER BY t.name ASC
	`, id)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	return out, nil
}

func validateCreate(in CreateInput) error {
	if err := domain.ValidateTitle(in.Title); err != nil {
		return err
	}
	if in.Priority != "" && !in.Priority.Valid() {
		return fmt.Errorf("invalid priority %q", in.Priority)
	}
	if in.Recurrence != "" && !in.Recurrence.Valid() {
		return fmt.Errorf("invalid recurrence %q", in.Recurrence)
	}
	return nil
}

func nullableStringPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
