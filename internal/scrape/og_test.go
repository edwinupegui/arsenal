package scrape_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/edwinupegui/arsenal/internal/domain"
	"github.com/edwinupegui/arsenal/internal/scrape"
)

func newServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, body)
	}))
}

func TestFetch_OGTags(t *testing.T) {
	srv := newServer(`<!DOCTYPE html>
<html lang="es-AR">
<head>
  <title>Page Title</title>
  <meta name="description" content="standard description">
  <meta property="og:title" content="OG Title">
  <meta property="og:description" content="OG description">
  <meta property="og:type" content="article">
  <meta property="og:locale" content="es_AR">
</head>
<body>ignored</body>
</html>`)
	defer srv.Close()

	got, err := scrape.Fetch(context.Background(), srv.URL, scrape.FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.Title != "OG Title" {
		t.Errorf("Title = %q, want %q", got.Title, "OG Title")
	}
	if got.Description != "OG description" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Type != domain.TypeArticle {
		t.Errorf("Type = %q, want article", got.Type)
	}
	if got.Language != domain.LangES {
		t.Errorf("Language = %q, want ES", got.Language)
	}
}

func TestFetch_OnlyTitleTag(t *testing.T) {
	// No OG metadata; should fall back to <title> and host-based type guess.
	srv := newServer(`<html><head><title>Plain Page</title></head><body></body></html>`)
	defer srv.Close()

	got, err := scrape.Fetch(context.Background(), srv.URL, scrape.FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.Title != "Plain Page" {
		t.Errorf("Title = %q", got.Title)
	}
	// httptest URL hostname is 127.0.0.1; doesn't match any known host so
	// the inferred type is TypeOther.
	if got.Type != domain.TypeOther {
		t.Errorf("Type = %q, want other", got.Type)
	}
}

func TestFetch_OGVideoType(t *testing.T) {
	srv := newServer(`<html><head>
		<meta property="og:title" content="Clip">
		<meta property="og:type" content="video.other">
	</head></html>`)
	defer srv.Close()

	got, err := scrape.Fetch(context.Background(), srv.URL, scrape.FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.Type != domain.TypeVideo {
		t.Errorf("Type = %q, want video", got.Type)
	}
}

func TestFetch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := scrape.Fetch(context.Background(), srv.URL, scrape.FetchOptions{})
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestFetch_BadURL(t *testing.T) {
	_, err := scrape.Fetch(context.Background(), "not a url", scrape.FetchOptions{})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}
