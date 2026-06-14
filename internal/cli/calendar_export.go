package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/calendar"
)

// newCalendarExportCmd builds `arsenal calendar export`.
// Exports events to iCal format (stdout by default, file when --output is given).
// Only "ical" is supported in v3.x; --format csv exits with an error.
func newCalendarExportCmd() *cobra.Command {
	var (
		flagFormat string
		flagOutput string
		flagFrom   string
		flagTo     string
		flagCat    string
		flagTag    string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export calendar events to iCal",
		Example: `  arsenal calendar export --format ical
  arsenal calendar export --format ical --output /tmp/calendar.ics
  arsenal calendar export --format ical --from 2026-06-01 --to 2026-06-30 --cat work`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagFormat != "ical" {
				return fmt.Errorf("unsupported format %q (only ical is supported)", flagFormat)
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

			rows, err := calendar.New(app.DB).Export(cmd.Context(), calendar.Filter{
				From:         fromPtr,
				To:           toPtr,
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

			return calendar.WriteICal(w, rows)
		},
	}

	cmd.Flags().StringVar(&flagFormat, "format", "ical", "export format (ical)")
	cmd.Flags().StringVar(&flagOutput, "output", "", "write to file instead of stdout")
	cmd.Flags().StringVar(&flagFrom, "from", "", "start bound (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "end bound (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug")
	cmd.Flags().StringVar(&flagTag, "tag", "", "tag name")
	return cmd
}
