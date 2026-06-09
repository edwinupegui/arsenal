package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/todos"
)

// newTodoRmCmd builds `arsenal todo rm <id>` (soft-delete).
func newTodoRmCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Move a todo to the trash (soft delete)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("id must be an integer: %w", err)
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := todos.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("todo %d not found", id)
			}
			if err != nil {
				return err
			}

			already := res.Row.DeletedAt != nil
			if err := svc.SoftDelete(cmd.Context(), id); err != nil {
				return err
			}

			if flagJSON {
				fmt.Fprintf(cmd.OutOrStdout(), `{"id":%d,"title":%q,"trashed":true}`+"\n", id, res.Row.Title)
				return nil
			}

			verb := "moved to trash"
			if already {
				verb = "already in trash"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d: %s\n", verb, id, res.Row.Title)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

// newTodoRestoreCmd builds `arsenal todo restore <id>`.
func newTodoRestoreCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a trashed todo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("id must be an integer: %w", err)
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := todos.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("todo %d not found", id)
			}
			if err != nil {
				return err
			}

			already := res.Row.DeletedAt == nil
			if err := svc.Restore(cmd.Context(), id); err != nil {
				return err
			}

			if flagJSON {
				fmt.Fprintf(cmd.OutOrStdout(), `{"id":%d,"title":%q,"restored":true}`+"\n", id, res.Row.Title)
				return nil
			}

			verb := "restored"
			if already {
				verb = "already active"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d: %s\n", verb, id, res.Row.Title)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

// newTodoPurgeCmd builds `arsenal todo purge <id>`.
// Requires --yes in non-interactive mode; prompts when stdin is a TTY.
func newTodoPurgeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "purge <id>",
		Short: "Permanently delete a todo (cannot be undone)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("id must be an integer: %w", err)
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := todos.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("todo %d not found", id)
			}
			if err != nil {
				return err
			}

			if !yes {
				if !isStdinTTY() {
					return fmt.Errorf("--yes required in non-interactive mode")
				}
				ok, err := confirm(cmd.OutOrStdout(), os.Stdin,
					fmt.Sprintf("Permanently delete %d %q? [y/N] ", id, res.Row.Title))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return nil
				}
			}

			if err := svc.Purge(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "purged %d: %s\n", id, res.Row.Title)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

// isStdinTTY reports whether os.Stdin is connected to a character device.
func isStdinTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
