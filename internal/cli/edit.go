package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/exportmd"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
)

// newEditCmd builds `arsenal edit <id>`. It serializes the current resource
// into the same markdown format `arsenal export` produces, hands it to the
// user's $EDITOR (vi when unset), then re-parses the saved file and applies
// the diff through resources.Service.Update. This keeps editing as
// expressive as the form (every field reachable) without piling up flags.
func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a resource in $EDITOR (markdown with frontmatter)",
		Long: `Edit opens the resource as a markdown file in your $EDITOR (defaults to
vi if unset), waits for the editor to exit, then parses your changes back
and applies them. Frontmatter fields (title, url, type, language,
category, tags, favorite) are mutable; the body becomes the description,
and an optional '## Notes' section becomes the notes field. Closing the
editor without saving leaves the resource unchanged.`,
		Args: cobra.ExactArgs(1),
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

			res, err := app.Queries.GetResource(cmd.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("resource %d not found", id)
			}
			if err != nil {
				return err
			}

			tmpPath, err := writeEditBuffer(cmd.Context(), app.Queries, res)
			if err != nil {
				return err
			}
			defer os.Remove(tmpPath)

			beforeStat, err := os.Stat(tmpPath)
			if err != nil {
				return err
			}

			if err := openInEditor(tmpPath); err != nil {
				return err
			}

			afterStat, err := os.Stat(tmpPath)
			if err != nil {
				return err
			}
			if afterStat.ModTime().Equal(beforeStat.ModTime()) && afterStat.Size() == beforeStat.Size() {
				fmt.Fprintln(cmd.OutOrStdout(), "no changes; resource left unchanged")
				return nil
			}

			pf, err := exportmd.ParseFile(tmpPath)
			if err != nil {
				return fmt.Errorf("parse edited buffer: %w", err)
			}

			catID, err := resolveCategoryID(cmd.Context(), app.Queries, pf.Category)
			if err != nil {
				return err
			}

			rtype := domain.ResourceType(strings.ToLower(pf.Type))
			if !rtype.Valid() {
				return fmt.Errorf("invalid type %q", pf.Type)
			}
			lang := domain.Language(strings.ToUpper(pf.Language))
			if !lang.Valid() {
				return fmt.Errorf("invalid language %q", pf.Language)
			}

			if _, err := resources.New(app.DB).Update(cmd.Context(), id, resources.UpdateInput{
				Title:       pf.Title,
				URL:         pf.URL,
				Description: pf.Description,
				Type:        rtype,
				Language:    lang,
				CategoryID:  catID,
				Notes:       pf.Notes,
				Tags:        pf.Tags,
			}); err != nil {
				return err
			}
			// Favorite isn't part of UpdateInput; flip it through SetFavorite
			// so the round-trip preserves the flag.
			svc := resources.New(app.DB)
			if err := svc.SetFavorite(cmd.Context(), id, pf.Favorite); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "updated %d: %s\n", id, pf.Title)
			return nil
		},
	}
}

// writeEditBuffer dumps the resource (plus its tags + category) into a temp
// .md file using the same renderer the export command uses, so import/edit
// stay byte-compatible.
func writeEditBuffer(ctx context.Context, q *store.Queries, res store.Resource) (string, error) {
	tagRows, err := q.ListTagsForResource(ctx, res.ID)
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	tags := make([]string, 0, len(tagRows))
	for _, t := range tagRows {
		tags = append(tags, t.Name)
	}

	lr := store.ListedResource{Resource: res, Tags: tags}
	if res.CategoryID.Valid {
		c, err := q.GetCategory(ctx, res.CategoryID.Int64)
		if err == nil {
			lr.CategoryName = sql.NullString{String: c.Name, Valid: true}
			lr.CategorySlug = sql.NullString{String: c.Slug, Valid: true}
		}
	}

	tmp, err := os.CreateTemp("", fmt.Sprintf("arsenal-edit-%d-*.md", res.ID))
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	if _, err := tmp.WriteString(exportmd.RenderResource(lr)); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

// openInEditor invokes $EDITOR on path, falling back to vi. The editor
// runs interactively with shared stdio so the user actually sees their
// terminal.
func openInEditor(path string) error {
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}
	bin, args := splitEditor(editor)
	cmd := exec.Command(bin, append(args, path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// splitEditor handles the common case where $EDITOR contains arguments
// (e.g. "code --wait" or "vim -p"). Treats the first token as the binary
// and the rest as positional args, ignoring quoting nuances — sufficient
// for the editors people set in practice.
func splitEditor(s string) (string, []string) {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "vi", nil
	}
	return parts[0], parts[1:]
}

// keep this tiny helper local — exportmd.RenderResource is the symmetric
// piece, exposed below from exportmd via a thin wrapper.
var _ = filepath.Separator
