package chat

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// RunTUI starts the Bubble Tea chat REPL.
//
// ctx is the top-level cancellation context from cmd.runChat (a
// signal.NotifyContext wrapping SIGINT). Passing it via tea.WithContext
// lets Bubble Tea shut down gracefully if the user interrupts before
// the program has captured the terminal. The same ctx is also stored
// on the Model so slash-command RPCs (e.g., /get, /cancel) cancel
// promptly when the user quits the TUI.
//
// Alt-screen mode and mouse support are enabled per-frame via the
// tea.View struct in Model.View, as Bubble Tea v2 no longer uses
// program-level options for these.
func RunTUI(ctx context.Context, c client.A2AClient, card *a2a.AgentCard, baseURL string, initialParts []*a2a.Part) error {
	// baseURL is accepted for symmetry with RunSimple but not used by
	// the TUI itself — the header no longer displays it and the user
	// already knows the URL from the command line invocation.
	_ = baseURL
	m := newModel(ctx, c, card, initialParts)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	if _, err := p.Run(); err != nil {
		// Suppress tea.ErrProgramKilled / ErrInterrupted so Ctrl+C at
		// the top level produces a clean exit status rather than
		// surfacing as a cobra error.
		if errors.Is(err, tea.ErrProgramKilled) ||
			errors.Is(err, tea.ErrInterrupted) ||
			errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	return nil
}
