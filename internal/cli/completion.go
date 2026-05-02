package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/store"
)

// wireCompletions attaches dynamic shell-completion handlers to subcommands
// after they've been added to the root tree. Doing it here, rather than in
// each newXCmd, keeps the per-command files focused on logic and makes it
// easy to see the full completion surface in one place.
//
// All completion handlers are best-effort: if the database can't be opened
// (e.g. completion runs before `arsenal init`) they return zero suggestions
// instead of crashing the shell.
func wireCompletions(root *cobra.Command) {
	// Subcommands that take a resource id as the first positional arg.
	for _, name := range []string{"show", "rm", "restore", "purge", "star", "unstar", "edit"} {
		if c, _, err := root.Find([]string{name}); err == nil {
			c.ValidArgsFunction = completeResourceIDs
		}
	}

	// `arsenal list` filters and `arsenal add` overrides share the same
	// flag set vocabulary; register them in one pass.
	for _, name := range []string{"list", "add"} {
		c, _, err := root.Find([]string{name})
		if err != nil {
			continue
		}
		_ = c.RegisterFlagCompletionFunc("cat", completeCategorySlugs)
		_ = c.RegisterFlagCompletionFunc("tag", completeTagNames)
		_ = c.RegisterFlagCompletionFunc("type", completeResourceTypes)
		_ = c.RegisterFlagCompletionFunc("lang", completeLanguages)
	}

	if c, _, err := root.Find([]string{"cat", "rm"}); err == nil {
		c.ValidArgsFunction = completeCategorySlugs
	}
	for _, sub := range []string{"rename", "merge"} {
		if c, _, err := root.Find([]string{"tag", sub}); err == nil {
			c.ValidArgsFunction = completeTagNames
		}
	}
}

// completeResourceIDs returns recent resource IDs paired with title preview.
// Cobra's display format is "value\tdescription" — the description shows
// next to the suggestion in zsh/fish.
func completeResourceIDs(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	app, err := initApp(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer app.DB.Close()

	items, err := app.Queries.ListResourcesFiltered(cmd.Context(), store.ListFilter{Limit: 200})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(items))
	for _, r := range items {
		out = append(out, fmt.Sprintf("%d\t%s", r.Resource.ID, truncateForCompletion(r.Resource.Title, 50)))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeCategorySlugs returns every category slug, with the human-readable
// name as the description.
func completeCategorySlugs(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	app, err := initApp(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer app.DB.Close()
	cats, err := app.Queries.ListCategories(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		out = append(out, fmt.Sprintf("%s\t%s", c.Slug, c.Name))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeTagNames returns every tag with usage count as description.
func completeTagNames(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	app, err := initApp(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer app.DB.Close()
	tags, err := app.Queries.ListTags(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, fmt.Sprintf("%s\t%d resources", t.Name, t.ResourceCount))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeResourceTypes is purely static — the closed enum from domain.
func completeResourceTypes(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := domain.AllResourceTypes()
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, string(t))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeLanguages is the static language enum.
func completeLanguages(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	langs := domain.AllLanguages()
	out := make([]string, 0, len(langs))
	for _, l := range langs {
		out = append(out, string(l))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// truncateForCompletion shrinks long titles so the description column in
// the completion popup stays readable. Local copy of cli.truncate
// behavior — kept here to avoid coupling completion to the table renderer.
func truncateForCompletion(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return strings.ReplaceAll(s, "\t", " ")
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
