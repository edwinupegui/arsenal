package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/todos"
)

// newTodoShowCmd builds `arsenal todo show <id>`.
func newTodoShowCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show full detail for a single todo",
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

			res, err := todos.New(app.DB).Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("todo %d not found", id)
			}
			if err != nil {
				return err
			}

			if flagJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "ID:          %d\n", res.Row.ID)
			fmt.Fprintf(out, "Title:       %s\n", res.Row.Title)
			fmt.Fprintf(out, "Status:      %s\n", res.Row.Status)
			fmt.Fprintf(out, "Priority:    %s\n", res.Row.Priority)
			if res.Row.DueDate != nil {
				fmt.Fprintf(out, "Due:         %s\n", *res.Row.DueDate)
			}
			if res.Row.CategoryID != nil {
				cat, err := app.Queries.GetCategory(cmd.Context(), *res.Row.CategoryID)
				if err == nil {
					fmt.Fprintf(out, "Category:    %s (%s)\n", cat.Name, cat.Slug)
				}
			}
			if len(res.Tags) > 0 {
				fmt.Fprintf(out, "Tags:        %s\n", fmt.Sprintf("%s", res.Tags))
			}
			fmt.Fprintf(out, "Recurrence:  %s\n", res.Row.Recurrence)
			fmt.Fprintf(out, "Created:     %s\n", res.Row.CreatedAt)
			fmt.Fprintf(out, "Updated:     %s\n", res.Row.UpdatedAt)
			if res.Row.DoneAt != nil {
				fmt.Fprintf(out, "Done at:     %s\n", *res.Row.DoneAt)
			}
			if res.Row.DeletedAt != nil {
				fmt.Fprintf(out, "Trashed at:  %s\n", *res.Row.DeletedAt)
			}
			if res.Row.Description != nil && *res.Row.Description != "" {
				fmt.Fprintln(out)
				fmt.Fprintln(out, "Description:")
				fmt.Fprintln(out, indent(*res.Row.Description, "  "))
			}
			if res.Row.Notes != nil && *res.Row.Notes != "" {
				fmt.Fprintln(out)
				fmt.Fprintln(out, "Notes:")
				fmt.Fprintln(out, indent(*res.Row.Notes, "  "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}
