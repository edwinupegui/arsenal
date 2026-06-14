package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/finance"
)

// newFinanceEditCmd builds `arsenal finance edit <id>` with optional field flags.
// Missing flags leave fields unchanged.
func newFinanceEditCmd() *cobra.Command {
	var (
		flagDate       string
		flagAmount     float64
		flagKind       string
		flagAccount    string
		flagCat        string
		flagTags       []string
		flagNotes      string
		flagRecurrence string
	)
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an existing transaction",
		Example: `  arsenal finance edit 5 --amount 99 --kind income
  arsenal finance edit 5 --cat food --tag personal`,
		Args: cobra.ExactArgs(1),
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
			current, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("transaction %d not found", id)
			}
			if err != nil {
				return err
			}

			in := finance.CreateInput{
				Date:       current.Row.Date,
				Amount:     current.Row.Amount,
				Kind:       finance.Kind(current.Row.Kind),
				Account:    current.Row.Account,
				CategoryID: nullableInt64Ptr(current.Row.CategoryID),
				Notes:      derefString(nullableStringPtr(current.Row.Notes)),
				Recurrence: finance.Recurrence(current.Row.Recurrence),
				Tags:       current.Tags,
			}

			if cmd.Flags().Changed("date") {
				in.Date = strings.TrimSpace(flagDate)
			}
			if cmd.Flags().Changed("amount") {
				in.Amount = flagAmount
			}
			if cmd.Flags().Changed("kind") {
				k := finance.Kind(strings.ToLower(strings.TrimSpace(flagKind)))
				if !k.Valid() {
					return fmt.Errorf("invalid kind %q (valid: expense, income)", flagKind)
				}
				in.Kind = k
			}
			if cmd.Flags().Changed("account") {
				in.Account = flagAccount
			}
			if cmd.Flags().Changed("cat") {
				catID, err := resolveCategoryID(cmd.Context(), app.Queries, flagCat)
				if err != nil {
					return err
				}
				in.CategoryID = catID
			}
			if cmd.Flags().Changed("tag") {
				in.Tags = flagTags
			}
			if cmd.Flags().Changed("notes") {
				in.Notes = flagNotes
			}
			if cmd.Flags().Changed("recurrence") {
				r := finance.Recurrence(strings.ToLower(strings.TrimSpace(flagRecurrence)))
				if !r.Valid() {
					return fmt.Errorf("invalid recurrence %q (valid: none, daily, weekly, monthly)", flagRecurrence)
				}
				in.Recurrence = r
			}

			updated, err := svc.Update(cmd.Context(), id, in)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated %d: %s (%s)\n", id, updated.Row.Account, updated.Row.Kind)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "new date (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&flagAmount, "amount", 0, "new amount")
	cmd.Flags().StringVar(&flagKind, "kind", "", "new kind (expense, income)")
	cmd.Flags().StringVar(&flagAccount, "account", "", "new account")
	cmd.Flags().StringVar(&flagCat, "cat", "", "new category slug")
	cmd.Flags().StringSliceVar(&flagTags, "tag", nil, "replace tags (repeat for multiple)")
	cmd.Flags().StringVar(&flagNotes, "notes", "", "new notes")
	cmd.Flags().StringVar(&flagRecurrence, "recurrence", "", "new recurrence (none, daily, weekly, monthly)")
	return cmd
}
