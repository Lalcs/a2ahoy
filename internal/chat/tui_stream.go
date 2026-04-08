package chat

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// streamEventInternal is the over-the-channel transport between the
// streaming goroutine and the Bubble Tea Update loop. Keeping it
// unexported prevents it from leaking into the public Msg surface.
type streamEventInternal struct {
	event a2a.Event
	err   error
}

// streamEventMsg is delivered to Update for every successful stream
// event. It unwraps the per-event a2a.Event so handlers can type-switch
// on it directly.
type streamEventMsg struct {
	event a2a.Event
}

// streamEndMsg is delivered exactly once at the end of a stream. When
// err is non-nil the stream failed (or was interrupted by ctx cancel).
type streamEndMsg struct {
	err error
}

// startStream begins a streaming turn. It spawns a goroutine that
// drains the SendStreamingMessage iterator into m.streamCh, and
// returns the first tea.Cmd that waits for an event. Subsequent
// events are requested by handleStreamEvent chaining waitForStreamEvent.
//
// Safety notes:
//   - The goroutine captures only the arguments passed in and the
//     local ctx/ch, not m, so there is no data race on Model fields.
//   - m.streamCancel is stored on the returned Model so that Update
//     can trigger cancellation on Ctrl+C.
//   - We intentionally use context.Background as the parent (not
//     a program-level ctx) so only the per-turn Ctrl+C affects the
//     stream; the Bubble Tea program context handles program shutdown.
func (m Model) startStream(req *a2a.SendMessageRequest) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	ch := make(chan streamEventInternal, 32)
	m.streamCh = ch
	m.streaming = true
	m.streamBuf.Reset()
	m.lastTurnInfo = a2a.TaskInfo{}
	m.errMsg = ""

	// Capture the client by value into the goroutine so there is no
	// race with the Update loop reassigning m.client (which never
	// happens, but future-proofs against it). The goroutine is the
	// only writer to ch and must close it when the iterator exhausts
	// (normal EOF) or after delivering an error. Update relies on the
	// close to fire the final streamEndMsg.
	go streamPump(ctx, m.client, req, ch)

	return m, waitForStreamEvent(ch)
}

// streamPump drains the SendStreamingMessage iterator into ch. It
// closes ch on return so the consumer observes EOF even when the
// underlying iterator returns zero events.
func streamPump(ctx context.Context, c client.A2AClient, req *a2a.SendMessageRequest, ch chan<- streamEventInternal) {
	defer close(ch)
	for event, err := range c.SendStreamingMessage(ctx, req) {
		ch <- streamEventInternal{event: event, err: err}
		if err != nil {
			return
		}
	}
}

// waitForStreamEvent blocks on the channel and returns a Bubble Tea
// msg for the next event. This is the tea.Cmd idiom for pumping async
// work through the Update loop one message at a time.
func waitForStreamEvent(ch <-chan streamEventInternal) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			// Channel closed without an explicit error → clean end of stream.
			return streamEndMsg{}
		}
		if msg.err != nil {
			return streamEndMsg{err: msg.err}
		}
		return streamEventMsg{event: msg.event}
	}
}

// handleStreamEvent folds one stream event into the model and
// requests the next one. The text extraction intentionally reuses
// the logic of presenter.PrintStreamEvent but re-formats the output
// for the viewport's in-line display (no ANSI codes from fatih/color,
// lipgloss styles only).
func (m Model) handleStreamEvent(msg streamEventMsg) (tea.Model, tea.Cmd) {
	// Merge TaskInfo so whatever IDs the event carries are preserved
	// even if a later event omits one of them.
	info := msg.event.TaskInfo()
	if info.TaskID != "" {
		m.lastTurnInfo.TaskID = info.TaskID
	}
	if info.ContextID != "" {
		m.lastTurnInfo.ContextID = info.ContextID
	}

	// Extract human-readable text from the event and fold it into the
	// current agent bubble. Unrecognised event types render as a
	// compact system line so nothing silently vanishes.
	text, meta := extractEventText(msg.event)
	if text != "" {
		m.streamBuf.WriteString(text)
		m.setLastAgentText(m.streamBuf.String())
	}
	if meta != "" {
		m.appendMessage(roleSystem, meta)
	}

	return m, waitForStreamEvent(m.streamCh)
}

// handleStreamEnd finalises the streaming turn. On success, state is
// updated with the accumulated TaskInfo; on cancellation an
// "[interrupted]" system line is logged and state is preserved
// unchanged; on hard errors the error is shown and state is preserved.
func (m Model) handleStreamEnd(msg streamEndMsg) (tea.Model, tea.Cmd) {
	m.streaming = false
	if m.streamCancel != nil {
		// Safe to call again even after the goroutine has returned.
		m.streamCancel()
		m.streamCancel = nil
	}
	m.streamCh = nil

	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			m.appendMessage(roleSystem, "[interrupted]")
		} else {
			m.appendMessage(roleError, fmt.Sprintf("stream error: %v", msg.err))
		}
		return m, nil
	}

	if m.lastTurnInfo.TaskID != "" || m.lastTurnInfo.ContextID != "" {
		m.state.Update(m.lastTurnInfo)
	}
	return m, nil
}

// extractEventText derives the text to append to the agent bubble and
// any optional meta line for the given event. A non-empty meta line
// is rendered as a system message (italic grey) and is used for things
// like status-update chips and artifact headers.
func extractEventText(event a2a.Event) (text, meta string) {
	switch v := event.(type) {
	case *a2a.Message:
		// Agent-authored messages contain zero or more text parts.
		return presenter.TextFromParts(v.Parts), ""
	case *a2a.TaskStatusUpdateEvent:
		state := string(v.Status.State)
		if v.Status.Message != nil && len(v.Status.Message.Parts) > 0 {
			return presenter.TextFromParts(v.Status.Message.Parts), fmt.Sprintf("[status] %s", state)
		}
		return "", fmt.Sprintf("[status] %s", state)
	case *a2a.TaskArtifactUpdateEvent:
		if v.Artifact == nil {
			return "", ""
		}
		var header string
		if !v.Append && v.Artifact.Name != "" {
			header = fmt.Sprintf("[artifact] %s", v.Artifact.Name)
		}
		return presenter.TextFromParts(v.Artifact.Parts), header
	case *a2a.Task:
		// Task events are informational; show id/state as a system line.
		return "", fmt.Sprintf("[task] id=%s state=%s", v.ID, v.Status.State)
	default:
		return "", fmt.Sprintf("[unknown event] %T", event)
	}
}
