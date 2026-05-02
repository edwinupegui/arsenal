package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/resources"
)

// newShowCmd builds `arsenal show <id>` to print a single resource detail.
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show full detail for a single resource by id",
		Args:  cobra.ExactArgs(1),
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

			svc := resources.New(app.DB)
			res, err := svc.Get(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("resource %d not found", id)
			}
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "ID:          %d\n", res.Row.ID)
			fmt.Fprintf(out, "Title:       %s\n", res.Row.Title)
			fmt.Fprintf(out, "URL:         %s\n", res.Row.Url)
			fmt.Fprintf(out, "Type:        %s\n", res.Row.Type)
			fmt.Fprintf(out, "Language:    %s\n", res.Row.Language)
			fmt.Fprintf(out, "Favorite:    %s\n", boolMark(res.Row.Favorite == 1))

			if res.Row.CategoryID.Valid {
				cat, err := app.Queries.GetCategory(cmd.Context(), res.Row.CategoryID.Int64)
				if err == nil {
					fmt.Fprintf(out, "Category:    %s (%s)\n", cat.Name, cat.Slug)
				}
			} else {
				fmt.Fprintln(out, "Category:    -")
			}

			tags := "-"
			if len(res.Tags) > 0 {
				tags = strings.Join(res.Tags, ", ")
			}
			fmt.Fprintf(out, "Tags:        %s\n", tags)
			fmt.Fprintf(out, "Created:     %s\n", res.Row.CreatedAt)
			fmt.Fprintf(out, "Updated:     %s\n", res.Row.UpdatedAt)
			if res.Row.DeletedAt.Valid {
				fmt.Fprintf(out, "Trashed at:  %s\n", res.Row.DeletedAt.String)
			}
			if res.Row.Description.Valid && res.Row.Description.String != "" {
				fmt.Fprintln(out)
				fmt.Fprintln(out, "Description:")
				fmt.Fprintln(out, indent(res.Row.Description.String, "  "))
			}
			if res.Row.Notes.Valid && res.Row.Notes.String != "" {
				fmt.Fprintln(out)
				fmt.Fprintln(out, "Notes:")
				fmt.Fprintln(out, indent(res.Row.Notes.String, "  "))
			}
			return nil
		},
	}
}

func boolMark(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
