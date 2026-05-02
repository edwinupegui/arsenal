// Package tui implements the interactive Bubble Tea application that runs
// when the user invokes `arsenal` with no subcommand or `arsenal tui`.
package tui

import "github.com/charmbracelet/lipgloss"

// Palette — picked once so every view uses the same colors. Tuned for both
// light and dark backgrounds via lipgloss.AdaptiveColor.
var (
	colorAccent  = lipgloss.AdaptiveColor{Light: "#4f46e5", Dark: "#818cf8"}
	colorMuted   = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"}
	colorWarning = lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#fbbf24"}
	colorDanger  = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#f87171"}
	colorOK      = lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#4ade80"}
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fafafa")).
			Background(colorAccent).
			Padding(0, 1).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().Foreground(colorMuted)
	keyStyle   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	statusOKStyle    = lipgloss.NewStyle().Foreground(colorOK)
	statusErrorStyle = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)

	detailLabelStyle = lipgloss.NewStyle().Foreground(colorAccent).Width(12).Bold(true)
	detailValueStyle = lipgloss.NewStyle()

	trashBannerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#fafafa")).
				Background(colorWarning).
				Padding(0, 1).
				Bold(true)
)
