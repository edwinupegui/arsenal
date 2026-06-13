package configstore_test

import (
	"context"
	"testing"

	"github.com/edwinupegui/arsenal/internal/config"
)

func TestGetDefault_UserTimezone_Unset(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	got, err := s.GetDefault(ctx, config.KeyUserTimezone)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if got != "UTC" {
		t.Errorf("GetDefault = %q, want UTC", got)
	}
}

func TestGetDefault_UserTimezone_Set(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	if err := s.Set(ctx, config.KeyUserTimezone, "America/Argentina/Buenos_Aires"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.GetDefault(ctx, config.KeyUserTimezone)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if got != "America/Argentina/Buenos_Aires" {
		t.Errorf("GetDefault = %q, want America/Argentina/Buenos_Aires", got)
	}
}

func TestSet_UserTimezone_InvalidIANA_Stored(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	// Invalid IANA names are NOT rejected at write time; fallback is handled
	// by the UserLocation helper at read time.
	if err := s.Set(ctx, config.KeyUserTimezone, "Mars/Colony"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.GetDefault(ctx, config.KeyUserTimezone)
	if err != nil {
		t.Fatalf("GetDefault: %v", err)
	}
	if got != "Mars/Colony" {
		t.Errorf("GetDefault = %q, want Mars/Colony", got)
	}
}
