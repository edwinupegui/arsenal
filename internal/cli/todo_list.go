package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/todos"
)

// newTodoListCmd builds `arsenal todo list` with filter flags.
func newTodoListCmd() *cobra.Command {
	var (
		flagStatus   string
		flagPriority string
		flagOverdue  bool
		flagCat      string
		flagTag      string
		flagTrashed  bool
		flagLimit    int
		flagOffset   int
		flagJSON     bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List todos, optionally filtered",
		Example: `  arsenal todo list
  arsenal todo list --priority high --overdue
  arsenal todo list --status done --limit 10 --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			status := todos.Status(strings.ToLower(strings.TrimSpace(flagStatus)))
			if flagStatus != "" && !status.Valid() {
				return fmt.Errorf("invalid status %q (valid: open, done)", flagStatus)
			}
			priority := todos.Priority(strings.ToLower(strings.TrimSpace(flagPriority)))
			if flagPriority != "" && !priority.Valid() {
				return fmt.Errorf("invalid priority %q (valid: low, med, high)", flagPriority)
			}

			if status == "" && !flagTrashed {
				status = todos.StatusOpen
			}

			items, err := todos.New(app.DB).List(cmd.Context(), todos.ListFilter{
				Status:       status,
				Priority:     priority,
				OnlyOverdue:  flagOverdue,
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
				return writeTodoJSON(cmd.OutOrStdout(), items)
			}
			writeTodoTable(cmd.OutOrStdout(), items)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagStatus, "status", "", "status filter (open, done)")
	cmd.Flags().StringVar(&flagPriority, "priority", "", "priority filter (low, med, high)")
	cmd.Flags().BoolVar(&flagOverdue, "overdue", false, "only overdue open todos")
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug")
	cmd.Flags().StringVar(&flagTag, "tag", "", "tag name")
	cmd.Flags().BoolVar(&flagTrashed, "trashed", false, "only trashed todos")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "max rows to return")
	cmd.Flags().IntVar(&flagOffset, "offset", 0, "rows to skip")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

func writeTodoTable(out io.Writer, items []*todos.Todo) {
	if len(items) == 0 {
		fmt.Fprintln(out, "(no todos)")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tPRIORITY\tDUE\tTITLE")
	for _, t := range items {
		due := "-"
		if t.Row.DueDate != nil {
			due = *t.Row.DueDate
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			t.Row.ID, t.Row.Status, t.Row.Priority, due, truncate(t.Row.Title, 60))
	}
	_ = w.Flush()
}

func writeTodoJSON(out io.Writer, items []*todos.Todo) error {
	type todoJSON struct {
		ID          int64    `json:"id"`
		Title       string   `json:"title"`
		Description *string  `json:"description,omitempty"`
		Priority    string   `json:"priority"`
		Status      string   `json:"status"`
		DueDate     *string  `json:"due_date,omitempty"`
		CategoryID  *int64   `json:"category_id,omitempty"`
		Notes       *string  `json:"notes,omitempty"`
		Recurrence  string   `json:"recurrence"`
		DoneAt      *string  `json:"done_at,omitempty"`
		CreatedAt   string   `json:"created_at"`
		UpdatedAt   string   `json:"updated_at"`
		DeletedAt   *string  `json:"deleted_at,omitempty"`
		Tags        []string `json:"tags"`
	}

	mapped := make([]todoJSON, 0, len(items))
	for _, t := range items {
		mapped = append(mapped, todoJSON{
			ID:          t.Row.ID,
			Title:       t.Row.Title,
			Description: t.Row.Description,
			Priority:    t.Row.Priority,
			Status:      t.Row.Status,
			DueDate:     t.Row.DueDate,
			CategoryID:  t.Row.CategoryID,
			Notes:       t.Row.Notes,
			Recurrence:  t.Row.Recurrence,
			DoneAt:      t.Row.DoneAt,
			CreatedAt:   t.Row.CreatedAt,
			UpdatedAt:   t.Row.UpdatedAt,
			DeletedAt:   t.Row.DeletedAt,
			Tags:        t.Tags,
		})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(mapped)
}
