package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/todos"
)

// newTodoEditCmd builds `arsenal todo edit <id>` with optional field flags.
// Missing flags leave fields unchanged. --clear-due sets due_date to NULL.
func newTodoEditCmd() *cobra.Command {
	var (
		flagTitle       string
		flagDescription string
		flagPriority    string
		flagDue         string
		flagCat         string
		flagTags        []string
		flagNotes       string
		flagRecurrence  string
		flagClearDue    bool
	)
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an existing todo",
		Example: `  arsenal todo edit 5 --title "new title"
  arsenal todo edit 5 --priority high --due 2026-06-10 --clear-due`,
		Args: cobra.ExactArgs(1),
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

			svc := todos.New(app.DB)
			current, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("todo %d not found", id)
			}
			if err != nil {
				return err
			}

			// Start with current values, override with flags.
			in := todos.CreateInput{
				Title:       current.Row.Title,
				Description: derefString(current.Row.Description),
				Priority:    todos.Priority(current.Row.Priority),
				DueDate:     current.Row.DueDate,
				CategoryID:  current.Row.CategoryID,
				Notes:       derefString(current.Row.Notes),
				Recurrence:  todos.Recurrence(current.Row.Recurrence),
				Tags:        current.Tags,
			}

			if cmd.Flags().Changed("title") {
				in.Title = strings.TrimSpace(flagTitle)
			}
			if cmd.Flags().Changed("description") {
				in.Description = flagDescription
			}
			if cmd.Flags().Changed("priority") {
				p := todos.Priority(strings.ToLower(strings.TrimSpace(flagPriority)))
				if !p.Valid() {
					return fmt.Errorf("invalid priority %q (valid: low, med, high)", flagPriority)
				}
				in.Priority = p
			}
			if cmd.Flags().Changed("due") {
				d := strings.TrimSpace(flagDue)
				if d != "" {
					in.DueDate = &d
				}
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
				r := todos.Recurrence(strings.ToLower(strings.TrimSpace(flagRecurrence)))
				if !r.Valid() {
					return fmt.Errorf("invalid recurrence %q (valid: none, daily, weekly, monthly)", flagRecurrence)
				}
				in.Recurrence = r
			}
			if flagClearDue {
				in.DueDate = nil
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
	cmd.Flags().StringVar(&flagDescription, "description", "", "new description")
	cmd.Flags().StringVar(&flagPriority, "priority", "", "new priority (low, med, high)")
	cmd.Flags().StringVar(&flagDue, "due", "", "new due date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagCat, "cat", "", "new category slug")
	cmd.Flags().StringSliceVar(&flagTags, "tag", nil, "replace tags (repeat for multiple)")
	cmd.Flags().StringVar(&flagNotes, "notes", "", "new notes")
	cmd.Flags().StringVar(&flagRecurrence, "recurrence", "", "new recurrence (none, daily, weekly, monthly)")
	cmd.Flags().BoolVar(&flagClearDue, "clear-due", false, "clear the due date")
	return cmd
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
