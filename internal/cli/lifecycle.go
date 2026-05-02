package cli

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
)

// newRmCmd: soft-delete a resource (moves it to trash).
func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Move a resource to the trash (soft delete)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			res, err := requireResource(cmd.Context(), app.Queries, id)
			if err != nil {
				return err
			}
			if res.DeletedAt.Valid {
				return fmt.Errorf("resource %d is already in trash", id)
			}
			svc := resources.New(app.DB)
			if err := svc.SoftDelete(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "moved %d to trash: %s\n", id, res.Title)
			return nil
		},
	}
}

// newRestoreCmd: bring a trashed resource back.
func newRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a trashed resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			res, err := requireResource(cmd.Context(), app.Queries, id)
			if err != nil {
				return err
			}
			if !res.DeletedAt.Valid {
				return fmt.Errorf("resource %d is not in trash", id)
			}
			svc := resources.New(app.DB)
			if err := svc.Restore(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "restored %d: %s\n", id, res.Title)
			return nil
		},
	}
}

// newPurgeCmd: permanently delete a resource. Asks for Y/n on stdin
// unless --yes is set.
func newPurgeCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "purge <id>",
		Short: "Permanently delete a resource (cannot be undone)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			res, err := requireResource(cmd.Context(), app.Queries, id)
			if err != nil {
				return err
			}
			if !yes {
				ok, err := confirm(cmd.OutOrStdout(), os.Stdin,
					fmt.Sprintf("Permanently delete %d %q? [y/N] ", id, res.Title))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted")
					return nil
				}
			}
			svc := resources.New(app.DB)
			if err := svc.Purge(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "purged %d: %s\n", id, res.Title)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

// newTrashCmd: thin alias over `list --trashed` for ergonomics.
func newTrashCmd() *cobra.Command {
	var (
		flagLimit int
		flagJSON  bool
	)
	cmd := &cobra.Command{
		Use:   "trash",
		Short: "List trashed (soft-deleted) resources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()
			items, err := app.Queries.ListResourcesFiltered(cmd.Context(), store.ListFilter{
				Trashed: true,
				Limit:   flagLimit,
			})
			if err != nil {
				return err
			}
			if flagJSON {
				return writeJSON(cmd.OutOrStdout(), items)
			}
			writeTable(cmd.OutOrStdout(), items)
			return nil
		},
	}
	cmd.Flags().IntVarP(&flagLimit, "limit", "n", 50, "max rows to return")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON instead of a table")
	return cmd
}

// newStarCmd / newUnstarCmd: toggle the favorite flag.
func newStarCmd() *cobra.Command {
	return makeStarCmd("star", true,
		"Mark a resource as favorite",
		"starred %d: %s\n")
}

func newUnstarCmd() *cobra.Command {
	return makeStarCmd("unstar", false,
		"Remove the favorite mark from a resource",
		"unstarred %d: %s\n")
}

func makeStarCmd(name string, fav bool, short, msg string) *cobra.Command {
	return &cobra.Command{
		Use:   name + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			res, err := requireResource(cmd.Context(), app.Queries, id)
			if err != nil {
				return err
			}
			svc := resources.New(app.DB)
			if err := svc.SetFavorite(cmd.Context(), id, fav); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), msg, id, res.Title)
			return nil
		},
	}
}

// --- helpers -----------------------------------------------------------------

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("id must be an integer: %w", err)
	}
	return id, nil
}

// requireResource fetches a resource by id and turns sql.ErrNoRows into a
// stable user-facing error.
func requireResource(ctx context.Context, q *store.Queries, id int64) (store.Resource, error) {
	res, err := q.GetResource(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Resource{}, fmt.Errorf("resource %d not found", id)
	}
	return res, err
}

// confirm prints prompt to out, reads a line from in, and returns true when
// the user typed y / yes (case-insensitive). EOF or any other input is false.
func confirm(out io.Writer, in io.Reader, prompt string) (bool, error) {
	_, _ = fmt.Fprint(out, prompt)
	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes", nil
}
