package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/finance"
)

// newFinanceExportCmd builds `arsenal finance export`.
// Exports transactions to CSV (stdout by default, file when --output is given).
// Encoding: UTF-8, comma delimiter, RFC 4180 escaping via encoding/csv.
// Tags are comma-separated within a single cell (encoding/csv quotes the cell
// when it contains a comma, satisfying RFC 4180 automatically).
func newFinanceExportCmd() *cobra.Command {
	var (
		flagFormat string
		flagOutput string
		flagFrom   string
		flagTo     string
		flagKind   string
		flagCat    string
		flagTag    string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export transactions to CSV",
		Example: `  arsenal finance export --format csv
  arsenal finance export --format csv --output /tmp/finance.csv
  arsenal finance export --format csv --from 2026-06-01 --to 2026-06-30 --kind expense`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagFormat != "csv" {
				return fmt.Errorf("unsupported format %q (only csv is supported)", flagFormat)
			}

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

			rows, err := finance.New(app.DB).Export(cmd.Context(), finance.Filter{
				From:         fromPtr,
				To:           toPtr,
				Kind:         nilOrString(kind),
				CategorySlug: flagCat,
				TagName:      flagTag,
			})
			if err != nil {
				return err
			}

			var w io.Writer
			if flagOutput != "" {
				f, err := os.Create(flagOutput)
				if err != nil {
					return fmt.Errorf("create output file: %w", err)
				}
				defer f.Close()
				w = f
			} else {
				w = cmd.OutOrStdout()
			}

			return writeFinanceCSV(w, rows)
		},
	}

	cmd.Flags().StringVar(&flagFormat, "format", "csv", "export format (csv)")
	cmd.Flags().StringVar(&flagOutput, "output", "", "write to file instead of stdout")
	cmd.Flags().StringVar(&flagFrom, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagKind, "kind", "", "kind filter (expense, income)")
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug")
	cmd.Flags().StringVar(&flagTag, "tag", "", "tag name")
	return cmd
}

// csvHeader is the ordered column list required by the finance-csv-export spec.
var csvHeader = []string{"date", "kind", "amount", "currency", "account", "category", "notes", "tags"}

// writeFinanceCSV writes rows to w using encoding/csv (RFC 4180). The header
// is always written; an empty rows slice produces only the header row.
func writeFinanceCSV(w io.Writer, rows []finance.ExportRow) error {
	cw := csv.NewWriter(w)

	if err := cw.Write(csvHeader); err != nil {
		return err
	}

	for _, r := range rows {
		record := []string{
			r.Date,
			r.Kind,
			fmt.Sprintf("%.2f", r.Amount),
			r.Currency,
			r.Account,
			r.Category,
			r.Notes,
			strings.Join(r.Tags, ","),
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}
