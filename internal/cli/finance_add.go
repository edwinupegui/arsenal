package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/finance"
)

// newFinanceAddCmd builds `arsenal finance add`.
func newFinanceAddCmd() *cobra.Command {
	var (
		flagDate       string
		flagAmount     float64
		flagKind       string
		flagAccount    string
		flagCat        string
		flagTags       []string
		flagNotes      string
		flagRecurrence string
		flagJSON       bool
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new finance transaction",
		Example: `  arsenal finance add --amount 42.50 --kind expense --account checking --cat food --tag work --notes lunch
  arsenal finance add --amount 1000 --kind income --account salary --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("amount") {
				return fmt.Errorf("--amount is required")
			}

			kind := finance.Kind(strings.ToLower(strings.TrimSpace(flagKind)))
			if !kind.Valid() {
				return fmt.Errorf("kind must be expense or income")
			}
			recurrence := finance.Recurrence(strings.ToLower(strings.TrimSpace(flagRecurrence)))
			if flagRecurrence != "" && !recurrence.Valid() {
				return fmt.Errorf("invalid recurrence %q (valid: none, daily, weekly, monthly)", flagRecurrence)
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			catID, err := resolveCategoryID(cmd.Context(), app.Queries, flagCat)
			if err != nil {
				return err
			}

			created, err := finance.New(app.DB).Create(cmd.Context(), finance.CreateInput{
				Date:       strings.TrimSpace(flagDate),
				Amount:     flagAmount,
				Kind:       kind,
				Account:    flagAccount,
				CategoryID: catID,
				Notes:      flagNotes,
				Recurrence: recurrence,
				Tags:       flagTags,
			})
			if err != nil {
				return err
			}

			if flagJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(created)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "created %d: %s (%s)\n", created.Row.ID, created.Row.Account, created.Row.Kind)
			fmt.Fprintf(out, "  amount:     %s\n", finance.FormatAmount(created.Row.Amount, created.Row.Currency))
			fmt.Fprintf(out, "  date:       %s\n", created.Row.Date)
			if created.Row.Recurrence != "" {
				fmt.Fprintf(out, "  recurrence: %s\n", created.Row.Recurrence)
			}
			if flagCat != "" {
				fmt.Fprintf(out, "  category:   %s\n", flagCat)
			}
			if len(created.Tags) > 0 {
				fmt.Fprintf(out, "  tags:       %s\n", strings.Join(created.Tags, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "transaction date (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&flagAmount, "amount", 0, "transaction amount")
	cmd.Flags().StringVar(&flagKind, "kind", "", "transaction kind (expense, income)")
	cmd.Flags().StringVar(&flagAccount, "account", "", "account or payment method")
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug")
	cmd.Flags().StringSliceVar(&flagTags, "tag", nil, "tag (repeat for multiple)")
	cmd.Flags().StringVar(&flagNotes, "notes", "", "free-form notes")
	cmd.Flags().StringVar(&flagRecurrence, "recurrence", "", "recurrence (none, daily, weekly, monthly)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

// parseFinanceID parses a positional transaction id argument.
func parseFinanceID(arg string) (int64, error) {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("id must be an integer: %w", err)
	}
	return id, nil
}
