package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/exportmd"
)

// newImportCmd builds `arsenal import <dir>`. Walks the directory for *.md
// files and routes each through resources.Service.Import. Duplicate URLs
// are skipped so the command is safe to run twice.
func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <dir>",
		Short: "Import markdown files (as produced by `arsenal export --md`) into the database",
		Long: `Import scans the given directory recursively for .md files, parses each
file's frontmatter and inserts the resource through the same code path
used by 'arsenal add' and 'arsenal migrate'. Created/updated/deleted
timestamps are preserved when present in the frontmatter; a missing
created_at falls back to "now".

The matching key is the URL: any file whose URL already exists in the
database is skipped. The command is therefore idempotent — re-running
it never creates duplicates.`,
		Example: `  arsenal import ./arsenal-export
  arsenal import ./single-folder`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			info, err := os.Stat(dir)
			if err != nil {
				return fmt.Errorf("import: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("import: %s is not a directory", dir)
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := resourcesService(app)
			report, err := exportmd.Import(cmd.Context(), dir, svc, app.Queries)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "imported from %s\n", dir)
			fmt.Fprintf(out, "  files scanned: %d\n", report.FilesScanned)
			fmt.Fprintf(out, "  imported:      %d\n", report.Imported)
			if report.SkippedDup > 0 {
				fmt.Fprintf(out, "  skipped (dup URL): %d\n", report.SkippedDup)
			}
			if report.Failed > 0 {
				fmt.Fprintf(out, "  failed:        %d\n", report.Failed)
			}
			for _, w := range report.Warnings {
				fmt.Fprintf(out, "  warn: %s\n", w)
			}
			return nil
		},
	}
	return cmd
}
