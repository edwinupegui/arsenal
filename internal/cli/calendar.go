package cli

import (
	"github.com/spf13/cobra"
)

// newCalendarCmd returns the `arsenal calendar` parent command. Subcommands
// cover the full calendar event lifecycle plus iCal export. The parent has no
// RunE — invoking it without a subcommand prints help.
func newCalendarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Manage calendar events",
		Long:  "Create, list, update, and delete calendar events. Export to iCal format.",
	}
	cmd.AddCommand(newCalendarAddCmd())
	cmd.AddCommand(newCalendarListCmd())
	cmd.AddCommand(newCalendarShowCmd())
	cmd.AddCommand(newCalendarEditCmd())
	cmd.AddCommand(newCalendarRmCmd())
	cmd.AddCommand(newCalendarRestoreCmd())
	cmd.AddCommand(newCalendarPurgeCmd())
	cmd.AddCommand(newCalendarExportCmd())
	return cmd
}
