// Package legacy imports rows from an Arsenal v1 (Astro/SSR) SQLite database
// into the v2 schema. It runs once per machine and is invoked through the
// `arsenal migrate --from <path>` command.
//
// All writes go through internal/resources.Service so the same validation
// rules, transactional guarantees and FTS5 sync triggers that protect
// `arsenal add` also protect imported rows.
package legacy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/resources"
	"github.com/edwinupegui/arsenal/internal/store"

	_ "modernc.org/sqlite" // ensure driver is registered when this pkg is imported standalone
)

// Report describes what an import did (or, in dry-run mode, would have done).
// Counts are post-deduplication.
type Report struct {
	DryRun                bool
	CategoriesInserted    int
	CategoriesAlreadyKept int // already existed in the destination
	ResourcesImported     int
	ResourcesSkippedDup   int // URL already exists in destination
	TagsCreated           int
	Warnings              []string
}

// Options controls the import.
type Options struct {
	SourcePath string  // path to the legacy resources.db
	DryRun     bool    // if true, opens a tx and rolls it back
	Service    *resources.Service
	Queries    *store.Queries // for category writes / lookups outside the service
}

// Import opens the legacy DB read-only and copies its contents into the
// destination DB owned by opts.Service. Any failure rolls back all writes.
func Import(ctx context.Context, opts Options) (Report, error) {
	if opts.Service == nil {
		return Report{}, errors.New("legacy.Import: Service is required")
	}
	if opts.Queries == nil {
		return Report{}, errors.New("legacy.Import: Queries is required")
	}
	if strings.TrimSpace(opts.SourcePath) == "" {
		return Report{}, errors.New("legacy.Import: SourcePath is required")
	}

	src, err := openLegacyDB(opts.SourcePath)
	if err != nil {
		return Report{}, err
	}
	defer src.Close()

	if err := validateSchema(ctx, src); err != nil {
		return Report{}, fmt.Errorf("schema check: %w", err)
	}

	report := Report{DryRun: opts.DryRun}

	cats, err := readLegacyCategories(ctx, src)
	if err != nil {
		return Report{}, err
	}
	rows, err := readLegacyResources(ctx, src)
	if err != nil {
		return Report{}, err
	}

	// Sort categories by their original ID so sort_order in the new DB matches
	// the user's curated ordering.
	sort.Slice(cats, func(i, j int) bool { return cats[i].ID < cats[j].ID })

	// Map old categoryID -> new categoryID so resources relink correctly.
	catMap := make(map[int64]int64, len(cats))

	for _, c := range cats {
		newID, created, err := upsertCategory(ctx, opts.Queries, c)
		if err != nil {
			return report, fmt.Errorf("upsert category %q: %w", c.Name, err)
		}
		catMap[c.ID] = newID
		if created {
			report.CategoriesInserted++
		} else {
			report.CategoriesAlreadyKept++
		}
	}

	// Track distinct tag names we attach so the report shows tag growth.
	tagSeen := map[string]struct{}{}

	for _, r := range rows {
		if dup, err := opts.Queries.GetResourceByURL(ctx, r.URL); err == nil && dup.ID != 0 {
			report.ResourcesSkippedDup++
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("skip url already present: %s", r.URL))
			continue
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return report, fmt.Errorf("lookup url %q: %w", r.URL, err)
		}

		var catPtr *int64
		if newID, ok := catMap[r.CategoryID]; ok {
			catPtr = &newID
		}

		in := resources.ImportInput{
			CreateInput: resources.CreateInput{
				Title:       r.Title,
				URL:         r.URL,
				Description: r.Description,
				Type:        coerceType(r.Type),
				Language:    coerceLanguage(r.Language),
				CategoryID:  catPtr,
				Tags:        r.Tags,
			},
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.CreatedAt, // legacy schema has no updated_at
			DeletedAt: r.DeletedAt,
		}
		if _, err := opts.Service.Import(ctx, in); err != nil {
			return report, fmt.Errorf("import resource %q (%s): %w", r.Title, r.URL, err)
		}
		report.ResourcesImported++
		for _, t := range r.Tags {
			if norm, err := domain.NormalizeTag(t); err == nil {
				tagSeen[norm] = struct{}{}
			}
		}
	}
	report.TagsCreated = len(tagSeen)

	return report, nil
}

