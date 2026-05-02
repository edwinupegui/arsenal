package cli

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/config"
	_ "modernc.org/sqlite"
)

// newRestoreBackupCmd builds `arsenal restore-backup <file>`. Restoring is
// "stop the world, swap files, move on": rename the live DB out of the way,
// copy the backup into place, and trust normal connection logic to find it.
//
// This intentionally does not use VACUUM INTO in reverse — backups are
// already complete SQLite files thanks to VACUUM INTO, so a plain copy is
// the right primitive.
func newRestoreBackupCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "restore-backup <file>",
		Short: "Replace the live database with a backup file",
		Long: `Restore-backup swaps the active arsenal.db with the snapshot at <file>.
The currently-live database is preserved with a timestamped suffix
(arsenal.db.pre-restore-YYYYMMDD-HHMMSS) inside the same directory in
case you change your mind.

Stop any running 'arsenal web' or 'arsenal' (TUI) before restoring;
SQLite will refuse to swap a file that another process holds open.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			info, err := os.Stat(src)
			if err != nil {
				return fmt.Errorf("restore-backup: %w", err)
			}
			if info.IsDir() {
				return fmt.Errorf("restore-backup: %s is a directory", src)
			}
			if err := assertSQLiteFile(src); err != nil {
				return err
			}

			paths, err := config.Resolve()
			if err != nil {
				return err
			}

			if !yes {
				ok, err := confirm(cmd.OutOrStdout(), os.Stdin,
					fmt.Sprintf("Replace %s with %s? Current DB will be saved with .pre-restore-* suffix. [y/N] ",
						paths.DB, src))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return nil
				}
			}

			if _, err := os.Stat(paths.DB); err == nil {
				ts := time.Now().UTC().Format("20060102-150405")
				bak := paths.DB + ".pre-restore-" + ts
				if err := os.Rename(paths.DB, bak); err != nil {
					return fmt.Errorf("rename live db: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "moved current db to %s\n", bak)
				// Also move WAL and SHM siblings so the restored file isn't
				// shadowed by stale auxiliary state.
				for _, ext := range []string{"-wal", "-shm"} {
					_ = os.Rename(paths.DB+ext, bak+ext)
				}
			}

			if err := copyFile(src, paths.DB); err != nil {
				return fmt.Errorf("copy snapshot: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "restored %s ← %s\n", paths.DB, src)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

// assertSQLiteFile opens path read-only and runs a trivial query to make
// sure the file is actually a SQLite database, not an HTML page or a
// half-downloaded archive.
func assertSQLiteFile(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", abs+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer db.Close()
	if _, err := db.Exec("SELECT 1"); err != nil {
		return fmt.Errorf("%s is not a valid SQLite database: %w", path, err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
