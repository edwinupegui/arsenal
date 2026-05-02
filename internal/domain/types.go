// Package domain holds pure types and validators shared across the cli, store
// and service layers. No I/O, no DB, no external deps beyond stdlib.
package domain

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// ResourceType is the closed enum of supported resource categories.
// Must stay in sync with the CHECK constraint in migrations.
type ResourceType string

const (
	TypeVideo      ResourceType = "video"
	TypeArticle    ResourceType = "article"
	TypeTool       ResourceType = "tool"
	TypeRepo       ResourceType = "repo"
	TypeCourse     ResourceType = "course"
	TypePodcast    ResourceType = "podcast"
	TypeNewsletter ResourceType = "newsletter"
	TypeCommunity  ResourceType = "community"
	TypeBook       ResourceType = "book"
	TypeOther      ResourceType = "other"
)

// AllResourceTypes returns every valid type in declaration order. Useful for
// help text and TUI selectors.
func AllResourceTypes() []ResourceType {
	return []ResourceType{
		TypeVideo, TypeArticle, TypeTool, TypeRepo, TypeCourse,
		TypePodcast, TypeNewsletter, TypeCommunity, TypeBook, TypeOther,
	}
}

// Valid reports whether t is one of the known types.
func (t ResourceType) Valid() bool {
	switch t {
	case TypeVideo, TypeArticle, TypeTool, TypeRepo, TypeCourse,
		TypePodcast, TypeNewsletter, TypeCommunity, TypeBook, TypeOther:
		return true
	}
	return false
}

// String implements fmt.Stringer.
func (t ResourceType) String() string { return string(t) }

// Language is the closed enum of supported languages.
type Language string

const (
	LangES    Language = "ES"
	LangEN    Language = "EN"
	LangPT    Language = "PT"
	LangOther Language = "OTHER"
)

// AllLanguages returns every valid language in declaration order.
func AllLanguages() []Language {
	return []Language{LangES, LangEN, LangPT, LangOther}
}

// Valid reports whether l is one of the known languages.
func (l Language) Valid() bool {
	switch l {
	case LangES, LangEN, LangPT, LangOther:
		return true
	}
	return false
}

// String implements fmt.Stringer.
func (l Language) String() string { return string(l) }

// MaxTagLength caps the on-wire length of a single normalized tag. Tags longer
// than this are rejected — they are almost always either typos or someone
// trying to stuff a sentence into a label.
const MaxTagLength = 40

// ErrEmptyTag is returned when normalization collapses a tag to the empty string.
var ErrEmptyTag = errors.New("tag is empty after normalization")

// NormalizeTag lowercases, trims, and collapses internal whitespace. The
// resulting string is suitable for storage and case-insensitive comparison.
// Returns ErrEmptyTag for tags that survive only as whitespace.
func NormalizeTag(raw string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(raw))
	t = strings.Join(strings.Fields(t), " ")
	if t == "" {
		return "", ErrEmptyTag
	}
	if len(t) > MaxTagLength {
		return "", fmt.Errorf("tag %q exceeds %d chars", t, MaxTagLength)
	}
	return t, nil
}

// NormalizeTags applies NormalizeTag to each entry, drops empties silently,
// deduplicates (case-insensitive), and returns a stable sorted slice.
// Tags exceeding MaxTagLength are reported as errors with their raw value.
func NormalizeTags(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		norm, err := NormalizeTag(t)
		if err != nil {
			if errors.Is(err, ErrEmptyTag) {
				continue // empty tags are dropped, not an error
			}
			return nil, err
		}
		if _, dup := seen[norm]; dup {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	sort.Strings(out)
	return out, nil
}

// ValidateURL ensures the string parses as an absolute http/https URL.
// Other schemes (file:, ftp:, mailto:) are rejected — Arsenal targets web
// resources, and a stricter front door catches typos early.
func ValidateURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("url is missing host")
	}
	return nil
}

// ValidateTitle ensures the title is non-empty after trimming and within a
// sensible upper bound to prevent pathological storage.
const MaxTitleLength = 500

// ValidateTitle returns nil when title is non-empty after trim and below MaxTitleLength.
func ValidateTitle(title string) error {
	t := strings.TrimSpace(title)
	if t == "" {
		return errors.New("title is required")
	}
	if len(t) > MaxTitleLength {
		return fmt.Errorf("title exceeds %d chars", MaxTitleLength)
	}
	return nil
}
