// Package config resolves runtime paths for the arsenal CLI following
// XDG Base Directory conventions, with a single ~/.arsenal/ root for simplicity.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds resolved filesystem locations for the arsenal runtime.
type Paths struct {
	Home    string // ~/.arsenal
	DB      string // ~/.arsenal/arsenal.db
	Backups string // ~/.arsenal/backups
	Logs    string // ~/.arsenal/logs
}

// Resolve returns absolute paths derived from $ARSENAL_HOME, falling back to
// ~/.arsenal. The directory tree is created on demand.
func Resolve() (Paths, error) {
	root := os.Getenv("ARSENAL_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("user home dir: %w", err)
		}
		root = filepath.Join(home, ".arsenal")
	}

	p := Paths{
		Home:    root,
		DB:      filepath.Join(root, "arsenal.db"),
		Backups: filepath.Join(root, "backups"),
		Logs:    filepath.Join(root, "logs"),
	}

	for _, dir := range []string{p.Home, p.Backups, p.Logs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Paths{}, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	return p, nil
}
