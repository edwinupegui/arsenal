package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/calendar"
)

// newCalendarAddCmd builds `arsenal calendar add`.
func newCalendarAddCmd() *cobra.Command {
	var (
		flagTitle       string
		flagStart       string
		flagEnd         string
		flagAllDay      bool
		flagLocation    string
		flagCat         string
		flagTags        []string
		flagDescription string
		flagNotes       string
		flagRecurrence  string
		flagJSON        bool
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new calendar event",
		Example: `  arsenal calendar add --title "Team standup" --start 2026-06-15T09:00 --end 2026-06-15T09:30 --recurrence daily
  arsenal calendar add --title "Holiday" --start 2026-12-25 --all-day
  arsenal calendar add --title "Meeting" --start 2026-06-15T14:00 --cat work --tag team --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(flagTitle) == "" {
				return fmt.Errorf("--title is required")
			}
			if strings.TrimSpace(flagStart) == "" {
				return fmt.Errorf("--start is required")
			}

			// Normalize start and end to storage format; infer all_day when not explicit.
			startAt, allDay, err := normalizeCalendarDatetime(flagStart, flagAllDay, cmd.Flags().Changed("all-day"))
			if err != nil {
				return fmt.Errorf("--start: %w", err)
			}

			endAt := ""
			if strings.TrimSpace(flagEnd) != "" {
				endAt, _, err = normalizeCalendarDatetime(flagEnd, allDay, false)
				if err != nil {
					return fmt.Errorf("--end: %w", err)
				}
			}

			recurrence := calendar.Recurrence(strings.ToLower(strings.TrimSpace(flagRecurrence)))
			if flagRecurrence != "" && !recurrence.Valid() {
				return fmt.Errorf("invalid recurrence %q (valid: none, daily, weekly, monthly, yearly)", flagRecurrence)
			}
			if flagRecurrence == "" {
				recurrence = calendar.RecurrenceNone
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

			created, err := calendar.New(app.DB).Create(cmd.Context(), calendar.CreateInput{
				Title:       strings.TrimSpace(flagTitle),
				Description: flagDescription,
				StartAt:     startAt,
				EndAt:       endAt,
				AllDay:      allDay,
				Location:    flagLocation,
				CategoryID:  catID,
				Notes:       flagNotes,
				Recurrence:  recurrence,
				Tags:        flagTags,
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
			fmt.Fprintf(out, "created %d: %s\n", created.Row.ID, created.Row.Title)
			fmt.Fprintf(out, "  start:      %s\n", created.Row.StartAt)
			if created.Row.EndAt.Valid {
				fmt.Fprintf(out, "  end:        %s\n", created.Row.EndAt.String)
			}
			if allDay {
				fmt.Fprintf(out, "  all-day:    true\n")
			}
			if created.Row.Location != "" {
				fmt.Fprintf(out, "  location:   %s\n", created.Row.Location)
			}
			if created.Row.Recurrence != "none" {
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
	cmd.Flags().StringVar(&flagTitle, "title", "", "event title")
	cmd.Flags().StringVar(&flagStart, "start", "", "start date/time (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	cmd.Flags().StringVar(&flagEnd, "end", "", "end date/time (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	cmd.Flags().BoolVar(&flagAllDay, "all-day", false, "mark as all-day event")
	cmd.Flags().StringVar(&flagLocation, "location", "", "event location")
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug")
	cmd.Flags().StringSliceVar(&flagTags, "tag", nil, "tag (repeat for multiple)")
	cmd.Flags().StringVar(&flagDescription, "description", "", "event description")
	cmd.Flags().StringVar(&flagNotes, "notes", "", "free-form notes")
	cmd.Flags().StringVar(&flagRecurrence, "recurrence", "", "recurrence (none, daily, weekly, monthly, yearly)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

// normalizeCalendarDatetime converts a user-supplied start/end value to the
// canonical storage format and infers all_day when the explicit flag is not set.
//
//   - "YYYY-MM-DD" → storage "YYYY-MM-DD", allDay=true
//   - "YYYY-MM-DDTHH:MM" → storage "YYYY-MM-DDTHH:MM:SS", allDay=false
//   - "YYYY-MM-DDTHH:MM:SS" → storage verbatim, allDay=false
//
// When allDayExplicit is true the caller's all-day flag overrides inference.
func normalizeCalendarDatetime(s string, allDayFlag bool, allDayExplicit bool) (string, bool, error) {
	s = strings.TrimSpace(s)

	// Try date-only format.
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return s, true, nil
	}

	// Try YYYY-MM-DDTHH:MM (without seconds — add :00).
	if t, err := time.ParseInLocation("2006-01-02T15:04", s, time.Local); err == nil {
		if allDayExplicit && allDayFlag {
			return "", false, fmt.Errorf("datetime %q is not compatible with --all-day (use YYYY-MM-DD for all-day events)", s)
		}
		return t.Format("2006-01-02T15:04:05"), false, nil
	}

	// Try full datetime format.
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local); err == nil {
		if allDayExplicit && allDayFlag {
			return "", false, fmt.Errorf("datetime %q is not compatible with --all-day (use YYYY-MM-DD for all-day events)", s)
		}
		return t.Format("2006-01-02T15:04:05"), false, nil
	}

	return "", false, fmt.Errorf("unrecognized format %q (expected YYYY-MM-DD or YYYY-MM-DDTHH:MM)", s)
}

// parseCalendarID parses a positional event id argument.
func parseCalendarID(arg string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(arg, "%d", &id); err != nil {
		return 0, fmt.Errorf("id must be an integer: %w", err)
	}
	return id, nil
}
