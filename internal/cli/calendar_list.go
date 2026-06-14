package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/calendar"
)

// newCalendarListCmd builds `arsenal calendar list` with filter flags.
func newCalendarListCmd() *cobra.Command {
	var (
		flagFrom       string
		flagTo         string
		flagAllDay     bool
		flagRecurrence string
		flagCat        string
		flagTag        string
		flagTrashed    bool
		flagLimit      int
		flagOffset     int
		flagJSON       bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List calendar events, optionally filtered",
		Example: `  arsenal calendar list
  arsenal calendar list --from 2026-06-01 --to 2026-06-30 --json
  arsenal calendar list --recurrence daily --trashed`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			recurrence := strings.ToLower(strings.TrimSpace(flagRecurrence))
			if recurrence != "" {
				if !calendar.Recurrence(recurrence).Valid() {
					return fmt.Errorf("invalid recurrence %q (valid: none, daily, weekly, monthly, yearly)", flagRecurrence)
				}
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			var fromPtr, toPtr, recurrencePtr *string
			if f := strings.TrimSpace(flagFrom); f != "" {
				fromPtr = &f
			}
			if t := strings.TrimSpace(flagTo); t != "" {
				toPtr = &t
			}
			if recurrence != "" {
				recurrencePtr = &recurrence
			}

			items, err := calendar.New(app.DB).List(cmd.Context(), calendar.Filter{
				From:         fromPtr,
				To:           toPtr,
				Recurrence:   recurrencePtr,
				CategorySlug: flagCat,
				TagName:      flagTag,
				Trashed:      flagTrashed,
				Limit:        flagLimit,
				Offset:       flagOffset,
			})
			if err != nil {
				return err
			}

			// Apply all-day filter in memory (not supported by store filter).
			if cmd.Flags().Changed("all-day") {
				filtered := items[:0]
				for _, ev := range items {
					if (ev.Row.AllDay == 1) == flagAllDay {
						filtered = append(filtered, ev)
					}
				}
				items = filtered
			}

			if flagJSON {
				return writeCalendarJSON(cmd.OutOrStdout(), items)
			}
			writeCalendarTable(cmd.OutOrStdout(), items)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "start bound (YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS)")
	cmd.Flags().StringVar(&flagTo, "to", "", "end bound (YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS)")
	cmd.Flags().BoolVar(&flagAllDay, "all-day", false, "filter to all-day events only")
	cmd.Flags().StringVar(&flagRecurrence, "recurrence", "", "recurrence filter (none, daily, weekly, monthly, yearly)")
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug")
	cmd.Flags().StringVar(&flagTag, "tag", "", "tag name")
	cmd.Flags().BoolVar(&flagTrashed, "trashed", false, "only trashed events")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "max rows to return")
	cmd.Flags().IntVar(&flagOffset, "offset", 0, "rows to skip")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

func writeCalendarTable(out io.Writer, items []*calendar.Event) {
	if len(items) == 0 {
		fmt.Fprintln(out, "(no events)")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTART\tALL-DAY\tTITLE\tRECURRENCE")
	for _, ev := range items {
		allDay := "no"
		if ev.Row.AllDay == 1 {
			allDay = "yes"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			ev.Row.ID,
			ev.Row.StartAt,
			allDay,
			truncate(ev.Row.Title, 50),
			ev.Row.Recurrence,
		)
	}
	_ = w.Flush()
}

func writeCalendarJSON(out io.Writer, items []*calendar.Event) error {
	type calendarJSON struct {
		ID          int64    `json:"id"`
		Title       string   `json:"title"`
		StartAt     string   `json:"start_at"`
		EndAt       *string  `json:"end_at,omitempty"`
		AllDay      bool     `json:"all_day"`
		Location    string   `json:"location"`
		Description *string  `json:"description,omitempty"`
		Notes       *string  `json:"notes,omitempty"`
		CategoryID  *int64   `json:"category_id,omitempty"`
		Recurrence  string   `json:"recurrence"`
		CreatedAt   string   `json:"created_at"`
		UpdatedAt   string   `json:"updated_at"`
		DeletedAt   *string  `json:"deleted_at,omitempty"`
		Tags        []string `json:"tags"`
	}

	mapped := make([]calendarJSON, 0, len(items))
	for _, ev := range items {
		mapped = append(mapped, calendarJSON{
			ID:          ev.Row.ID,
			Title:       ev.Row.Title,
			StartAt:     ev.Row.StartAt,
			EndAt:       nullableStringPtr(ev.Row.EndAt),
			AllDay:      ev.Row.AllDay == 1,
			Location:    ev.Row.Location,
			Description: nullableStringPtr(ev.Row.Description),
			Notes:       nullableStringPtr(ev.Row.Notes),
			CategoryID:  nullableInt64Ptr(ev.Row.CategoryID),
			Recurrence:  ev.Row.Recurrence,
			CreatedAt:   ev.Row.CreatedAt,
			UpdatedAt:   ev.Row.UpdatedAt,
			DeletedAt:   nullableStringPtr(ev.Row.DeletedAt),
			Tags:        ev.Tags,
		})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(mapped)
}
