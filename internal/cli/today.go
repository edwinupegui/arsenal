package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/today"
	"github.com/edwinupegui/arsenal/internal/today/providers"
)

// newTodayCmd builds `arsenal today` which renders the cross-domain Today view
// (overdue / due-today / upcoming todos + recent resources) as a CLI surface.
// Mirrors the TUI areaToday and web /today; thin wrapper over today.Service.
func newTodayCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Show the Today cross-domain view (overdue, due-today, upcoming, recent)",
		Long: "Aggregates overdue and upcoming todos with recent resources into a single " +
			"dashboard. Mirrors the TUI areaToday and the web /today route. Use --json for " +
			"machine-readable output suitable for piping into other tools.",
		Example: `  arsenal today
  arsenal today --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := initApp(cmd.Context())
			if err != nil {
				return err
			}
			defer app.DB.Close()

			svc := today.New(app.DB)
			svc.Register(providers.NewTodosProvider(app.DB))
			svc.Register(providers.NewResourcesProvider(app.DB))

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			sections, providerErrs := svc.Build(ctx)
			for _, pe := range providerErrs {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: provider %q: %v\n", pe.Name, pe.Err)
			}

			if flagJSON {
				return writeTodayJSON(cmd.OutOrStdout(), sections)
			}
			writeTodayTable(cmd.OutOrStdout(), sections)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "output as JSON")
	return cmd
}

// writeTodayTable renders sections as a tab-aligned table. When a section
// has a ShowAllURL (more than 5 items in the source), a "show all" footer
// points the user to the web route that lists the full set.
func writeTodayTable(out io.Writer, sections []today.Section) {
	if len(sections) == 0 {
		fmt.Fprintln(out, "(nothing today)")
		return
	}
	for _, sec := range sections {
		// Section header
		header := sec.Title
		if sec.ShowAllURL != "" {
			header = fmt.Sprintf("%s  (showing %d, show all → %s)", sec.Title, len(sec.Items), sec.ShowAllURL)
		} else {
			header = fmt.Sprintf("%s  (%d)", sec.Title, len(sec.Items))
		}
		fmt.Fprintln(out, header)
		fmt.Fprintln(out, repeat("─", len(header)))

		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		for _, it := range sec.Items {
			tags := ""
			if len(it.Tags) > 0 {
				tags = "  #" + joinTags(it.Tags)
			}
			prio := ""
			if it.Priority != "" {
				prio = " [" + it.Priority + "]"
			}
			fmt.Fprintf(w, "  [%s]%s\t%s\t%s%s\n",
				it.Domain, prio, truncate(it.Title, 60), it.Subtitle, tags)
		}
		_ = w.Flush()
		fmt.Fprintln(out)
	}
}

func joinTags(tags []string) string {
	out := ""
	for i, tag := range tags {
		if i > 0 {
			out += " #"
		}
		out += tag
	}
	return out
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

// writeTodayJSON serializes sections as a JSON array suitable for piping.
// Each section includes its key, title, items, and show_all_url (empty when
// not overflowing). Items include domain, id, title, subtitle, priority, tags.
func writeTodayJSON(out io.Writer, sections []today.Section) error {
	type itemJSON struct {
		Domain   string   `json:"domain"`
		ID       int64    `json:"id"`
		Title    string   `json:"title"`
		Subtitle string   `json:"subtitle,omitempty"`
		Priority string   `json:"priority,omitempty"`
		Tags     []string `json:"tags,omitempty"`
		URL      string   `json:"url,omitempty"`
	}
	type sectionJSON struct {
		Key        string     `json:"key"`
		Title      string     `json:"title"`
		Items      []itemJSON `json:"items"`
		ShowAllURL string     `json:"show_all_url,omitempty"`
	}

	mapped := make([]sectionJSON, 0, len(sections))
	for _, sec := range sections {
		items := make([]itemJSON, 0, len(sec.Items))
		for _, it := range sec.Items {
			items = append(items, itemJSON{
				Domain:   it.Domain,
				ID:       it.ID,
				Title:    it.Title,
				Subtitle: it.Subtitle,
				Priority: it.Priority,
				Tags:     it.Tags,
				URL:      it.URL,
			})
		}
		mapped = append(mapped, sectionJSON{
			Key:        sec.Key,
			Title:      sec.Title,
			Items:      items,
			ShowAllURL: sec.ShowAllURL,
		})
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(mapped)
}
