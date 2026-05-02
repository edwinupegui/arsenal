package cli

import (
	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/tui"
)

// newTUICmd builds `arsenal tui`. The same handler is wired as the root
// command's RunE so that bare `arsenal` (no subcommand) opens the TUI.
func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive terminal UI (default when no subcommand is given)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTUI(cmd)
		},
	}
}

// runTUI is the shared entrypoint used by both `arsenal` (root) and
// `arsenal tui`. Opens the DB, applies migrations, and hands control to
// the Bubble Tea program.
func runTUI(cmd *cobra.Command) error {
	app, err := initApp(cmd.Context())
	if err != nil {
		return err
	}
	defer app.DB.Close()
	return tui.Run(app.DB)
}
