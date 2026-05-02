// Package exportmd writes the Arsenal database out as a directory tree of
// markdown files (one per resource) with a YAML-ish frontmatter block. The
// format is hand-rolled to avoid pulling in a YAML dependency for a few
// well-known fields. The same package handles import: parse a directory of
// these files back into Service.Import calls.
package exportmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edwinupegui/arsenal/internal/store"
)

// Report summarizes an export run.
type Report struct {
	Resources int
	Trashed   int
	OutputDir string
}

// Options for ExportAll. The caller is responsible for an existing or
// creatable Dir; partial writes leave already-written files in place.
type Options struct {
	Dir            string // destination directory
	IncludeTrashed bool   // include soft-deleted rows under <Dir>/_trashed/
}

// ExportAll walks every active resource (and optionally the trash) and
// writes one markdown file per row. The directory layout is:
//
//	<Dir>/<category-slug>/<id>-<title-slug>.md   for active rows
//	<Dir>/uncategorized/<id>-<title-slug>.md     for category-less rows
//	<Dir>/_trashed/<id>-<title-slug>.md          when IncludeTrashed
//
// The directory is created if missing; existing files with the same path
// are overwritten so the export is idempotent against schema changes.
func ExportAll(ctx context.Context, db *sql.DB, q *store.Queries, opts Options) (Report, error) {
	if opts.Dir == "" {
		return Report{}, fmt.Errorf("exportmd: Dir is required")
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return Report{}, fmt.Errorf("exportmd: mkdir: %w", err)
	}

	report := Report{OutputDir: opts.Dir}

	// Persist the category catalog so import can recreate it on a fresh DB.
	if err := writeCategoriesIndex(ctx, q, opts.Dir); err != nil {
		return report, err
	}

	active, err := q.ListResourcesFiltered(ctx, store.ListFilter{Limit: 100000})
	if err != nil {
		return report, fmt.Errorf("exportmd: list: %w", err)
	}
	for _, lr := range active {
		if err := writeResource(opts.Dir, lr, false); err != nil {
			return report, err
		}
		report.Resources++
	}

	if opts.IncludeTrashed {
		trashed, err := q.ListResourcesFiltered(ctx, store.ListFilter{Trashed: true, Limit: 100000})
		if err != nil {
			return report, fmt.Errorf("exportmd: list trashed: %w", err)
		}
		for _, lr := range trashed {
			if err := writeResource(opts.Dir, lr, true); err != nil {
				return report, err
			}
			report.Trashed++
		}
	}
	return report, nil
}

// writeResource picks the destination path for a resource and writes its
// markdown file. fromTrash forces the row under _trashed/ regardless of
// category so the surviving directory tree mirrors active state cleanly.
func writeResource(root string, lr store.ListedResource, fromTrash bool) error {
	subdir := categorySubdir(lr, fromTrash)
	dir := filepath.Join(root, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("exportmd: mkdir %s: %w", dir, err)
	}

	name := fmt.Sprintf("%d-%s.md", lr.Resource.ID, slugify(lr.Resource.Title))
	path := filepath.Join(dir, name)

	body := renderResource(lr)
	// Use 0o644 — these files are user-owned content meant to live in git.
	return os.WriteFile(path, []byte(body), 0o644)
}

func categorySubdir(lr store.ListedResource, fromTrash bool) string {
	if fromTrash {
		return "_trashed"
	}
	if lr.CategorySlug.Valid && lr.CategorySlug.String != "" {
		return lr.CategorySlug.String
	}
	return "uncategorized"
}

// CategoryEntry mirrors the columns of the categories table that we round-
// trip through the markdown export. It's exported so the import side can
// share the same JSON shape.
type CategoryEntry struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder int64  `json:"sort_order"`
}

// CategoriesIndexFilename is the filename used inside <dir> for the
// category catalog dump. Underscore prefix keeps it ahead alphabetically.
const CategoriesIndexFilename = "_categories.json"

// writeCategoriesIndex dumps the full categories table as JSON next to the
// resource markdown tree so import can rebuild references on a fresh DB.
func writeCategoriesIndex(ctx context.Context, q *store.Queries, root string) error {
	cats, err := q.ListCategories(ctx)
	if err != nil {
		return fmt.Errorf("exportmd: list categories: %w", err)
	}
	out := make([]CategoryEntry, 0, len(cats))
	for _, c := range cats {
		out = append(out, CategoryEntry{
			Slug: c.Slug, Name: c.Name, Icon: c.Icon, SortOrder: c.SortOrder,
		})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, CategoriesIndexFilename), data, 0o644)
}

// renderResource builds the markdown payload: a `---`-fenced frontmatter
// block followed by a blank line and the description / notes as the body.
func renderResource(lr store.ListedResource) string {
	var b strings.Builder
	b.WriteString("---\n")
	writeFM(&b, "id", fmt.Sprintf("%d", lr.Resource.ID))
	writeFM(&b, "title", quote(lr.Resource.Title))
	writeFM(&b, "url", quote(lr.Resource.Url))
	writeFM(&b, "type", lr.Resource.Type)
	writeFM(&b, "language", lr.Resource.Language)
	if lr.CategorySlug.Valid {
		writeFM(&b, "category", lr.CategorySlug.String)
	}
	if len(lr.Tags) > 0 {
		writeFM(&b, "tags", renderArray(lr.Tags))
	} else {
		writeFM(&b, "tags", "[]")
	}
	if lr.Resource.Favorite == 1 {
		writeFM(&b, "favorite", "true")
	} else {
		writeFM(&b, "favorite", "false")
	}
	writeFM(&b, "created_at", lr.Resource.CreatedAt)
	writeFM(&b, "updated_at", lr.Resource.UpdatedAt)
	if lr.Resource.DeletedAt.Valid {
		writeFM(&b, "deleted_at", lr.Resource.DeletedAt.String)
	}
	b.WriteString("---\n\n")

	if lr.Resource.Description.Valid && lr.Resource.Description.String != "" {
		b.WriteString(strings.TrimRight(lr.Resource.Description.String, "\n"))
		b.WriteString("\n")
	}
	if lr.Resource.Notes.Valid && lr.Resource.Notes.String != "" {
		if lr.Resource.Description.Valid && lr.Resource.Description.String != "" {
			b.WriteString("\n")
		}
		b.WriteString("## Notes\n\n")
		b.WriteString(strings.TrimRight(lr.Resource.Notes.String, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

func writeFM(b *strings.Builder, key, value string) {
	fmt.Fprintf(b, "%s: %s\n", key, value)
}

// quote produces a YAML-safe scalar. We always quote with double quotes and
// escape backslashes / quotes / newlines so the import side parses it back
// even when the title contains colons or hashes.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// renderArray emits a flow-style array. Tags are ASCII-ish so this stays
// readable; any awkward chars get the same escaping as quote().
func renderArray(items []string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, quote(it))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
