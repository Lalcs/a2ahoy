package chat

import (
	"charm.land/lipgloss/v2"
)

// TUI colour palette, aligned with the A2Ahoy logo (docs/logo.svg,
// docs/logo-concept.md). Kept as named constants so tweaks stay in one
// place and the styles read declaratively below.
const (
	// Logo primary (Google Blue). Header background, selected
	// suggestion background, border accents.
	colorAccent = "#1A73E8"
	// Neutral border / dim text. Matches the logo subtitle colour.
	colorBorder = "#666666"
	// Soft foreground for regular body text.
	colorFgDim = "#DDDDDD"
	// Error red. Not present in the logo, kept for UI convention.
	colorError = "#FF5555"
	// System / meta message colour. Aligned with the border grey.
	colorSystem = "#666666"
	// User message (the person typing) — signal green, matching the
	// logo's signal ring.
	colorUser = "#34A853"
	// Agent response — a lighter variant of the primary blue.
	colorAgent = "#4285F4"
	// Status bar background.
	colorStatusBg = "#1F1F1F"
)

// Styles used by the TUI view. All styles are package-level so the View
// function stays allocation-free on the hot path (Bubble Tea re-renders
// on every Update).
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color(colorAccent)).
			Padding(0, 1)

	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(colorBorder)).
				Padding(0, 1)

	// Suggestions dropdown container.
	suggBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(colorAccent)).
			Padding(0, 1)

	suggNormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFgDim))

	suggSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color(colorAccent)).
				Bold(true)

	suggHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBorder)).
			Italic(true)

	userPrefixStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorUser)).
			Bold(true)

	agentPrefixStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorAgent)).
				Bold(true)

	systemMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSystem)).
			Italic(true)

	errLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError)).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFgDim)).
			Background(lipgloss.Color(colorStatusBg)).
			Padding(0, 1)

	statusKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)).
			Background(lipgloss.Color(colorStatusBg)).
			Bold(true)

	// spinnerStyle paints the streaming spinner in the accent colour
	// against the status bar background so it sits flush with the
	// surrounding text.
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)).
			Background(lipgloss.Color(colorStatusBg))
)
