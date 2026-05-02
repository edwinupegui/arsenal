package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/store"
)

// newListCmd builds `arsenal list` with optional filters.
func newListCmd() *cobra.Command {
	var (
		flagCat     string
		flagTag     string
		flagType    string
		flagLang    string
		flagFav     bool
		flagTrashed bool
		flagLimit   int
		flagOffset  int
		flagJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List resources, optionally filtered by category, tag, type, language or favorite",
		Example: `  arsenal list
  arsenal list --type video --lang ES
  arsenal list --cat arquitectura-clean-code-ops --fav
  arsenal list --tag patrones --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			items, err := app.Queries.ListResourcesFiltered(cmd.Context(), store.ListFilter{
				CategorySlug: flagCat,
				TagName:      flagTag,
				Type:         flagType,
				Language:     flagLang,
				OnlyFavorite: flagFav,
				Trashed:      flagTrashed,
				Limit:        flagLimit,
				Offset:       flagOffset,
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
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug (e.g. arquitectura-clean-code-ops)")
	cmd.Flags().StringVar(&flagTag, "tag", "", "tag name (case-insensitive)")
	cmd.Flags().StringVar(&flagType, "type", "", "resource type (video, article, tool, repo, course, podcast, newsletter, community, book, other)")
	cmd.Flags().StringVar(&flagLang, "lang", "", "language (ES, EN, PT, OTHER)")
	cmd.Flags().BoolVar(&flagFav, "fav", false, "only favorites")
	cmd.Flags().BoolVar(&flagTrashed, "trashed", false, "only trashed (soft-deleted) resources")
	cmd.Flags().IntVarP(&flagLimit, "limit", "n", 50, "max rows to return")
	cmd.Flags().IntVar(&flagOffset, "offset", 0, "rows to skip (for pagination)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON instead of a table")
	return cmd
}

func writeTable(out io.Writer, items []store.ListedResource) {
	if len(items) == 0 {
		fmt.Fprintln(out, "no resources match the filter")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tFAV\tTYPE\tLANG\tTITLE\tCATEGORY\tTAGS")
	for _, r := range items {
		fav := " "
		if r.Resource.Favorite == 1 {
			fav = "*"
		}
		cat := "-"
		if r.CategorySlug.Valid {
			cat = r.CategorySlug.String
		}
		tags := strings.Join(r.Tags, ",")
		if tags == "" {
			tags = "-"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Resource.ID, fav, r.Resource.Type, r.Resource.Language,
			truncate(r.Resource.Title, 60), cat, tags)
	}
	_ = w.Flush()
}

func writeJSON(out io.Writer, items []store.ListedResource) error {
	type tagged struct {
		ID           int64   `json:"id"`
		Title        string  `json:"title"`
		URL          string  `json:"url"`
		Description  *string `json:"description,omitempty"`
		Type         string  `json:"type"`
		Language     string  `json:"language"`
		CategorySlug *string `json:"category_slug,omitempty"`
		CategoryName *string `json:"category_name,omitempty"`
		Notes        *string `json:"notes,omitempty"`
		Favorite     bool    `json:"favorite"`
		CreatedAt    string  `json:"created_at"`
		UpdatedAt    string  `json:"updated_at"`
		DeletedAt    *string `json:"deleted_at,omitempty"`
		Tags         []string `json:"tags"`
	}

	mapped := make([]tagged, 0, len(items))
	for _, r := range items {
		t := tagged{
			ID:        r.Resource.ID,
			Title:     r.Resource.Title,
			URL:       r.Resource.Url,
			Type:      r.Resource.Type,
			Language:  r.Resource.Language,
			Favorite:  r.Resource.Favorite == 1,
			CreatedAt: r.Resource.CreatedAt,
			UpdatedAt: r.Resource.UpdatedAt,
			Tags:      r.Tags,
		}
		if r.Resource.Description.Valid {
			s := r.Resource.Description.String
			t.Description = &s
		}
		if r.Resource.Notes.Valid {
			s := r.Resource.Notes.String
			t.Notes = &s
		}
		if r.CategorySlug.Valid {
			s := r.CategorySlug.String
			t.CategorySlug = &s
		}
		if r.CategoryName.Valid {
			s := r.CategoryName.String
			t.CategoryName = &s
		}
		if r.Resource.DeletedAt.Valid {
			s := r.Resource.DeletedAt.String
			t.DeletedAt = &s
		}
		mapped = append(mapped, t)
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(mapped)
}

// truncate trims s to at most n runes, appending ellipsis when truncated.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
