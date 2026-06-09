// Package config holds the typed catalog of configuration keys stored in the
// arsenal_config table (see internal/configstore). It is the single source of
// truth for what configuration keys exist, what their types are, and how to
// validate user-supplied values.
//
// New domains (todos, finance, calendar) and new settings (landing surface,
// active domains, etc.) register their keys here. Callers in the cli/tui/web
// layers read the catalog to drive help text, completions, and validation.
package config

import (
	"fmt"
	"sort"
	"strings"
)

// Key is a typed arsenal_config key. The underlying type is string for
// storage compatibility with the arsenal_config table. Use the package-level
// constants (KeyCurrency, KeyLandingSurface, etc.) instead of constructing
// raw strings at the call site.
type Key string

// Catalog keys. Add new constants here as new domains or settings need
// configuration. Keep the underlying string lowercase and stable — once a
// value is persisted, renaming the constant is a breaking change.
const (
	// KeyCurrency is the ISO-4217 currency code for finance entries (v3.x).
	// Allowed values come from KeyMeta.EnumValues.
	KeyCurrency Key = "currency"

	// KeyLandingSurface is the default surface to open when `arsenal` is run
	// with no args. Values: "tui" | "web".
	KeyLandingSurface Key = "landing_surface"

	// KeyActiveDomains is the comma-separated list of domains that appear in
	// the sidebar (web) and the TUI area switcher. Valid items are the five
	// domain names: resources, todos, today, finance, calendar.
	KeyActiveDomains Key = "active_domains"
)

// Type is the parsed type of a config value. The on-disk representation is
// always text; Type describes how to interpret and validate it.
type Type string

const (
	TypeString Type = "string"
	TypeBool   Type = "bool"
	TypeEnum   Type = "enum" // value must be one of EnumValues
	TypeList   Type = "list" // comma-separated; each item is validated
)

// KeyMeta describes a single config key: its type, default, human-readable
// description, allowed values (for enums), and an optional validator.
type KeyMeta struct {
	Type        Type
	Default     string
	Description string
	EnumValues  []string      // populated only when Type == TypeEnum
	Validate    func(string) error
}

// Catalog is the single source of truth for what config keys exist. Add new
// keys here; everything else (configstore, CLI, help text) reads from it.
var Catalog = map[Key]KeyMeta{
	KeyCurrency: {
		Type:        TypeEnum,
		Default:     "USD",
		Description: "ISO-4217 currency code for finance entries (v3.x).",
		EnumValues:  []string{"USD", "EUR", "ARS", "BRL", "MXN", "GBP"},
	},
	KeyLandingSurface: {
		Type:        TypeEnum,
		Default:     "tui",
		Description: "Default surface to open when `arsenal` is run with no args.",
		EnumValues:  []string{"tui", "web"},
	},
	KeyActiveDomains: {
		Type:        TypeList,
		Default:     "resources,todos",
		Description: "Comma-separated list of domains shown in the sidebar and TUI area switcher.",
		Validate:    validateActiveDomains,
	},
}

// validDomains is the closed set of domain names that may appear in
// KeyActiveDomains. Kept here (not in the catalog) because it is referenced
// by validators in TUI and web layers too.
var validDomains = map[string]struct{}{
	"resources": {},
	"todos":     {},
	"today":     {},
	"finance":   {},
	"calendar":  {},
}

func validateActiveDomains(s string) error {
	seen := make(map[string]struct{})
	for _, d := range strings.Split(s, ",") {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if _, ok := validDomains[d]; !ok {
			return fmt.Errorf("unknown domain %q (allowed: resources, todos, today, finance, calendar)", d)
		}
		if _, dup := seen[d]; dup {
			return fmt.Errorf("duplicate domain %q in list", d)
		}
		seen[d] = struct{}{}
	}
	return nil
}

// All returns every registered key, sorted by underlying string. Used by
// `arsenal config list`.
func All() []Key {
	keys := make([]Key, 0, len(Catalog))
	for k := range Catalog {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// GetMeta returns the KeyMeta for k, or false if the key is unknown.
func GetMeta(k Key) (KeyMeta, bool) {
	m, ok := Catalog[k]
	return m, ok
}

// IsValidDomain reports whether d is a recognized domain name. Useful for
// the TUI area-switcher and the web sidebar, which both consume the
// KeyActiveDomains list and may want to filter or validate.
func IsValidDomain(d string) bool {
	_, ok := validDomains[d]
	return ok
}
