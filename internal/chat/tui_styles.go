package chat

import (
	"charm.land/lipgloss/v2"
)

// TUI colour palette. Kept as named constants so tweaks stay in one place
// and the styles read declaratively below.
const (
	// Brand-ish accent used for headers and selected suggestions.
	colorAccent = "#5F5FFF"
	// Neutral border / dim text.
	colorBorder = "#888888"
	// Soft foreground for regular body text.
	colorFgDim = "#DDDDDD"
	// Error red.
	colorError = "#FF5555"
	// System / meta message colour (italic grey).
	colorSystem = "#888888"
	// User message (the person typing) — cool cyan.
	colorUser = "#87CEEB"
	// Agent response — calm green.
	colorAgent = "#90EE90"
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
)
