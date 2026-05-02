package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/backup"
)

// newBackupCmd builds `arsenal backup` — a one-shot SQLite snapshot via
// VACUUM INTO. By default the file lands under ~/.arsenal/backups/.
func newBackupCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Snapshot the database to a standalone .db file",
		Long: `Creates an independent SQLite file using VACUUM INTO. The snapshot is
compacted (no WAL leftovers) and consistent with the live database even
under concurrent writes. Restoring from a snapshot is just copying it
back over ~/.arsenal/arsenal.db while no arsenal process is running.`,
		Example: `  arsenal backup
  arsenal backup --out ./arsenal-prelaunch.db`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			dest := strings.TrimSpace(out)
			if dest == "" {
				dest = backup.DefaultPath(app.Paths.Backups, time.Now())
			}
			if err := backup.Snapshot(cmd.Context(), app.DB, dest); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "snapshot written to %s\n", dest)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "", "destination path (default: ~/.arsenal/backups/arsenal-<ts>.db)")
	return cmd
}
