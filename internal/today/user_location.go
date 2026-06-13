package today

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/configstore"
)

// UserLocation reads the user_timezone config key and returns a *time.Location.
// If the key is unset, missing, or the value is not a valid IANA name, it
// returns time.UTC and logs a warning (invalid values only).
func UserLocation(ctx context.Context, db *sql.DB) (*time.Location, error) {
	cs := configstore.New(db)
	v, err := cs.GetDefault(ctx, config.KeyUserTimezone)
	if err != nil {
		return time.UTC, fmt.Errorf("read user timezone: %w", err)
	}
	loc, err := time.LoadLocation(v)
	if err != nil {
		log.Printf("invalid user_timezone %q, falling back to UTC", v)
		return time.UTC, nil
	}
	return loc, nil
}
