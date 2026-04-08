package chat

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"
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

	// The status bar is the single-row footer: usage hint on the
	// left, keybinding hints on the right. Errors are rendered
	// inside the transcript (via roleError messages) so they scroll
	// with the rest of the conversation instead of squeezing into
	// this one-row slot.
	status := m.renderStatusBar()

	frame := lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		inputBox,
		dropdown,
		status,
	)

	return tea.View{
		Content:   frame,
		AltScreen: true,
		MouseMode: tea.MouseModeCellMotion,
	}
}

// renderHeader draws the top banner. Only the agent name is shown —
// the base URL is already known to the user from the command line
// invocation and its length would otherwise cause overflow on narrow
// terminals, so we deliberately keep the header minimal.
func (m Model) renderHeader() string {
	title := " " + m.agentName() + " "
	// Pad to full width so the background colour extends across the screen.
	if m.width > 0 {
		return headerStyle.Width(m.width).Render(title)
	}
	return headerStyle.Render(title)
}

// renderSuggestions draws the dropdown list under the input box.
// The highlighted row uses the accent background; the rest use a
// dimmed foreground. Each entry shows the command name on the left
// and the one-line help on the right.
//
// The dropdown is sized to match the input box width (m.width - 2)
// so the two elements line up visually, and each row is padded to
// the inner width so the selected-row background extends all the
// way across the box.
func (m Model) renderSuggestions() string {
	if len(m.suggestions) == 0 {
		return ""
	}
	// Match the input box outer width so the dropdown aligns with it.
	boxWidth := m.width - 2
	if boxWidth < 20 {
		boxWidth = 20
	}
	innerWidth := boxWidth - suggBoxStyle.GetHorizontalFrameSize()
	if innerWidth < 10 {
		innerWidth = 10
	}
	var rows []string
	for i, s := range m.suggestions {
		name := fmt.Sprintf("%-10s", s.Name)
		help := suggHelpStyle.Render(s.Help)
		line := fmt.Sprintf("%s  %s", name, help)
		if i == m.selectedSugg {
			line = suggSelectedStyle.Width(innerWidth).Render("▸ " + line)
		} else {
			line = suggNormalStyle.Width(innerWidth).Render("  " + line)
		}
		rows = append(rows, line)
	}
	return suggBoxStyle.Width(boxWidth).Render(strings.Join(rows, "\n"))
}

// renderStatusBar draws the single-row footer. The left side is the
// "Type a message or / for commands" usage hint (plus the streaming
// spinner while a turn is in flight). The right side is the
// keybinding cheat sheet. On narrow terminals the keys are dropped
// first so the hint stays visible.
//
// Task and context ids are intentionally not shown here: they are
// available via the /get slash command, and cramming them into the
// status bar used to push the hint off the edge on narrow windows.
func (m Model) renderStatusBar() string {
	// Left side always leads with the usage hint so new users see
	// the slash-command affordance at a glance.
	left := "Type a message or / for commands"
	if m.streaming {
		left = left + "  ·  " + m.spinner.View() + " streaming…"
	}

	// Keybinding hints always shown on the right when there is room.
	hints := fmt.Sprintf("%s: send  %s: autocomplete  %s: exit",
		statusKeyStyle.Render("⏎"),
		statusKeyStyle.Render("Tab"),
		statusKeyStyle.Render("Ctrl+C"),
	)

	if m.width > 0 {
		avail := m.width - statusBarStyle.GetHorizontalFrameSize()
		leftW := lipgloss.Width(left)
		hintsW := lipgloss.Width(hints)

		// Best case: hint + at least one cell of gap + keys all fit.
		if leftW+hintsW+1 <= avail {
			padding := avail - leftW - hintsW
			return statusBarStyle.Width(m.width).Render(left + strings.Repeat(" ", padding) + hints)
		}
		// Drop the keys so the usage hint stays readable. If the
		// hint itself still overflows, truncate it — wrapping would
		// push the status bar onto a second row and clip the bottom
		// of the frame out of view, which is exactly the failure
		// mode we're guarding against here.
		if leftW > avail {
			left = ansi.Truncate(left, avail, "…")
		}
		return statusBarStyle.Width(m.width).Render(left)
	}
	return statusBarStyle.Render(left + "   " + hints)
}
