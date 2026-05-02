package legacy

import "strings"

// slugify converts a category name to a stable, URL-safe identifier.
// Lowercases, keeps [a-z0-9], replaces every other run of characters with a
// single dash, and trims leading/trailing dashes. Diacritics are stripped
// by mapping common Latin-1 vowels — sufficient for the legacy dataset
// without pulling in golang.org/x/text just for one migration.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = stripDiacritics(s)
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if b.Len() > 0 && !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// stripDiacritics maps the small set of accented vowels that appear in the
// legacy category names ("Tendencias", "Adicionales") to their ASCII
// counterparts. Anything outside the map is left untouched — the caller's
// non-alphanum filter will then drop it.
func stripDiacritics(s string) string {
	r := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N",
	)
	return r.Replace(s)
}
