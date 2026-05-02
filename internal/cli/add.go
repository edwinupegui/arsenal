package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/scrape"
	"github.com/edwinupegui/arsenal/internal/store"
)

// newAddCmd builds `arsenal add <url>` with optional metadata flags.
// Default behavior: scrape OG metadata for title/description/type/lang and
// allow individual fields to be overridden by flags. --no-scrape skips the
// network call entirely.
func newAddCmd() *cobra.Command {
	var (
		flagTitle    string
		flagDesc     string
		flagType     string
		flagLang     string
		flagCat      string // category slug
		flagTags     []string
		flagNotes    string
		flagFav      bool
		flagNoScrape bool
	)
	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add a new resource. Scrapes Open Graph metadata by default.",
		Long: `Add fetches the URL, extracts Open Graph metadata (title, description,
type, language) and inserts a new resource. Each metadata field can be
overridden with the corresponding flag; --no-scrape skips the fetch and
relies on flag values entirely.`,
		Example: `  arsenal add https://example.com/post
  arsenal add https://github.com/user/repo --type repo --tag golang --tag tooling
  arsenal add https://localhost/x --no-scrape --title "Local thing" --type other`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawURL := args[0]
			if err := domain.ValidateURL(rawURL); err != nil {
				return err
			}

			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			meta := scrape.Metadata{}
			if !flagNoScrape {
				m, ferr := scrape.Fetch(cmd.Context(), rawURL, scrape.FetchOptions{})
				if ferr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: scrape failed (%v); using defaults\n", ferr)
				} else {
					meta = m
				}
			}

			title := firstNonEmpty(flagTitle, meta.Title, rawURL)
			desc := firstNonEmpty(flagDesc, meta.Description)

			rtype := domain.ResourceType(strings.ToLower(strings.TrimSpace(flagType)))
			if !rtype.Valid() {
				if meta.Type.Valid() {
					rtype = meta.Type
				} else {
					rtype = domain.TypeOther
				}
			}

			lang := domain.Language(strings.ToUpper(strings.TrimSpace(flagLang)))
			if !lang.Valid() {
				if meta.Language.Valid() {
					lang = meta.Language
				} else {
					lang = domain.LangOther
				}
			}

			catID, err := resolveCategoryID(cmd.Context(), app.Queries, flagCat)
			if err != nil {
				return err
			}

			created, err := resources.New(app.DB).Create(cmd.Context(), resources.CreateInput{
				Title:       title,
				URL:         rawURL,
				Description: desc,
				Type:        rtype,
				Language:    lang,
				CategoryID:  catID,
				Notes:       flagNotes,
				Favorite:    flagFav,
				Tags:        flagTags,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "added %d: %s\n", created.Row.ID, created.Row.Title)
			fmt.Fprintf(out, "  url:      %s\n", created.Row.Url)
			fmt.Fprintf(out, "  type:     %s\n", created.Row.Type)
			fmt.Fprintf(out, "  language: %s\n", created.Row.Language)
			if catID != nil {
				fmt.Fprintf(out, "  category: %s\n", flagCat)
			}
			if len(created.Tags) > 0 {
				fmt.Fprintf(out, "  tags:     %s\n", strings.Join(created.Tags, ", "))
			}
			if created.Row.Favorite == 1 {
				fmt.Fprintln(out, "  favorite: yes")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTitle, "title", "", "title (overrides scraped value)")
	cmd.Flags().StringVar(&flagDesc, "desc", "", "description (overrides scraped value)")
	cmd.Flags().StringVar(&flagType, "type", "", "type (overrides scraped value)")
	cmd.Flags().StringVar(&flagLang, "lang", "", "language ES|EN|PT|OTHER (overrides scraped value)")
	cmd.Flags().StringVar(&flagCat, "cat", "", "category slug")
	cmd.Flags().StringSliceVar(&flagTags, "tag", nil, "tag (repeat for multiple, e.g. --tag a --tag b)")
	cmd.Flags().StringVar(&flagNotes, "notes", "", "free-form notes (markdown)")
	cmd.Flags().BoolVar(&flagFav, "fav", false, "mark as favorite")
	cmd.Flags().BoolVar(&flagNoScrape, "no-scrape", false, "skip the OG fetch and use flags only")
	return cmd
}

// resolveCategoryID maps a slug flag value to the corresponding category.id.
// Empty slug returns nil (uncategorized). Unknown slug fails with a clear
// error so the user can spot a typo before the row lands.
func resolveCategoryID(ctx context.Context, q *store.Queries, slug string) (*int64, error) {
	if slug == "" {
		return nil, nil
	}
	cat, err := q.GetCategoryBySlug(ctx, strings.ToLower(strings.TrimSpace(slug)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("category %q not found (use `arsenal list` to see available slugs)", slug)
	}
	if err != nil {
		return nil, err
	}
	return &cat.ID, nil
}

// firstNonEmpty returns the first non-empty string from candidates, or "" if all are empty.
func firstNonEmpty(candidates ...string) string {
	for _, c := range candidates {
		if strings.TrimSpace(c) != "" {
			return c
		}
	}
	return ""
}
