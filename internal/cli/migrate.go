package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/migrate/legacy"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
)

// newMigrateCmd builds `arsenal migrate --from <legacy.db>`. With --dry-run
// the import targets a temporary destination DB so the user's real
// `~/.arsenal/arsenal.db` is never touched.
func newMigrateCmd() *cobra.Command {
	var (
		fromPath string
		dryRun   bool
	)
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Import resources and categories from an Arsenal v1 SQLite database",
		Long: `Migrate copies categories and resources from a legacy arsenal-app database
into the current Arsenal database, preserving timestamps and tag relationships.

The legacy file is opened read-only and never modified.

With --dry-run the import targets a temporary database and the report shows
what would have been migrated, leaving your real database untouched.`,
		Example: `  arsenal migrate --from ../arsenal-legacy/resources.db
  arsenal migrate --from ./old.db --dry-run`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(fromPath) == "" {
				return fmt.Errorf("--from is required")
			}
			if _, err := os.Stat(fromPath); err != nil {
				return fmt.Errorf("source %s: %w", fromPath, err)
			}

			ctx := cmd.Context()
			dest, cleanup, err := openDestination(ctx, dryRun)
			if err != nil {
				return err
			}
			defer cleanup()

			svc := resources.New(dest.DB)
			report, err := legacy.Import(ctx, legacy.Options{
				SourcePath: fromPath,
				DryRun:     dryRun,
				Service:    svc,
				Queries:    dest.Queries,
			})
			if err != nil {
				return err
			}

			printReport(cmd, report, dest.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&fromPath, "from", "", "path to the legacy resources.db (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "import into a throwaway DB and discard the result")
	_ = cmd.MarkFlagRequired("from")
	return cmd
}

// destination bundles the destination DB and its sqlc Queries together with
// the path so the report can show where data landed.
type destination struct {
	DB      *sql.DB
	Queries *store.Queries
	Path    string
}

// openDestination returns either the user's real DB (~/.arsenal/arsenal.db)
// or a temporary one for --dry-run. cleanup closes the connection and, in
// dry-run mode, removes the temp file.
func openDestination(ctx context.Context, dryRun bool) (destination, func(), error) {
	if !dryRun {
		app, err := initApp(ctx)
		if err != nil {
			return destination{}, nil, err
		}
		return destination{
				DB:      app.DB,
				Queries: app.Queries,
				Path:    app.Paths.DB,
			}, func() {
				_ = app.DB.Close()
			}, nil
	}

	tmpDir, err := os.MkdirTemp("", "arsenal-dryrun-*")
	if err != nil {
		return destination{}, nil, fmt.Errorf("mkdir temp: %w", err)
	}
	tmpPath := filepath.Join(tmpDir, "arsenal.db")

	db, err := store.Open(tmpPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return destination{}, nil, err
	}
	if err := store.Migrate(db, migrationsFS, "."); err != nil {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
		return destination{}, nil, err
	}
	return destination{
			DB:      db,
			Queries: store.New(db),
			Path:    tmpPath,
		}, func() {
			_ = db.Close()
			_ = os.RemoveAll(tmpDir)
		}, nil
}

func printReport(cmd *cobra.Command, r legacy.Report, destPath string) {
	out := cmd.OutOrStdout()
	mode := "applied"
	if r.DryRun {
		mode = "dry-run (changes discarded)"
	}
	fmt.Fprintf(out, "Migration %s\n", mode)
	fmt.Fprintf(out, "  destination: %s\n", destPath)
	fmt.Fprintf(out, "  categories inserted: %d\n", r.CategoriesInserted)
	if r.CategoriesAlreadyKept > 0 {
		fmt.Fprintf(out, "  categories already present: %d\n", r.CategoriesAlreadyKept)
	}
	fmt.Fprintf(out, "  resources imported: %d\n", r.ResourcesImported)
	if r.ResourcesSkippedDup > 0 {
		fmt.Fprintf(out, "  resources skipped (duplicate URL): %d\n", r.ResourcesSkippedDup)
	}
	fmt.Fprintf(out, "  distinct tags created: %d\n", r.TagsCreated)
	if len(r.Warnings) > 0 {
		fmt.Fprintf(out, "  warnings:\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(out, "    - %s\n", w)
		}
	}
}
