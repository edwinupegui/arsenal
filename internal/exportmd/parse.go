package exportmd

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"
)

// ParsedFile is the in-memory shape of a single .md document.
type ParsedFile struct {
	Path        string
	Title       string
	URL         string
	Description string
	Notes       string
	Type        string
	Language    string
	Category    string // slug
	Tags        []string
	Favorite    bool
	CreatedAt   string
	UpdatedAt   string
	DeletedAt   string
}

// ParseFile reads one markdown file into a ParsedFile. The frontmatter must
// be the first block; the body (everything after `---\n`) is split by an
// optional `## Notes` header into description + notes.
func ParseFile(path string) (ParsedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return ParsedFile{}, err
	}
	defer f.Close()

	pf := ParsedFile{Path: path}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)

	if !scanner.Scan() {
		return pf, fmt.Errorf("%s: empty file", path)
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return pf, fmt.Errorf("%s: missing frontmatter (expected '---' on line 1)", path)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		applyFrontmatter(&pf, line)
	}

	var body, notes strings.Builder
	inNotes := false
	for scanner.Scan() {
		line := scanner.Text()
		if !inNotes && strings.TrimSpace(line) == "## Notes" {
			inNotes = true
			continue
		}
		if inNotes {
			notes.WriteString(line)
			notes.WriteByte('\n')
		} else {
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return pf, fmt.Errorf("%s: scan: %w", path, err)
	}

	pf.Description = strings.TrimSpace(body.String())
	pf.Notes = strings.TrimSpace(notes.String())
	return pf, nil
}

// applyFrontmatter parses one `key: value` line and stores it on pf.
// Unknown keys are ignored — keeps the parser tolerant of future fields.
func applyFrontmatter(pf *ParsedFile, line string) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return
	}
	key := strings.TrimSpace(line[:idx])
	rawVal := strings.TrimSpace(line[idx+1:])

	switch key {
	case "id":
		// id is informational only; we don't reuse it on import.
	case "title":
		pf.Title = unquote(rawVal)
	case "url":
		pf.URL = unquote(rawVal)
	case "type":
		pf.Type = strings.Trim(rawVal, "\"")
	case "language":
		pf.Language = strings.Trim(rawVal, "\"")
	case "category":
		pf.Category = strings.Trim(rawVal, "\"")
	case "tags":
		pf.Tags = parseArray(rawVal)
	case "favorite":
		pf.Favorite = strings.ToLower(rawVal) == "true"
	case "created_at":
		pf.CreatedAt = strings.Trim(rawVal, "\"")
	case "updated_at":
		pf.UpdatedAt = strings.Trim(rawVal, "\"")
	case "deleted_at":
		pf.DeletedAt = strings.Trim(rawVal, "\"")
	}
}

// unquote removes the wrapping double quotes added by exportmd's quote()
// and decodes the same escape set we emit (\\ \" \n \r \t).
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(s[i])
				b.WriteByte(s[i+1])
			}
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseArray decodes flow-style arrays like `["a", "b", "c"]`. Whitespace
// and trailing commas are tolerated.
func parseArray(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil
	}
	out := []string{}
	depth := 0
	cur := strings.Builder{}
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c == '"' {
			depth ^= 1
			cur.WriteByte(c)
			continue
		}
		if c == ',' && depth == 0 {
			out = append(out, unquote(strings.TrimSpace(cur.String())))
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if last := strings.TrimSpace(cur.String()); last != "" {
		out = append(out, unquote(last))
	}
	return out
}

// ImportReport summarizes an Import run.
type ImportReport struct {
	FilesScanned int
	Imported     int
	SkippedDup   int
	Failed       int
	Warnings     []string
}

// Import walks dir for *.md files and inserts each one through
// resources.Service.Import, preserving timestamps. Existing rows (matched
// by URL) are skipped so the operation is idempotent. Before scanning the
// markdown tree, _categories.json (if present) is replayed so resources
// can attach to the right category on a fresh DB.
func Import(ctx context.Context, dir string, svc *resources.Service, q *store.Queries) (ImportReport, error) {
	rep := ImportReport{}

	if err := importCategoriesIndex(ctx, q, dir, &rep); err != nil {
		return rep, err
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rep.FilesScanned++

		pf, err := ParseFile(path)
		if err != nil {
			rep.Failed++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("parse %s: %v", path, err))
			return nil
		}
		if pf.URL == "" || pf.Title == "" {
			rep.Failed++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("skip %s: missing title or url", path))
			return nil
		}

		if dup, gerr := q.GetResourceByURL(ctx, pf.URL); gerr == nil && dup.ID != 0 {
			rep.SkippedDup++
			return nil
		} else if gerr != nil && !errors.Is(gerr, sql.ErrNoRows) {
			rep.Failed++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("lookup %s: %v", path, gerr))
			return nil
		}

		var catID *int64
		if pf.Category != "" {
			cat, gerr := q.GetCategoryBySlug(ctx, pf.Category)
			if gerr == nil {
				catID = &cat.ID
			}
		}

		typ := domain.ResourceType(strings.ToLower(pf.Type))
		if !typ.Valid() {
			typ = domain.TypeOther
		}
		lang := domain.Language(strings.ToUpper(pf.Language))
		if !lang.Valid() {
			lang = domain.LangOther
		}
		created := pf.CreatedAt
		updated := pf.UpdatedAt
		if updated == "" {
			updated = created
		}
		var deleted *string
		if pf.DeletedAt != "" {
			s := pf.DeletedAt
			deleted = &s
		}

		in := resources.ImportInput{
			CreateInput: resources.CreateInput{
				Title:       pf.Title,
				URL:         pf.URL,
				Description: pf.Description,
				Type:        typ,
				Language:    lang,
				CategoryID:  catID,
				Notes:       pf.Notes,
				Favorite:    pf.Favorite,
				Tags:        pf.Tags,
			},
			CreatedAt: orNow(created),
			UpdatedAt: orNow(updated),
			DeletedAt: deleted,
		}
		if _, err := svc.Import(ctx, in); err != nil {
			rep.Failed++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("import %s: %v", path, err))
			return nil
		}
		rep.Imported++
		return nil
	})
	if err != nil {
		return rep, err
	}
	return rep, nil
}

// orNow falls back to a current timestamp when frontmatter omitted it.
// Service.Import requires created_at to be non-empty.
func orNow(s string) string {
	if strings.TrimSpace(s) != "" {
		return s
	}
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// importCategoriesIndex consumes <dir>/_categories.json (when present) and
// upserts each entry into the destination DB so resources arriving from
// markdown frontmatter can find their category by slug. Missing file is
// not an error: older exports won't have it.
func importCategoriesIndex(ctx context.Context, q *store.Queries, dir string, rep *ImportReport) error {
	path := filepath.Join(dir, CategoriesIndexFilename)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("read %s: %v", path, err))
		return nil
	}
	var cats []CategoryEntry
	if err := json.Unmarshal(data, &cats); err != nil {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("parse %s: %v", path, err))
		return nil
	}
	for _, c := range cats {
		if c.Slug == "" {
			continue
		}
		if _, gerr := q.GetCategoryBySlug(ctx, c.Slug); gerr == nil {
			continue
		} else if !errors.Is(gerr, sql.ErrNoRows) {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("lookup category %q: %v", c.Slug, gerr))
			continue
		}
		if _, err := q.CreateCategory(ctx, store.CreateCategoryParams{
			Slug: c.Slug, Name: c.Name, Icon: c.Icon, SortOrder: c.SortOrder,
		}); err != nil {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("create category %q: %v", c.Slug, err))
		}
	}
	return nil
}
