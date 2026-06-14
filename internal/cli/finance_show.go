package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/finance"
)

// newFinanceShowCmd builds `arsenal finance show <id>`.
func newFinanceShowCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show full detail for a single transaction",
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

			res, err := finance.New(app.DB).Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("transaction %d not found", id)
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
			fmt.Fprintf(out, "Date:        %s\n", res.Row.Date)
			fmt.Fprintf(out, "Kind:        %s\n", res.Row.Kind)
			fmt.Fprintf(out, "Amount:      %s\n", finance.FormatAmount(res.Row.Amount, res.Row.Currency))
			fmt.Fprintf(out, "Account:     %s\n", res.Row.Account)
			if res.Row.CategoryID.Valid {
				cat, err := app.Queries.GetCategory(cmd.Context(), res.Row.CategoryID.Int64)
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
			if res.Row.Notes.Valid && res.Row.Notes.String != "" {
				fmt.Fprintln(out)
				fmt.Fprintln(out, "Notes:")
				fmt.Fprintln(out, indent(res.Row.Notes.String, "  "))
			}
			if res.Row.DeletedAt.Valid {
				fmt.Fprintf(out, "Trashed at:  %s\n", res.Row.DeletedAt.String)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}
