package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/calendar"
)

// newCalendarRmCmd builds `arsenal calendar rm <id>` (soft-delete).
func newCalendarRmCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Move a calendar event to the trash (soft delete)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCalendarID(args[0])
			if err != nil {
				return err
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := calendar.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("event %d not found", id)
			}
			if err != nil {
				return err
			}

			already := res.Row.DeletedAt.Valid
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

// newCalendarRestoreCmd builds `arsenal calendar restore <id>`.
func newCalendarRestoreCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a trashed calendar event",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCalendarID(args[0])
			if err != nil {
				return err
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := calendar.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("event %d not found", id)
			}
			if err != nil {
				return err
			}

			already := !res.Row.DeletedAt.Valid
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

// newCalendarPurgeCmd builds `arsenal calendar purge <id>`.
// Requires --yes in non-interactive mode; prompts when stdin is a TTY.
func newCalendarPurgeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "purge <id>",
		Short: "Permanently delete a calendar event (cannot be undone)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCalendarID(args[0])
			if err != nil {
				return err
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := calendar.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("event %d not found", id)
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