// openLegacyDB opens the legacy file in read-only mode so the migrate
// command can never accidentally mutate the source of truth.
func openLegacyDB(path string) (*sql.DB, error) {
	dsn := path + "?mode=ro&_pragma=query_only(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open legacy %s: %w", path, err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping legacy: %w", err)
	}
	return db, nil
}

// validateSchema fails fast if the file at SourcePath is not a recognizable
// Arsenal v1 database. Better to error on column "category_id" missing than
// to silently produce empty rows.
func validateSchema(ctx context.Context, db *sql.DB) error {
	required := map[string][]string{
		"categories": {"id", "name", "icon"},
		"resources":  {"id", "title", "url", "tags", "language", "type", "category_id", "created_at", "deleted_at"},
	}
	for table, cols := range required {
		rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
		if err != nil {
			return fmt.Errorf("inspect %s: %w", table, err)
		}
		have := map[string]bool{}
		for rows.Next() {
			var (
				cid     int
				name    string
				ctype   string
				notnull int
				dflt    sql.NullString
				pk      int
			)
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				rows.Close()
				return err
			}
			have[name] = true
		}
		rows.Close()
		if len(have) == 0 {
			return fmt.Errorf("table %s not found", table)
		}
		for _, c := range cols {
			if !have[c] {
				return fmt.Errorf("table %s missing column %s", table, c)
			}
		}
	}
	return nil
}

type legacyCategory struct {
	ID   int64
	Name string
	Icon string
}

func readLegacyCategories(ctx context.Context, db *sql.DB) ([]legacyCategory, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, icon FROM categories`)
	if err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}
	defer rows.Close()
	var out []legacyCategory
	for rows.Next() {
		var c legacyCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Icon); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type legacyResource struct {
	ID          int64
	Title       string
	URL         string
	Description string
	TagsJSON    sql.NullString
	Language    string
	Type        string
	CategoryID  int64
	CreatedAt   string
	DeletedAt   *string

	// Derived after read.
	Tags []string
}

func readLegacyResources(ctx context.Context, db *sql.DB) ([]legacyResource, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, url, COALESCE(description, ''), tags, language, type,
		       category_id, created_at, deleted_at
		FROM resources
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("read resources: %w", err)
	}
	defer rows.Close()

	var out []legacyResource
	for rows.Next() {
		var r legacyResource
		var deleted sql.NullString
		if err := rows.Scan(
			&r.ID, &r.Title, &r.URL, &r.Description, &r.TagsJSON,
			&r.Language, &r.Type, &r.CategoryID, &r.CreatedAt, &deleted,
		); err != nil {
			return nil, err
		}
		if deleted.Valid {
			s := deleted.String
			r.DeletedAt = &s
		}
		r.Tags = parseTagsJSON(r.TagsJSON)
		out = append(out, r)
	}
	return out, rows.Err()
}

// parseTagsJSON decodes the v1 ["a","b"] JSON column. Malformed values are
// treated as no tags; the legacy data is small enough to inspect manually
// if anything needs salvaging.
func parseTagsJSON(s sql.NullString) []string {
	if !s.Valid || strings.TrimSpace(s.String) == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(s.String), &arr); err != nil {
		return nil
	}
	return arr
}

// coerceType maps a legacy type string to the v2 closed enum, defaulting to
// `other` when the value is unknown.
func coerceType(t string) domain.ResourceType {
	rt := domain.ResourceType(strings.ToLower(strings.TrimSpace(t)))
	if rt.Valid() {
		return rt
	}
	return domain.TypeOther
}

// coerceLanguage maps a legacy language string to the v2 enum.
func coerceLanguage(l string) domain.Language {
	lang := domain.Language(strings.ToUpper(strings.TrimSpace(l)))
	if lang.Valid() {
		return lang
	}
	return domain.LangOther
}

// upsertCategory ensures a category with the same slug exists in the
// destination DB, returning its new ID and whether it was created in this run.
func upsertCategory(ctx context.Context, q *store.Queries, c legacyCategory) (int64, bool, error) {
	slug := slugify(c.Name)
	if existing, err := q.GetCategoryBySlug(ctx, slug); err == nil {
		return existing.ID, false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	created, err := q.CreateCategory(ctx, store.CreateCategoryParams{
		Slug:      slug,
		Name:      c.Name,
		Icon:      c.Icon,
		SortOrder: c.ID,
	})
	if err != nil {
		return 0, false, err
	}
	return created.ID, true, nil
}
