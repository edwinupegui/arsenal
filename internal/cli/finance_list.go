package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/finance"
)

// newFinanceListCmd builds `arsenal finance list` with filter flags.
func newFinanceListCmd() *cobra.Command {
	var (
		flagFrom    string
		flagTo      string
		flagKind    string
		flagCat     string
		flagTag     string
		flagTrashed bool
		flagLimit   int
		flagOffset  int
		flagJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List finance transactions, optionally filtered",
		Example: `  arsenal finance list
  arsenal finance list --from 2026-06-01 --to 2026-06-30 --kind expense --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			kind := strings.ToLower(strings.TrimSpace(flagKind))
			if kind != "" {
				if !finance.Kind(kind).Valid() {
					return fmt.Errorf("invalid kind %q (valid: expense, income)", flagKind)
				}
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			var fromPtr, toPtr *string
			if f := strings.TrimSpace(flagFrom); f != "" {
				fromPtr = &f
			}
			if t := strings.TrimSpace(flagTo); t != "" {
				toPtr = &t
			}

			items, err := finance.New(app.DB).List(cmd.Context(), finance.Filter{
				From:         fromPtr,
				To:           toPtr,
				Kind:         nilOrString(kind),
				CategorySlug: flagCat,
				TagName:      flagTag,
				Trashed:      flagTrashed,
				Limit:        flagLimit,
				Offset:       flagOffset,
			})
			if err != nil {
				return err
			}

			if flagJSON {
				return writeFinanceJSON(cmd.OutOrStdout(), items)
			}
			writeFinanceTable(cmd.OutOrStdout(), items)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagKind, "kind", "", "kind filter (expense, income)")
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug")
	cmd.Flags().StringVar(&flagTag, "tag", "", "tag name")
	cmd.Flags().BoolVar(&flagTrashed, "trashed", false, "only trashed transactions")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "max rows to return")
	cmd.Flags().IntVar(&flagOffset, "offset", 0, "rows to skip")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

func writeFinanceTable(out io.Writer, items []*finance.Transaction) {
	if len(items) == 0 {
		fmt.Fprintln(out, "(no transactions)")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDATE\tKIND\tAMOUNT\tACCOUNT")
	for _, tx := range items {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			tx.Row.ID,
			tx.Row.Date,
			tx.Row.Kind,
			finance.FormatAmount(tx.Row.Amount, tx.Row.Currency),
			truncate(tx.Row.Account, 40),
		)
	}
	_ = w.Flush()
}

func writeFinanceJSON(out io.Writer, items []*finance.Transaction) error {
	type financeJSON struct {
		ID         int64    `json:"id"`
		Date       string   `json:"date"`
		Amount     float64  `json:"amount"`
		Kind       string   `json:"kind"`
		Currency   string   `json:"currency"`
		Account    string   `json:"account"`
		CategoryID *int64   `json:"category_id,omitempty"`
		Notes      *string  `json:"notes,omitempty"`
		Recurrence string   `json:"recurrence"`
		CreatedAt  string   `json:"created_at"`
		UpdatedAt  string   `json:"updated_at"`
		DeletedAt  *string  `json:"deleted_at,omitempty"`
		Tags       []string `json:"tags"`
	}

	mapped := make([]financeJSON, 0, len(items))
	for _, tx := range items {
		mapped = append(mapped, financeJSON{
			ID:         tx.Row.ID,
			Date:       tx.Row.Date,
			Amount:     tx.Row.Amount,
			Kind:       tx.Row.Kind,
			Currency:   tx.Row.Currency,
			Account:    tx.Row.Account,
			CategoryID: nullableInt64Ptr(tx.Row.CategoryID),
			Notes:      nullableStringPtr(tx.Row.Notes),
			Recurrence: tx.Row.Recurrence,
			CreatedAt:  tx.Row.CreatedAt,
			UpdatedAt:  tx.Row.UpdatedAt,
			DeletedAt:  nullableStringPtr(tx.Row.DeletedAt),
			Tags:       tx.Tags,
		})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(mapped)
}

func nilOrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullableInt64Ptr(n sql.NullInt64) *int64 {
	if n.Valid {
		return &n.Int64
	}
	return nil
}

func nullableStringPtr(n sql.NullString) *string {
	if n.Valid {
		return &n.String
	}
	return nil
}
