package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/exportmd"
)

// newExportCmd builds `arsenal export` (markdown today; future formats
// can hang off subcommands or extra flags).
func newExportCmd() *cobra.Command {
	var (
		dir            string
		includeTrashed bool
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the database as a directory of markdown files",
		Long: `Export writes one markdown file per resource under <dir>, organized by
category slug (uncategorized rows go to <dir>/uncategorized/). Each file
opens with a YAML-style frontmatter block holding type, language, tags
and timestamps; the body is the description, with notes following an
optional '## Notes' heading.

The format is symmetric with 'arsenal import' — round-tripping through
markdown is lossless except for the row ID, which is reassigned on
import.`,
		Example: `  arsenal export --md ./arsenal-export
  arsenal export --md ./arsenal-export --include-trashed`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(dir) == "" {
				return fmt.Errorf("--md <dir> is required")
			}
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			report, err := exportmd.ExportAll(cmd.Context(), app.DB, app.Queries, exportmd.Options{
				Dir:            dir,
				IncludeTrashed: includeTrashed,
			})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "exported to %s\n", report.OutputDir)
			fmt.Fprintf(out, "  resources: %d\n", report.Resources)
			if includeTrashed {
				fmt.Fprintf(out, "  trashed:   %d\n", report.Trashed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "md", "", "destination directory for the markdown tree (required)")
	cmd.Flags().BoolVar(&includeTrashed, "include-trashed", false, "also export trashed resources to <dir>/_trashed/")
	_ = cmd.MarkFlagRequired("md")
	return cmd
}
