// Package scrape pulls Open Graph and HTML metadata for a URL so that
// `arsenal add <url>` can prefill the title, description, type and language
// without the user typing them by hand.
//
// The scraper is intentionally small: a single GET with a short timeout, a
// streaming parse via golang.org/x/net/html, and a few well-defined fallbacks.
// It is best-effort — failures map to a Metadata struct with whatever fields
// were resolved (often nothing) so the caller can still proceed.
package scrape

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/edwinupegui/arsenal/internal/domain"
)

// Metadata is the small subset of page metadata Arsenal cares about.
type Metadata struct {
	Title       string
	Description string
	Type        domain.ResourceType
	Language    domain.Language
	ImageURL    string
}

// FetchOptions tweaks the HTTP behavior. Zero value is safe and used by Fetch.
type FetchOptions struct {
	Client    *http.Client // nil → DefaultClient
	UserAgent string       // empty → "arsenal/scraper"
	Timeout   time.Duration
	MaxBytes  int64 // cap response body read (default 2 MiB)
}

// DefaultMaxBytes caps the body read at 2 MiB. Pages larger than this are
// truncated — by then we've almost certainly seen <head>.
const DefaultMaxBytes = 2 * 1024 * 1024

// Fetch retrieves rawURL and returns the parsed Metadata. The returned error
// is non-nil only on hard failures (transport error, non-2xx status, body
// read failure). A page with no OG tags simply yields an empty Metadata.
func Fetch(ctx context.Context, rawURL string, opts FetchOptions) (Metadata, error) {
	if err := domain.ValidateURL(rawURL); err != nil {
		return Metadata{}, err
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	max := opts.MaxBytes
	if max == 0 {
		max = DefaultMaxBytes
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = "arsenal/scraper"
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Metadata{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "es,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, fmt.Errorf("get %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Metadata{}, fmt.Errorf("http %d for %s", resp.StatusCode, rawURL)
	}

	body := io.LimitReader(resp.Body, max)
	meta, err := parseHTML(body)
	if err != nil && !errors.Is(err, io.EOF) {
		return meta, fmt.Errorf("parse html: %w", err)
	}

	// Fall back to host-based heuristics if og:type didn't yield a known type.
	if !meta.Type.Valid() {
		meta.Type = inferTypeFromHost(rawURL)
	}
	return meta, nil
}

// parseHTML walks the document looking for <title>, <meta name=description>,
// <meta property="og:*"> and the <html lang=""> attribute. It stops scanning
// once </head> closes — everything we care about lives there.
func parseHTML(r io.Reader) (Metadata, error) {
	z := html.NewTokenizer(r)
	var meta Metadata
	var seenHead bool

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if errors.Is(err, io.EOF) {
				return meta, nil
			}
			return meta, err

		case html.StartTagToken, html.SelfClosingTagToken:
			tag, hasAttr := z.TagName()
			name := string(tag)

			if name == "html" && hasAttr {
				meta.Language = pickLang(meta.Language, attrValue(z, "lang"))
			}
			if name == "head" {
				seenHead = true
			}
			if name == "title" {
				if z.Next() == html.TextToken {
					if t := strings.TrimSpace(string(z.Text())); t != "" && meta.Title == "" {
						meta.Title = t
					}
				}
			}
			if name == "meta" && hasAttr {
				readMeta(z, &meta)
			}

		case html.EndTagToken:
			tag, _ := z.TagName()
			if string(tag) == "head" && seenHead {
				return meta, nil
			}
		}
	}
}

// readMeta extracts whatever OG / standard meta values are interesting from
// the current <meta> tag. The tokenizer cursor must be on a meta start tag.
func readMeta(z *html.Tokenizer, meta *Metadata) {
	var key, content string
	for {
		k, v, more := z.TagAttr()
		switch strings.ToLower(string(k)) {
		case "property", "name":
			key = strings.ToLower(string(v))
		case "content":
			content = string(v)
		}
		if !more {
			break
		}
	}
	if content == "" {
		return
	}

	switch key {
	case "og:title":
		meta.Title = strings.TrimSpace(content)
	case "og:description":
		// og:description always wins — it's the canonical source.
		meta.Description = strings.TrimSpace(content)
	case "description", "twitter:description":
		if meta.Description == "" {
			meta.Description = strings.TrimSpace(content)
		}
	case "og:type":
		if t := mapOGType(content); t.Valid() {
			meta.Type = t
		}
	case "og:locale":
		meta.Language = pickLang(meta.Language, content)
	case "og:image", "twitter:image":
		if meta.ImageURL == "" {
			meta.ImageURL = content
		}
	}
}

// attrValue returns the value of the named attribute on the current tag.
// Returns "" if not present. Caller must ensure the tag has attributes.
func attrValue(z *html.Tokenizer, name string) string {
	for {
		k, v, more := z.TagAttr()
		if strings.EqualFold(string(k), name) {
			return string(v)
		}
		if !more {
			return ""
		}
	}
}

// mapOGType normalizes the og:type vocabulary to Arsenal's resource type
// enum. og:type "video.*" all map to TypeVideo. "article" → TypeArticle.
// "book" → TypeBook. "music.*" → TypeOther (no good match yet).
func mapOGType(s string) domain.ResourceType {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.HasPrefix(s, "video"):
		return domain.TypeVideo
	case s == "article":
		return domain.TypeArticle
	case s == "book":
		return domain.TypeBook
	case strings.HasPrefix(s, "podcast"):
		return domain.TypePodcast
	case s == "website" || s == "object":
		return ""
	}
	return ""
}

// pickLang folds a locale-ish string ("es_AR", "en", "pt-BR") onto the
// Arsenal language enum. Existing non-empty values are kept (first match wins).
func pickLang(current domain.Language, s string) domain.Language {
	if current.Valid() && current != domain.LangOther {
		return current
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return current
	}
	prefix := s
	if i := strings.IndexAny(s, "_-"); i > 0 {
		prefix = s[:i]
	}
	switch prefix {
	case "es":
		return domain.LangES
	case "en":
		return domain.LangEN
	case "pt":
		return domain.LangPT
	}
	return current
}

// inferTypeFromHost inspects the URL host and returns a sensible default type
// when the page didn't expose og:type.
func inferTypeFromHost(rawURL string) domain.ResourceType {
	u, err := url.Parse(rawURL)
	if err != nil {
		return domain.TypeOther
	}
	host := strings.ToLower(u.Host)
	switch {
	case hostMatch(host, "youtube.com", "youtu.be", "vimeo.com", "twitch.tv"):
		return domain.TypeVideo
	case hostMatch(host, "github.com", "gitlab.com", "bitbucket.org", "codeberg.org"):
		return domain.TypeRepo
	case hostMatch(host, "dev.to", "medium.com", "hashnode.dev", "substack.com"):
		return domain.TypeArticle
	case hostMatch(host, "npmjs.com", "pypi.org", "crates.io", "pkg.go.dev"):
		return domain.TypeTool
	case hostMatch(host, "spotify.com", "anchor.fm", "podcasts.apple.com"):
		return domain.TypePodcast
	}
	return domain.TypeOther
}

func hostMatch(host string, suffixes ...string) bool {
	for _, s := range suffixes {
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}
