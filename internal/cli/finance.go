package cli

import (
	"github.com/spf13/cobra"
)

// newFinanceCmd returns the `arsenal finance` parent command. Subcommands
// cover the full transaction lifecycle plus CSV export. The parent has no
// RunE — invoking it without a subcommand prints help.
func newFinanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finance",
		Short: "Manage finance transactions",
		Long:  "Create, list, update, and delete income and expense transactions.",
	}
	cmd.AddCommand(newFinanceAddCmd())
	cmd.AddCommand(newFinanceListCmd())
	cmd.AddCommand(newFinanceShowCmd())
	cmd.AddCommand(newFinanceEditCmd())
	cmd.AddCommand(newFinanceRmCmd())
	cmd.AddCommand(newFinanceRestoreCmd())
	cmd.AddCommand(newFinancePurgeCmd())
	cmd.AddCommand(newFinanceExportCmd())
	return cmd
}
