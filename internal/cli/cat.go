package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/store"
)

// newCatCmd builds the `arsenal cat` subcommand tree.
func newCatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cat",
		Short: "Manage curated categories",
	}
	cmd.AddCommand(newCatListCmd())
	cmd.AddCommand(newCatAddCmd())
	cmd.AddCommand(newCatRmCmd())
	return cmd
}

func newCatListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List categories with their active resource counts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			rows, err := app.Queries.ListCategoriesWithCounts(cmd.Context())
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no categories yet")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SORT\tSLUG\tICON\tNAME\tACTIVE")
			for _, c := range rows {
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\n",
					c.SortOrder, c.Slug, c.Icon, c.Name, c.ResourceCount)
			}
			return w.Flush()
		},
	}
}

func newCatAddCmd() *cobra.Command {
	var (
		slug string
		name string
		icon string
		sort int64
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new category",
		Example: `  arsenal cat add --slug devops --name "DevOps & SRE" --icon server
  arsenal cat add -s observability -n "Observability" -i activity --sort 50`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			slug = strings.TrimSpace(slug)
			name = strings.TrimSpace(name)
			icon = strings.TrimSpace(icon)
			if slug == "" || name == "" || icon == "" {
				return fmt.Errorf("--slug, --name and --icon are required")
			}
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			if existing, err := app.Queries.GetCategoryBySlug(cmd.Context(), slug); err == nil {
				return fmt.Errorf("category %q already exists (id %d)", slug, existing.ID)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}

			created, err := app.Queries.CreateCategory(cmd.Context(), store.CreateCategoryParams{
				Slug:      slug,
				Name:      name,
				Icon:      icon,
				SortOrder: sort,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added category %d %s (%s)\n",
				created.ID, created.Slug, created.Name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&slug, "slug", "s", "", "lowercase slug (required, unique)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "display name (required)")
	cmd.Flags().StringVarP(&icon, "icon", "i", "", "icon identifier, e.g. lucide name (required)")
	cmd.Flags().Int64Var(&sort, "sort", 100, "sort order (lower comes first)")
	return cmd
}

func newCatRmCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rm <slug>",
		Short: "Delete a category. Resources keep their data; their category becomes uncategorized.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := strings.ToLower(strings.TrimSpace(args[0]))
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			cat, err := app.Queries.GetCategoryBySlug(cmd.Context(), slug)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("category %q not found", slug)
			}
			if err != nil {
				return err
			}

			count, err := countResourcesInCategory(cmd.Context(), app.DB, cat.ID)
			if err != nil {
				return err
			}
			if !yes {
				prompt := fmt.Sprintf("Delete category %q (%s)? %d active resources will become uncategorized. [y/N] ",
					cat.Slug, cat.Name, count)
				ok, err := confirm(cmd.OutOrStdout(), os.Stdin, prompt)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return nil
				}
			}
			if err := app.Queries.DeleteCategory(cmd.Context(), cat.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted category %s; %d resources are now uncategorized\n",
				cat.Slug, count)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

// countResourcesInCategory returns the number of non-trashed resources still
// attached to catID. Used in cat rm to preview the impact of the FK
// ON DELETE SET NULL cascade.
func countResourcesInCategory(ctx context.Context, db *sql.DB, catID int64) (int64, error) {
	var n int64
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM resources WHERE category_id = ? AND deleted_at IS NULL`,
		catID,
	).Scan(&n)
	return n, err
}

