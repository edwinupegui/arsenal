package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// newSearchCmd builds `arsenal search <query>` over the FTS5 index.
func newSearchCmd() *cobra.Command {
	var (
		flagLimit int
		flagJSON  bool
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search over title, description, notes and tags",
		Long: `Search uses SQLite's FTS5 index built over title, description, notes and
the joined tag list. Each whitespace-separated term becomes a prefix match,
joined by implicit AND. Diacritics are folded automatically.`,
		Example: `  arsenal search arquitectura
  arsenal search "react patterns" -n 5
  arsenal search avanzado --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			items, err := app.Queries.SearchResources(cmd.Context(), query, int64(flagLimit))
			if err != nil {
				return err
			}
			if flagJSON {
				return writeJSON(cmd.OutOrStdout(), items)
			}
			writeTable(cmd.OutOrStdout(), items)
			return nil
		},
	}
	cmd.Flags().IntVarP(&flagLimit, "limit", "n", 25, "max rows to return")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON instead of a table")
	return cmd
}
