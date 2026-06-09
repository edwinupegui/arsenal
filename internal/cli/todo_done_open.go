package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/todos"
)

// newTodoDoneCmd builds `arsenal todo done <id>`.
func newTodoDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a todo as done",
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

			if err := svc.MarkDone(cmd.Context(), id); err != nil {
				return err
			}

			verb := "marked done"
			if res.Row.Status == "done" {
				verb = "already done"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d: %s\n", verb, id, res.Row.Title)
			return nil
		},
	}
}

// newTodoOpenCmd builds `arsenal todo open <id>`.
func newTodoOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <id>",
		Short: "Reopen a done todo",
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

			if err := svc.MarkOpen(cmd.Context(), id); err != nil {
				return err
			}

			verb := "reopened"
			if res.Row.Status == "open" {
				verb = "already open"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d: %s\n", verb, id, res.Row.Title)
			return nil
		},
	}
}
