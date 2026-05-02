package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/edwinupegui/arsenal/internal/domain"
)

func TestNormalizeTag(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr error
	}{
		{"  Architecture  ", "architecture", nil},
		{"Clean  Code", "clean code", nil},                          // collapses inner whitespace
		{"PATTERNS", "patterns", nil},                               // lowercases
		{"   ", "", domain.ErrEmptyTag},                              // pure whitespace
		{"", "", domain.ErrEmptyTag},                                 // empty
		{"\t\n", "", domain.ErrEmptyTag},                             // weird whitespace
		{"go", "go", nil},                                            // short ok
		{strings.Repeat("a", domain.MaxTagLength), strings.Repeat("a", domain.MaxTagLength), nil},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := domain.NormalizeTag(c.in)
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestNormalizeTag_TooLong(t *testing.T) {
	in := strings.Repeat("a", domain.MaxTagLength+1)
	_, err := domain.NormalizeTag(in)
	if err == nil {
		t.Fatal("expected error for over-long tag")
	}
}

func TestNormalizeTags_DedupSortDropEmpty(t *testing.T) {
	in := []string{"Patterns", "  patterns  ", "", "DDD", "ddd", "   ", "Architecture"}
	got, err := domain.NormalizeTags(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []string{"architecture", "ddd", "patterns"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestNormalizeTags_OverLongFails(t *testing.T) {
	in := []string{"ok", strings.Repeat("a", domain.MaxTagLength+1)}
	if _, err := domain.NormalizeTags(in); err == nil {
		t.Fatal("expected error for over-long tag")
	}
}

func TestValidateURL(t *testing.T) {
	good := []string{
		"https://example.com",
		"http://example.com/path?q=1",
		"https://sub.example.com:8443/x",
	}
	for _, u := range good {
		if err := domain.ValidateURL(u); err != nil {
			t.Errorf("expected %q ok, got %v", u, err)
		}
	}
	bad := []string{
		"",
		"   ",
		"not a url",
		"ftp://example.com",
		"mailto:hi@example.com",
		"file:///etc/passwd",
		"https://",
	}
	for _, u := range bad {
		if err := domain.ValidateURL(u); err == nil {
			t.Errorf("expected %q to fail", u)
		}
	}
}

func TestValidateTitle(t *testing.T) {
	if err := domain.ValidateTitle("ok"); err != nil {
		t.Errorf("expected ok, got %v", err)
	}
	if err := domain.ValidateTitle("   "); err == nil {
		t.Error("expected empty title to fail")
	}
	if err := domain.ValidateTitle(strings.Repeat("a", domain.MaxTitleLength+1)); err == nil {
		t.Error("expected over-long title to fail")
	}
}

func TestResourceType_Valid(t *testing.T) {
	for _, t1 := range domain.AllResourceTypes() {
		if !t1.Valid() {
			t.Errorf("%s should be valid", t1)
		}
	}
	if domain.ResourceType("podcasts").Valid() {
		t.Error("typo should not validate")
	}
	if domain.ResourceType("").Valid() {
		t.Error("empty should not validate")
	}
}

func TestLanguage_Valid(t *testing.T) {
	for _, l := range domain.AllLanguages() {
		if !l.Valid() {
			t.Errorf("%s should be valid", l)
		}
	}
	if domain.Language("FR").Valid() {
		t.Error("FR should not validate (not in enum)")
	}
}
