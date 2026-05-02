package cli

import (
	"context"
	"database/sql"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// newStatsCmd builds `arsenal stats` — a one-glance summary of the
// database. Run it before/after a curation session to see the shape of
// the collection at a glance.
func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show counts and a small dashboard about the database",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			s, err := collectStats(cmd.Context(), app.DB)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Arsenal database — %s\n", app.Paths.DB)
			fmt.Fprintf(out, "  Active resources: %d\n", s.activeCount)
			fmt.Fprintf(out, "  Trashed:          %d\n", s.trashedCount)
			fmt.Fprintf(out, "  Categories:       %d\n", s.categoryCount)
			fmt.Fprintf(out, "  Tags:             %d\n", s.tagCount)
			fmt.Fprintf(out, "  Favorites:        %d\n", s.favoriteCount)
			fmt.Fprintln(out)

			fmt.Fprintln(out, "By type")
			writeKVTable(out, s.byType)
			fmt.Fprintln(out)

			fmt.Fprintln(out, "By language")
			writeKVTable(out, s.byLanguage)
			fmt.Fprintln(out)

			if len(s.topTags) > 0 {
				fmt.Fprintln(out, "Top tags")
				writeKVTable(out, s.topTags)
				fmt.Fprintln(out)
			}

			if len(s.recent) > 0 {
				fmt.Fprintln(out, "Recent activity")
				w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
				for _, r := range s.recent {
					fmt.Fprintf(w, "  %d\t%s\n", r.id, r.title)
				}
				_ = w.Flush()
			}
			return nil
		},
	}
}

// statsAggregate groups every metric the stats command surfaces. Filling
// it in one pass keeps the SQL roundtrips visible and makes the renderer
// trivial.
type statsAggregate struct {
	activeCount   int64
	trashedCount  int64
	categoryCount int64
	tagCount      int64
	favoriteCount int64

	byType     []kv
	byLanguage []kv
	topTags    []kv

	recent []recentItem
}

type kv struct {
	k string
	v int64
}

type recentItem struct {
	id    int64
	title string
}

// collectStats runs the small handful of aggregations the dashboard needs.
// Each one is short and intentional — not worth abstracting beyond inline.
func collectStats(ctx context.Context, db *sql.DB) (statsAggregate, error) {
	var s statsAggregate

	scalar := func(stmt string) int64 {
		var n int64
		_ = db.QueryRowContext(ctx, stmt).Scan(&n)
		return n
	}
	s.activeCount = scalar(`SELECT COUNT(*) FROM resources WHERE deleted_at IS NULL`)
	s.trashedCount = scalar(`SELECT COUNT(*) FROM resources WHERE deleted_at IS NOT NULL`)
	s.categoryCount = scalar(`SELECT COUNT(*) FROM categories`)
	s.tagCount = scalar(`SELECT COUNT(*) FROM tags`)
	s.favoriteCount = scalar(`SELECT COUNT(*) FROM resources WHERE deleted_at IS NULL AND favorite = 1`)

	rowsKV := func(stmt string, args ...any) ([]kv, error) {
		rows, err := db.QueryContext(ctx, stmt, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []kv
		for rows.Next() {
			var k string
			var v int64
			if err := rows.Scan(&k, &v); err != nil {
				return nil, err
			}
			out = append(out, kv{k: k, v: v})
		}
		return out, rows.Err()
	}

	var err error
	s.byType, err = rowsKV(`
		SELECT type, COUNT(*) FROM resources
		WHERE deleted_at IS NULL
		GROUP BY type ORDER BY COUNT(*) DESC, type ASC`)
	if err != nil {
		return s, err
	}
	s.byLanguage, err = rowsKV(`
		SELECT language, COUNT(*) FROM resources
		WHERE deleted_at IS NULL
		GROUP BY language ORDER BY COUNT(*) DESC, language ASC`)
	if err != nil {
		return s, err
	}
	s.topTags, err = rowsKV(`
		SELECT t.name, COUNT(rt.resource_id)
		FROM tags t
		LEFT JOIN resource_tags rt ON rt.tag_id = t.id
		LEFT JOIN resources r       ON r.id = rt.resource_id AND r.deleted_at IS NULL
		GROUP BY t.id
		HAVING COUNT(rt.resource_id) > 0
		ORDER BY COUNT(rt.resource_id) DESC, t.name ASC
		LIMIT 10`)
	if err != nil {
		return s, err
	}

	rRows, err := db.QueryContext(ctx, `
		SELECT id, title FROM resources
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 5`)
	if err != nil {
		return s, err
	}
	defer rRows.Close()
	for rRows.Next() {
		var r recentItem
		if err := rRows.Scan(&r.id, &r.title); err != nil {
			return s, err
		}
		s.recent = append(s.recent, r)
	}
	return s, rRows.Err()
}

// writeKVTable prints a 2-column "key value" table aligned via tabwriter.
func writeKVTable(out interface{ Write([]byte) (int, error) }, rows []kv) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		fmt.Fprintf(w, "  %s\t%d\n", r.k, r.v)
	}
	_ = w.Flush()
}
