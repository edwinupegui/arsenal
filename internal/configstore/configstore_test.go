package configstore_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/sqliteutil"
)

func newTestDB(t *testing.T) *configstore.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "arsenal.db")
	db, err := sqliteutil.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqliteutil.Migrate(db, migrations.FS, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return configstore.New(db)
}

func TestGet_NotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	_, err := s.Get(ctx, config.KeyLandingSurface)
	if !errors.Is(err, configstore.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGet_UnknownKey(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	_, err := s.Get(ctx, config.Key("totally_made_up"))
	if !errors.Is(err, configstore.ErrUnknownKey) {
		t.Errorf("expected ErrUnknownKey, got %v", err)
	}
}

func TestSetGet_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	if err := s.Set(ctx, config.KeyCurrency, "USD"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(ctx, config.KeyCurrency)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "USD" {
		t.Errorf("Get = %q, want USD", got)
	}
}

func TestSet_Overwrites(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	_ = s.Set(ctx, config.KeyCurrency, "USD")
	if err := s.Set(ctx, config.KeyCurrency, "EUR"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, _ := s.Get(ctx, config.KeyCurrency)
	if got != "EUR" {
		t.Errorf("after overwrite Get = %q, want EUR", got)
	}
}

func TestSet_InvalidEnumValue(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	err := s.Set(ctx, config.KeyLandingSurface, "wibble")
	if !errors.Is(err, configstore.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	// The bad value must NOT be persisted.
	_, getErr := s.Get(ctx, config.KeyLandingSurface)
	if !errors.Is(getErr, configstore.ErrNotFound) {
		t.Errorf("expected ErrNotFound after rejected Set, got %v", getErr)
	}
}

func TestSet_UnknownKey(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	err := s.Set(ctx, config.Key("not_a_real_key"), "x")
	if !errors.Is(err, configstore.ErrUnknownKey) {
		t.Errorf("expected ErrUnknownKey, got %v", err)
	}
}

func TestSet_ListValidatorAcceptsValid(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	if err := s.Set(ctx, config.KeyActiveDomains, "resources,todos,today"); err != nil {
		t.Errorf("Set with valid list: %v", err)
	}
}

func TestSet_ListValidatorRejectsUnknownDomain(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	err := s.Set(ctx, config.KeyActiveDomains, "resources,mars")
	if !errors.Is(err, configstore.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestSet_ListValidatorRejectsDuplicate(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	err := s.Set(ctx, config.KeyActiveDomains, "resources,todos,resources")
	if !errors.Is(err, configstore.ErrValidation) {
		t.Errorf("expected ErrValidation for duplicate, got %v", err)
	}
}

func TestGetDefault(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	// Unset: should return the catalog default.
	got, err := s.GetDefault(ctx, config.KeyLandingSurface)
	if err != nil {
		t.Fatalf("GetDefault unset: %v", err)
	}
	if got != "today" {
		t.Errorf("GetDefault unset = %q, want today", got)
	}
	// Set: should return the stored value.
	if err := s.Set(ctx, config.KeyLandingSurface, "resources"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err = s.GetDefault(ctx, config.KeyLandingSurface)
	if err != nil {
		t.Fatalf("GetDefault set: %v", err)
	}
	if got != "resources" {
		t.Errorf("GetDefault set = %q, want resources", got)
	}
}

func TestGetDefault_UnknownKey(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	_, err := s.GetDefault(ctx, config.Key("nope"))
	if !errors.Is(err, configstore.ErrUnknownKey) {
		t.Errorf("expected ErrUnknownKey, got %v", err)
	}
}

func TestUnset(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	if err := s.Set(ctx, config.KeyCurrency, "USD"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Unset(ctx, config.KeyCurrency); err != nil {
		t.Fatalf("Unset: %v", err)
	}
	_, err := s.Get(ctx, config.KeyCurrency)
	if !errors.Is(err, configstore.ErrNotFound) {
		t.Errorf("after Unset, expected ErrNotFound, got %v", err)
	}
	// Unset on already-missing key is a no-op, not an error.
	if err := s.Unset(ctx, config.KeyCurrency); err != nil {
		t.Errorf("Unset on missing key: %v", err)
	}
}

func TestUnset_UnknownKey(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	err := s.Unset(ctx, config.Key("not_a_key"))
	if !errors.Is(err, configstore.ErrUnknownKey) {
		t.Errorf("expected ErrUnknownKey, got %v", err)
	}
}

func TestAll(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)
	_ = s.Set(ctx, config.KeyCurrency, "USD")
	_ = s.Set(ctx, config.KeyLandingSurface, "resources")
	all, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if all[config.KeyCurrency] != "USD" {
		t.Errorf("all[KeyCurrency] = %q, want USD", all[config.KeyCurrency])
	}
	if all[config.KeyLandingSurface] != "resources" {
		t.Errorf("all[KeyLandingSurface] = %q, want resources", all[config.KeyLandingSurface])
	}
}
