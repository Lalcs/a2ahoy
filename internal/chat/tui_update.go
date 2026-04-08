package chat

import (
	tea "charm.land/bubbletea/v2"
)

// Layout constants.
// These determine how vertical space is distributed: the viewport
// consumes whatever remains after the header, input, suggestions (if
// shown), and status bar have taken their share.
const (
	headerHeight    = 1 // 1 line for agent name banner
	inputHeight     = 3 // borderlines + content
	statusBarHeight = 1
	errLineHeight   = 1
)

// Update satisfies tea.Model. The switch is ordered so specific
// message types are handled first, then the unhandled tail is
// delegated to child components (textinput, viewport) so their
// internal state — cursor blinks, scroll offsets — continues to work.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case streamEventMsg:
		return m.handleStreamEvent(msg)
	case streamEndMsg:
		return m.handleStreamEnd(msg)
	case slashResultMsg:
		return m.handleSlashResult(msg)
	case errDisplayMsg:
		m.errMsg = msg.msg
		return m, nil
	}

	// Fall through: let the components process the message. This
	// keeps viewport mouse wheel scrolling and textinput cursor
	// blinking working without extra glue.
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// handleResize updates component dimensions on WindowSizeMsg. The
// first resize also flips the ready flag so View starts rendering.
func (m Model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true

	// Subtract the chrome from the total height to size the viewport.
	chrome := headerHeight + inputHeight + statusBarHeight
	if m.errMsg != "" {
		chrome += errLineHeight
	}
	vh := msg.Height - chrome
	if vh < 3 {
		vh = 3
	}
	m.viewport.SetWidth(msg.Width)
	m.viewport.SetHeight(vh)

	// Input width accounts for the rounded border and horizontal
	// padding (2 characters on each side).
	iw := msg.Width - 4
	if iw < 10 {
		iw = 10
	}
	m.textInput.SetWidth(iw)

	// Re-render after resize so wrapping is up to date.
	m.updateViewportContent()
	return m, nil
}

// handleKey dispatches all key-press events. Slash-command
// autocomplete, stream cancellation, and message submission all flow
// through here. Any key not explicitly consumed falls through to
// textinput so normal typing works.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.streaming {
			// Cancel the in-flight stream but stay in the REPL. The
			// handleStreamEnd handler will append an "[interrupted]"
			// system line when the goroutine returns.
			if m.streamCancel != nil {
				m.streamCancel()
			}
			return m, nil
		}
		return m, tea.Quit

	case "enter":
		// When the dropdown is open with a selection, Enter accepts
		// the suggestion instead of submitting — the user still has
		// to hit Enter again to actually send the command, which
		// matches the "confirm then commit" convention of shell
		// autocomplete and gives IME users a safety beat between
		// conversion confirm and command send.
		if m.showSuggestions && m.selectedSugg >= 0 && m.selectedSugg < len(m.suggestions) {
			m.acceptSuggestion()
			return m, nil
		}
		return m.submitInput()

	case "tab":
		if m.showSuggestions && len(m.suggestions) > 0 {
			m.acceptSuggestion()
			return m, nil
		}
		// If no dropdown, leave tab alone; textinput will ignore it.

	case "up":
		if m.showSuggestions {
			if m.selectedSugg > 0 {
				m.selectedSugg--
			}
			return m, nil
		}
		// Without an open dropdown, Up scrolls the viewport.
		m.viewport.ScrollUp(1)
		return m, nil

	case "down":
		if m.showSuggestions {
			if m.selectedSugg < len(m.suggestions)-1 {
				m.selectedSugg++
			}
			return m, nil
		}
		m.viewport.ScrollDown(1)
		return m, nil

	case "pgup":
		m.viewport.ScrollUp(m.viewport.Height())
		return m, nil

	case "pgdown":
		m.viewport.ScrollDown(m.viewport.Height())
		return m, nil

	case "esc":
		if m.showSuggestions {
			m.showSuggestions = false
			return m, nil
		}
	}

	// Normal key event: let textinput process it, then reassess
	// suggestion visibility based on the new value. The error line
	// is cleared on any keystroke so stale errors don't linger.
	m.errMsg = ""
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	m.updateSuggestions()
	return m, cmd
}

// submitInput is called when the user presses Enter with no
// suggestion selected. It parses the line, dispatches slash commands,
// or starts a streaming turn for regular messages.
func (m Model) submitInput() (tea.Model, tea.Cmd) {
	line := m.textInput.Value()
	text, isSlash, sc := ParseInputLine(line)

	// Always clear the input and hide the dropdown on submit so the
	// next turn starts with a clean slate.
	m.textInput.SetValue("")
	m.showSuggestions = false

	if text == "" {
		return m, nil
	}

	if isSlash {
		return m.dispatchSlash(sc)
	}

	// Regular message → append to transcript, then kick off stream.
	m.appendMessage(roleUser, text)
	req := BuildChatRequest(&m.state, text)
	return m.startStream(req)
}
