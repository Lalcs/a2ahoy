package chat

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
)

// View renders the full TUI frame. It returns a tea.View struct so
// the alt-screen flag, mouse mode, and cursor position can be set per
// frame (Bubble Tea v2 moved these from program-level options into
// the per-frame View).
func (m Model) View() tea.View {
	if !m.ready {
		return tea.View{Content: "initializing…"}
	}

	// Compose the frame top-down.
	header := m.renderHeader()
	body := m.viewport.View()
	inputBox := inputBorderStyle.Width(m.width - 2).Render(m.textInput.View())

	var dropdown string
	if m.showSuggestions {
		dropdown = m.renderSuggestions()
	}

	var errLine string
	if m.errMsg != "" {
		errLine = errLineStyle.Render("⚠ " + m.errMsg)
	}

	status := m.renderStatusBar()

	frame := lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		inputBox,
		dropdown,
		errLine,
		status,
	)

	return tea.View{
		Content:   frame,
		AltScreen: true,
		MouseMode: tea.MouseModeCellMotion,
	}
}

// renderHeader draws the top banner with agent name and base URL.
func (m Model) renderHeader() string {
	title := fmt.Sprintf(" %s — %s ", m.agentName(), m.baseURL)
	// Pad to full width so the background colour extends across the screen.
	if m.width > 0 {
		style := headerStyle.Width(m.width)
		return style.Render(title)
	}
	return headerStyle.Render(title)
}

// renderSuggestions draws the dropdown list under the input box.
// The highlighted row uses the accent background; the rest use a
// dimmed foreground. Each entry shows the command name on the left
// and the one-line help on the right.
func (m Model) renderSuggestions() string {
	if len(m.suggestions) == 0 {
		return ""
	}
	var rows []string
	for i, s := range m.suggestions {
		name := fmt.Sprintf("%-10s", s.Name)
		help := suggHelpStyle.Render(s.Help)
		line := fmt.Sprintf("%s  %s", name, help)
		if i == m.selectedSugg {
			line = suggSelectedStyle.Render("▸ " + line)
		} else {
			line = suggNormalStyle.Render("  " + line)
		}
		rows = append(rows, line)
	}
	return suggBoxStyle.Render(strings.Join(rows, "\n"))
}

// renderStatusBar shows continuation state (task/context ids),
// the streaming indicator, and short keybinding hints.
func (m Model) renderStatusBar() string {
	var parts []string

	if id := m.state.TaskID(); id != "" {
		parts = append(parts, fmt.Sprintf("task:%s", truncateID(string(id), 10)))
	}
	if id := m.state.ContextID(); id != "" {
		parts = append(parts, fmt.Sprintf("ctx:%s", truncateID(id, 10)))
	}
	if m.streaming {
		parts = append(parts, m.spinner.View()+" streaming…")
	}

	// Keybinding hints always shown on the right.
	hints := fmt.Sprintf("%s: send  %s: autocomplete  %s: exit",
		statusKeyStyle.Render("⏎"),
		statusKeyStyle.Render("Tab"),
		statusKeyStyle.Render("Ctrl+C"),
	)

	// Left side: meta parts; right side: hints. Pad between them.
	left := strings.Join(parts, "  |  ")
	if left == "" {
		left = "ready"
	}

	if m.width > 0 {
		// Reserve space for the hints (computed by lipgloss width
		// which handles ANSI-aware width correctly). Fill the middle
		// with spaces so the hints sit flush right.
		hintW := lipgloss.Width(hints)
		leftW := lipgloss.Width(left)
		padding := m.width - hintW - leftW - 2
		if padding < 1 {
			padding = 1
		}
		return statusBarStyle.Width(m.width).Render(left + strings.Repeat(" ", padding) + hints)
	}
	return statusBarStyle.Render(left + "   " + hints)
}

// truncateID shortens a UUID-like string to the leading n characters
// followed by "…" when longer. Useful for keeping the status bar from
// wrapping on long task/context identifiers.
func truncateID(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
