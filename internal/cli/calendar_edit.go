package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/calendar"
)

// newCalendarEditCmd builds `arsenal calendar edit <id>` with optional field flags.
// Missing flags leave fields unchanged.
func newCalendarEditCmd() *cobra.Command {
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
	)
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an existing calendar event",
		Example: `  arsenal calendar edit 5 --title "Updated title" --recurrence weekly
  arsenal calendar edit 5 --cat work --tag team`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseCalendarID(args[0])
			if err != nil {
				return err
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := calendar.New(app.DB)
			current, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("event %d not found", id)
			}
			if err != nil {
				return err
			}

			// Populate input from current values.
			in := calendar.CreateInput{
				Title:       current.Row.Title,
				Description: derefString(nullableStringPtr(current.Row.Description)),
				StartAt:     current.Row.StartAt,
				EndAt:       derefString(nullableStringPtr(current.Row.EndAt)),
				AllDay:      current.Row.AllDay == 1,
				Location:    current.Row.Location,
				CategoryID:  nullableInt64Ptr(current.Row.CategoryID),
				Notes:       derefString(nullableStringPtr(current.Row.Notes)),
				Recurrence:  calendar.Recurrence(current.Row.Recurrence),
				Tags:        current.Tags,
			}

			if cmd.Flags().Changed("title") {
				in.Title = strings.TrimSpace(flagTitle)
			}
			if cmd.Flags().Changed("all-day") {
				in.AllDay = flagAllDay
			}
			if cmd.Flags().Changed("start") {
				allDayExplicit := cmd.Flags().Changed("all-day")
				startAt, allDay, err := normalizeCalendarDatetime(flagStart, in.AllDay, allDayExplicit)
				if err != nil {
					return fmt.Errorf("--start: %w", err)
				}
				in.StartAt = startAt
				if !cmd.Flags().Changed("all-day") {
					in.AllDay = allDay
				}
			}
			if cmd.Flags().Changed("end") {
				if flagEnd == "" {
					in.EndAt = ""
				} else {
					endAt, _, err := normalizeCalendarDatetime(flagEnd, in.AllDay, false)
					if err != nil {
						return fmt.Errorf("--end: %w", err)
					}
					in.EndAt = endAt
				}
			}
			if cmd.Flags().Changed("location") {
				in.Location = flagLocation
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
			if cmd.Flags().Changed("description") {
				in.Description = flagDescription
			}
			if cmd.Flags().Changed("notes") {
				in.Notes = flagNotes
			}
			if cmd.Flags().Changed("recurrence") {
				r := calendar.Recurrence(strings.ToLower(strings.TrimSpace(flagRecurrence)))
				if !r.Valid() {
					return fmt.Errorf("invalid recurrence %q (valid: none, daily, weekly, monthly, yearly)", flagRecurrence)
				}
				in.Recurrence = r
			}

			updated, err := svc.Update(cmd.Context(), id, in)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated %d: %s\n", id, updated.Row.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTitle, "title", "", "new title")
	cmd.Flags().StringVar(&flagStart, "start", "", "new start date/time (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	cmd.Flags().StringVar(&flagEnd, "end", "", "new end date/time (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	cmd.Flags().BoolVar(&flagAllDay, "all-day", false, "set all-day flag")
	cmd.Flags().StringVar(&flagLocation, "location", "", "new location")
	cmd.Flags().StringVar(&flagCat, "cat", "", "new category slug")
	cmd.Flags().StringSliceVar(&flagTags, "tag", nil, "replace tags (repeat for multiple)")
	cmd.Flags().StringVar(&flagDescription, "description", "", "new description")
	cmd.Flags().StringVar(&flagNotes, "notes", "", "new notes")
	cmd.Flags().StringVar(&flagRecurrence, "recurrence", "", "new recurrence (none, daily, weekly, monthly, yearly)")
	return cmd
}
