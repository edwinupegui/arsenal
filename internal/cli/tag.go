package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/store"
)

// newTagCmd builds the `arsenal tag` subcommand tree.
func newTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage free-form tags",
	}
	cmd.AddCommand(newTagListCmd())
	cmd.AddCommand(newTagRenameCmd())
	cmd.AddCommand(newTagMergeCmd())
	return cmd
}

func newTagListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tags with the number of resources using each one",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			rows, err := app.Queries.ListTags(cmd.Context())
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no tags yet")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TAG\tRESOURCES")
			for _, t := range rows {
				fmt.Fprintf(w, "%s\t%d\n", t.Name, t.ResourceCount)
			}
			return w.Flush()
		},
	}
}

func newTagRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a tag everywhere",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName, newName := args[0], args[1]
			normNew, err := domain.NormalizeTag(newName)
			if err != nil {
				return fmt.Errorf("new name: %w", err)
			}
			normOld, err := domain.NormalizeTag(oldName)
			if err != nil {
				return fmt.Errorf("old name: %w", err)
			}
			if normOld == normNew {
				return fmt.Errorf("old and new normalized to %q — nothing to do", normNew)
			}
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			from, err := app.Queries.GetTagByName(cmd.Context(), normOld)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("tag %q not found", oldName)
			}
			if err != nil {
				return err
			}
			// If a tag with the new name already exists, route through merge
			// so the user doesn't get a UNIQUE constraint failure surprise.
			if _, err := app.Queries.GetTagByName(cmd.Context(), normNew); err == nil {
				return fmt.Errorf("tag %q already exists; use `arsenal tag merge %q %q` instead",
					normNew, oldName, newName)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if _, err := app.Queries.RenameTag(cmd.Context(), store.RenameTagParams{
				Name: normNew,
				ID:   from.ID,
			}); err != nil {
				return err
			}
			// FTS5 tags column is rebuilt by the resource_tags triggers when
			// the link set changes; renaming a tag in place doesn't fire those,
			// so we touch every resource holding it to refresh the index.
			if err := touchResourcesWithTag(cmd.Context(), app.DB, from.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "renamed %q -> %q\n", normOld, normNew)
			return nil
		},
	}
}

func newTagMergeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "merge <from> <into>",
		Short: "Merge all resources tagged <from> into <into>, then delete <from>",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromRaw, intoRaw := args[0], args[1]
			fromN, err := domain.NormalizeTag(fromRaw)
			if err != nil {
				return fmt.Errorf("from: %w", err)
			}
			intoN, err := domain.NormalizeTag(intoRaw)
			if err != nil {
				return fmt.Errorf("into: %w", err)
			}
			if fromN == intoN {
				return fmt.Errorf("from and into normalized to %q — nothing to do", fromN)
			}
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			from, err := app.Queries.GetTagByName(cmd.Context(), fromN)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("tag %q not found", fromRaw)
			}
			if err != nil {
				return err
			}
			into, err := upsertTagByName(cmd.Context(), app.Queries, intoN)
			if err != nil {
				return err
			}
			if !yes {
				ok, cerr := confirm(cmd.OutOrStdout(), os.Stdin,
					fmt.Sprintf("Merge tag %q into %q? [y/N] ", fromN, intoN))
				if cerr != nil {
					return cerr
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return nil
				}
			}

			tx, err := app.DB.BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback() }()

			// Reattach every resource that had the from tag to the into tag.
			// INSERT OR IGNORE handles cases where a resource already had both.
			if _, err := tx.ExecContext(cmd.Context(), `
				INSERT OR IGNORE INTO resource_tags (resource_id, tag_id)
				SELECT resource_id, ? FROM resource_tags WHERE tag_id = ?`,
				into.ID, from.ID); err != nil {
				return err
			}
			// Detach every link to the old tag (this fires the FTS trigger
			// per resource, which then re-aggregates tag list).
			if _, err := tx.ExecContext(cmd.Context(),
				`DELETE FROM resource_tags WHERE tag_id = ?`, from.ID); err != nil {
				return err
			}
			// Drop the now-orphan tag row.
			if _, err := tx.ExecContext(cmd.Context(),
				`DELETE FROM tags WHERE id = ?`, from.ID); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "merged %q into %q\n", fromN, intoN)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

// --- helpers -----------------------------------------------------------------

// touchResourcesWithTag triggers the FTS5 update path for every resource
// linked to tagID by re-issuing the same resource_tags insert. The INSERT
// OR IGNORE is a no-op for storage but fires the after-insert trigger that
// rebuilds the FTS tags column.
func touchResourcesWithTag(ctx context.Context, db *sql.DB, tagID int64) error {
	rows, err := db.QueryContext(ctx,
		`SELECT resource_id FROM resource_tags WHERE tag_id = ?`, tagID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := db.ExecContext(ctx,
			`INSERT OR IGNORE INTO resource_tags (resource_id, tag_id) VALUES (?, ?)`,
			id, tagID); err != nil {
			return err
		}
	}
	return nil
}

// upsertTagByName returns the tag row for normName, creating it if absent.
func upsertTagByName(ctx context.Context, q *store.Queries, normName string) (store.Tag, error) {
	if existing, err := q.GetTagByName(ctx, normName); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.Tag{}, err
	}
	return q.UpsertTag(ctx, normName)
}
