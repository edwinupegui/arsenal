package cli

import (
	"github.com/spf13/cobra"
)

// newTodoCmd returns the `arsenal todo` parent command. Subcommands (add,
// list, show, done, open, rm, restore, edit, purge) are wired as children.
// The parent has no RunE — invoking it with no subcommand prints the help.
func newTodoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Manage todos",
		Long:  "Create, list, update, and delete todos.",
	}
	cmd.AddCommand(newTodoAddCmd())
	cmd.AddCommand(newTodoListCmd())
	cmd.AddCommand(newTodoShowCmd())
	cmd.AddCommand(newTodoDoneCmd())
	cmd.AddCommand(newTodoOpenCmd())
	cmd.AddCommand(newTodoRmCmd())
	cmd.AddCommand(newTodoRestoreCmd())
	cmd.AddCommand(newTodoEditCmd())
	cmd.AddCommand(newTodoPurgeCmd())
	return cmd
}
