// Package backup snapshots the live SQLite database to a standalone file.
// Uses SQLite's VACUUM INTO statement so the snapshot is a fully independent
// database (compacted, no WAL leftovers) and stays consistent under concurrent
// writes from another process.
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Snapshot writes a copy of the open db to dest using `VACUUM INTO`.
// dest must NOT exist; SQLite will refuse to overwrite. The parent
// directory is created if needed.
func Snapshot(ctx context.Context, db *sql.DB, dest string) error {
	if dest == "" {
		return fmt.Errorf("backup: dest is required")
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("backup: refusing to overwrite existing file %s", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("backup: mkdir parent: %w", err)
	}
	abs, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("backup: resolve dest: %w", err)
	}
	// VACUUM INTO needs a string literal. Path is from our own filesystem
	// resolution, not user-supplied SQL — quoting is enough.
	stmt := fmt.Sprintf("VACUUM INTO '%s'", escapeSQLLiteral(abs))
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("backup: vacuum into: %w", err)
	}
	return nil
}

// DefaultPath returns the conventional snapshot location under
// dirRoot/arsenal-YYYYMMDD-HHMMSS.db.
func DefaultPath(dirRoot string, t time.Time) string {
	return filepath.Join(dirRoot, fmt.Sprintf("arsenal-%s.db", t.UTC().Format("20060102-150405")))
}

// escapeSQLLiteral doubles any single quote in the path so VACUUM INTO can
// accept paths containing apostrophes. SQLite literals don't need escapes
// for any other character.
func escapeSQLLiteral(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' {
			out = append(out, '\'', '\'')
			continue
		}
		out = append(out, c)
	}
	return string(out)
}
