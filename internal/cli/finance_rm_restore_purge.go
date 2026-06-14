package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/finance"
)

// newFinanceRmCmd builds `arsenal finance rm <id>` (soft-delete).
func newFinanceRmCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Move a transaction to the trash (soft delete)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseFinanceID(args[0])
			if err != nil {
				return err
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := finance.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("transaction %d not found", id)
			}
			if err != nil {
				return err
			}

			already := res.Row.DeletedAt.Valid
			if err := svc.SoftDelete(cmd.Context(), id); err != nil {
				return err
			}

			if flagJSON {
				fmt.Fprintf(cmd.OutOrStdout(), `{"id":%d,"account":%q,"trashed":true}`+"\n", id, res.Row.Account)
				return nil
			}

			verb := "moved to trash"
			if already {
				verb = "already in trash"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d: %s (%s)\n", verb, id, res.Row.Account, res.Row.Kind)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

// newFinanceRestoreCmd builds `arsenal finance restore <id>`.
func newFinanceRestoreCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a trashed transaction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseFinanceID(args[0])
			if err != nil {
				return err
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := finance.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("transaction %d not found", id)
			}
			if err != nil {
				return err
			}

			already := !res.Row.DeletedAt.Valid
			if err := svc.Restore(cmd.Context(), id); err != nil {
				return err
			}

			if flagJSON {
				fmt.Fprintf(cmd.OutOrStdout(), `{"id":%d,"account":%q,"restored":true}`+"\n", id, res.Row.Account)
				return nil
			}

			verb := "restored"
			if already {
				verb = "already active"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d: %s (%s)\n", verb, id, res.Row.Account, res.Row.Kind)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

// newFinancePurgeCmd builds `arsenal finance purge <id>`.
// Requires --yes in non-interactive mode; prompts when stdin is a TTY.
func newFinancePurgeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "purge <id>",
		Short: "Permanently delete a transaction (cannot be undone)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseFinanceID(args[0])
			if err != nil {
				return err
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := finance.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("transaction %d not found", id)
			}
			if err != nil {
				return err
			}

			if !yes {
				if !isStdinTTY() {
					return fmt.Errorf("--yes required in non-interactive mode")
				}
				ok, err := confirm(cmd.OutOrStdout(), os.Stdin,
					fmt.Sprintf("Permanently delete %d %q? [y/N] ", id, res.Row.Account))
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
			fmt.Fprintf(cmd.OutOrStdout(), "purged %d: %s (%s)\n", id, res.Row.Account, res.Row.Kind)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
