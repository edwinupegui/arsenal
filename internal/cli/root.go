// Package cli wires up the cobra command tree for the arsenal binary.
package cli

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/config"
	"github.com/edwinupegui/arsenal/internal/store"
)

// AppContext holds long-lived process resources accessible to subcommands.
type AppContext struct {
	Paths   config.Paths
	DB      *sql.DB
	Queries *store.Queries
}

// migrationsFS is set by Execute() so the store package stays free of binary-only deps.
var migrationsFS embed.FS

// Execute is the entrypoint called from main with the embedded migrations FS.
// It builds the cobra root, wires shared state, and runs the command.
func Execute(fsys embed.FS) error {
	migrationsFS = fsys

	root := &cobra.Command{
		Use:           "arsenal",
		Short:         "Local-first manager for your curated technical resources",
		Long:          "Arsenal — manage videos, articles, repos, tools and more in a single local SQLite database, via TUI, command line, or a localhost web UI.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date),
		// No subcommand → open the TUI. This matches the project's "default
		// to interactive" UX while still allowing every script-friendly
		// subcommand to take over with explicit invocation.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTUI(cmd)
		},
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newMigrateCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newShowCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newRmCmd())
	root.AddCommand(newRestoreCmd())
	root.AddCommand(newPurgeCmd())
	root.AddCommand(newTrashCmd())
	root.AddCommand(newStarCmd())
	root.AddCommand(newUnstarCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newCatCmd())
	root.AddCommand(newTagCmd())
	root.AddCommand(newTUICmd())
	root.AddCommand(newWebCmd())
	root.AddCommand(newBackupCmd())
	root.AddCommand(newExportCmd())
	root.AddCommand(newImportCmd())

	wireCompletions(root)

	return root.ExecuteContext(context.Background())
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the arsenal home directory and database, applying pending migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()
			fmt.Fprintf(cmd.OutOrStdout(),
				"arsenal initialized\n  home: %s\n  db:   %s\n",
				app.Paths.Home, app.Paths.DB)
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(),
				"arsenal %s\n  commit: %s\n  built:  %s\n",
				Version, Commit, Date)
		},
	}
}

// initApp opens the DB and runs pending migrations. Subcommands that need DB
// access call this; pure metadata commands (version, completion) skip it.
func initApp(ctx context.Context) (*AppContext, error) {
	paths, err := config.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}
	db, err := store.Open(paths.DB)
	if err != nil {
		return nil, err
	}
	if err := store.Migrate(db, migrationsFS, "."); err != nil {
		_ = db.Close()
		return nil, err
	}
	_ = ctx // reserved for future cancellation propagation
	return &AppContext{
		Paths:   paths,
		DB:      db,
		Queries: store.New(db),
	}, nil
}


