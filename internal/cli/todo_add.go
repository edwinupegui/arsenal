package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/todos"
)

// newTodoAddCmd builds `arsenal todo add <title>`.
func newTodoAddCmd() *cobra.Command {
	var (
		flagPriority   string
		flagDue        string
		flagCat        string
		flagTags       []string
		flagNotes      string
		flagRecurrence string
		flagDesc       string
		flagJSON       bool
	)
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Create a new todo",
		Example: `  arsenal todo add "review PRs"
  arsenal todo add "pay electricity" --priority high --due 2026-06-10 --tag urgent --tag home --notes "monthly" --recurrence weekly`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.TrimSpace(args[0])
			if title == "" {
				return fmt.Errorf("title is required")
			}

			p := todos.Priority(strings.ToLower(strings.TrimSpace(flagPriority)))
			if flagPriority != "" && !p.Valid() {
				return fmt.Errorf("invalid priority %q (valid: low, med, high)", flagPriority)
			}
			r := todos.Recurrence(strings.ToLower(strings.TrimSpace(flagRecurrence)))
			if flagRecurrence != "" && !r.Valid() {
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

			var due *string
			if strings.TrimSpace(flagDue) != "" {
				d := strings.TrimSpace(flagDue)
				due = &d
			}

			created, err := todos.New(app.DB).Create(cmd.Context(), todos.CreateInput{
				Title:       title,
				Description: flagDesc,
				Priority:    p,
				DueDate:     due,
				CategoryID:  catID,
				Notes:       flagNotes,
				Recurrence:  r,
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
			fmt.Fprintf(out, "  priority:   %s\n", created.Row.Priority)
			if created.Row.DueDate != nil {
				fmt.Fprintf(out, "  due:        %s\n", *created.Row.DueDate)
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
	cmd.Flags().StringVar(&flagPriority, "priority", "", "priority (low, med, high)")
	cmd.Flags().StringVar(&flagDue, "due", "", "due date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug")
	cmd.Flags().StringSliceVar(&flagTags, "tag", nil, "tag (repeat for multiple)")
	cmd.Flags().StringVar(&flagNotes, "notes", "", "free-form notes")
	cmd.Flags().StringVar(&flagRecurrence, "recurrence", "", "recurrence (none, daily, weekly, monthly)")
	cmd.Flags().StringVar(&flagDesc, "desc", "", "description")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}
