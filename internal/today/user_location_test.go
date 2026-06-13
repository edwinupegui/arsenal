package today

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/sqliteutil"
)

func newTestDB(t *testing.T) *sql.DB {
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
	return db
}

func TestUserLocation_Unset(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	loc, err := UserLocation(ctx, db)
	if err != nil {
		t.Fatalf("UserLocation: %v", err)
	}
	if loc != time.UTC {
		t.Errorf("UserLocation = %v, want UTC", loc)
	}
}

func TestUserLocation_Valid(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	cs := configstore.New(db)

	if err := cs.Set(ctx, config.KeyUserTimezone, "America/Argentina/Buenos_Aires"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	loc, err := UserLocation(ctx, db)
	if err != nil {
		t.Fatalf("UserLocation: %v", err)
	}
	if loc.String() != "America/Argentina/Buenos_Aires" {
		t.Errorf("UserLocation = %v, want America/Argentina/Buenos_Aires", loc)
	}
}

func TestUserLocation_Invalid(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	cs := configstore.New(db)

	if err := cs.Set(ctx, config.KeyUserTimezone, "Mars/Colony"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	loc, err := UserLocation(ctx, db)
	if err != nil {
		t.Fatalf("UserLocation: %v", err)
	}
	if loc != time.UTC {
		t.Errorf("UserLocation = %v, want UTC", loc)
	}
	if buf.Len() == 0 {
		t.Error("expected a log warning for invalid timezone, got none")
	}
}
