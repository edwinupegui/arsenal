package cli

// Build-time information injected via -ldflags. See Makefile and .goreleaser.yaml.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
