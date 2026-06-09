// Package configstore persists runtime configuration as key-value rows in
// the arsenal_config table. Keys are typed (see internal/config) and values
// are validated against the catalog before being persisted. The store is
// intentionally generic — no domain-specific knowledge lives here.
package configstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/edwinupegui/arsenal/internal/config"
)

// ErrNotFound is returned by Get when a key is not set.
var ErrNotFound = errors.New("config: key not found")

// ErrUnknownKey is returned when callers pass a Key that is not in the
// config.Catalog. This is a programmer error, not a user error — surface it
// loudly at the call site.
var ErrUnknownKey = errors.New("config: unknown key")

// ErrValidation is returned by Set when the value fails the catalog
// validator (enum mismatch, list with bad item, etc.). The caller should
// treat this as a user error and print a helpful message.
var ErrValidation = errors.New("config: invalid value")

// Store wraps a *sql.DB and exposes typed get/set helpers for the
// arsenal_config table. Safe for concurrent use.
type Store struct {
	db *sql.DB
}

// New returns a Store bound to db. The caller owns the DB lifecycle.
func New(db *sql.DB) *Store { return &Store{db: db} }

// Get returns the value for k, or ErrNotFound if the key is unset.
// ErrUnknownKey is returned if k is not in the config catalog.
func (s *Store) Get(ctx context.Context, k config.Key) (string, error) {
	if _, ok := config.GetMeta(k); !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownKey, k)
	}
	var v string
	err := s.db.QueryRowContext(ctx,
		`SELECT v FROM arsenal_config WHERE k = ?`, string(k)).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("configstore.Get %q: %w", k, err)
	}
	return v, nil
}

// GetDefault returns the value for k, the catalog default if the key is
// unset, or an error if k is unknown.
func (s *Store) GetDefault(ctx context.Context, k config.Key) (string, error) {
	meta, ok := config.GetMeta(k)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownKey, k)
	}
	v, err := s.Get(ctx, k)
	if errors.Is(err, ErrNotFound) {
		return meta.Default, nil
	}
	return v, err
}

// Set writes the value for k, upserting if the key already exists. The value
// is validated against the catalog (enum membership, custom Validate) before
// being persisted. Returns ErrUnknownKey for unknown keys or
// ErrValidation (wrapped with detail) for bad values.
func (s *Store) Set(ctx context.Context, k config.Key, v string) error {
	meta, ok := config.GetMeta(k)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownKey, k)
	}
	if err := validateValue(meta, v); err != nil {
		return fmt.Errorf("%w: %s", ErrValidation, err)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO arsenal_config (k, v) VALUES (?, ?)
		ON CONFLICT(k) DO UPDATE SET
			v = excluded.v,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
	`, string(k), v)
	if err != nil {
		return fmt.Errorf("configstore.Set %q: %w", k, err)
	}
	return nil
}

// Unset removes the key from the store. A no-op if the key was not set.
// Returns ErrUnknownKey if k is not in the catalog.
func (s *Store) Unset(ctx context.Context, k config.Key) error {
	if _, ok := config.GetMeta(k); !ok {
		return fmt.Errorf("%w: %q", ErrUnknownKey, k)
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM arsenal_config WHERE k = ?`, string(k))
	if err != nil {
		return fmt.Errorf("configstore.Unset %q: %w", k, err)
	}
	return nil
}

// All returns every (key, value) pair currently stored. Used by
// `arsenal config list` and for debugging. Keys are config.Key, sorted by
// the underlying string in config.All().
func (s *Store) All(ctx context.Context) (map[config.Key]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT k, v FROM arsenal_config`)
	if err != nil {
		return nil, fmt.Errorf("configstore.All: %w", err)
	}
	defer rows.Close()
	out := make(map[config.Key]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("configstore.All scan: %w", err)
		}
		out[config.Key(k)] = v
	}
	return out, rows.Err()
}

// validateValue applies the catalog's type-based and custom validation to v.
func validateValue(meta config.KeyMeta, v string) error {
	switch meta.Type {
	case config.TypeEnum:
		for _, allowed := range meta.EnumValues {
			if v == allowed {
				if meta.Validate != nil {
					return meta.Validate(v)
				}
				return nil
			}
		}
		return fmt.Errorf("value %q is not one of [%s]",
			v, strings.Join(meta.EnumValues, ", "))
	case config.TypeBool:
		lower := strings.ToLower(v)
		if lower != "true" && lower != "false" && lower != "1" && lower != "0" {
			return fmt.Errorf("value %q is not a valid boolean (expected true, false, 1, or 0)", v)
		}
		if meta.Validate != nil {
			return meta.Validate(v)
		}
		return nil
	case config.TypeList, config.TypeString:
		// For TypeList, the Validate func handles the comma-separated parse.
		// For TypeString, the Validate func (if any) is the only check.
		if meta.Validate != nil {
			return meta.Validate(v)
		}
	}
	return nil
}
