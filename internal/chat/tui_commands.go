package chat

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// slashResultMsg carries the task returned by a /get or /cancel
// invocation back into the Update loop so the rendering happens on
// the main goroutine (no data race on m.messages). Failures are
// reported via errDisplayMsg instead, so task is always non-nil here.
type slashResultMsg struct {
	verb string
	task *a2a.Task
}

// errDisplayMsg requests the Update loop to show a transient error.
type errDisplayMsg struct {
	msg string
}

// dispatchSlash routes a parsed slash command to its handler.
// exit commands return (m, tea.Quit); async operations return a Cmd
// that performs the RPC off the main goroutine; synchronous
// commands (like /new, /help) return nil.
func (m Model) dispatchSlash(sc SlashCmd) (tea.Model, tea.Cmd) {
	switch sc.Name {
	case "exit", "quit":
		return m, tea.Quit
	case "help":
		return m.handleHelp()
	case "new":
		return m.handleNew()
	case "get":
		return m.handleGet(sc.Arg)
	case "cancel":
		return m.handleCancel(sc.Arg)
	case "":
		// A bare "/" typed by itself.
		m.appendMessage(roleError, "enter a command name after / (try /help)")
		return m, nil
	default:
		m.appendMessage(roleError, fmt.Sprintf("unknown command: /%s (try /help)", sc.Name))
		return m, nil
	}
}

// handleHelp prints the slash command reference into the viewport as
// a single multi-line system message so it scrolls with the rest of
// the conversation.
func (m Model) handleHelp() (tea.Model, tea.Cmd) {
	lines := []string{"Commands:"}
	for _, s := range AllSuggestions {
		lines = append(lines, fmt.Sprintf("  %-10s %s", s.Name, s.Help))
	}
	lines = append(lines,
		"",
		"Tab / Enter accepts a suggestion while the dropdown is open.",
		"Arrow keys navigate suggestions; Esc closes the dropdown.",
		"Ctrl+C during a stream cancels the request and returns to the prompt.",
	)
	for _, ln := range lines {
		m.appendMessage(roleSystem, ln)
	}
	return m, nil
}

// handleNew clears the current task/context and logs a system line.
// The next user message will start a fresh conversation.
func (m Model) handleNew() (tea.Model, tea.Cmd) {
	m.state.Reset()
	m.appendMessage(roleSystem, "Started a new conversation")
	return m, nil
}

// handleGet invokes GetTask asynchronously. If arg is empty the
// current state's task is used; otherwise the explicit id is used
// without touching state. The RPC runs off the Update goroutine
// via a tea.Cmd so the UI stays responsive.
func (m Model) handleGet(arg string) (tea.Model, tea.Cmd) {
	targetID, err := m.state.ResolveTaskID(arg, "get")
	if err != nil {
		m.appendMessage(roleError, err.Error())
		return m, nil
	}
	ctx, c := m.ctx, m.client
	return m, func() tea.Msg {
		task, err := c.GetTask(ctx, &a2a.GetTaskRequest{ID: targetID})
		if err != nil {
			return errDisplayMsg{msg: fmt.Sprintf("tasks/get failed: %v", err)}
		}
		return slashResultMsg{verb: "get", task: task}
	}
}

func (m Model) handleCancel(arg string) (tea.Model, tea.Cmd) {
	targetID, err := m.state.ResolveTaskID(arg, "cancel")
	if err != nil {
		m.appendMessage(roleError, err.Error())
		return m, nil
	}
	ctx, c := m.ctx, m.client
	return m, func() tea.Msg {
		task, err := c.CancelTask(ctx, &a2a.CancelTaskRequest{ID: targetID})
		if err != nil {
			return errDisplayMsg{msg: fmt.Sprintf("tasks/cancel failed: %v", err)}
		}
		return slashResultMsg{verb: "cancel", task: task}
	}
}

// handleSlashResult renders a /get or /cancel result into the
// viewport as a system block. Kept terse intentionally: the TUI is
// not a full task browser, just a continuation-aware REPL.
func (m Model) handleSlashResult(msg slashResultMsg) (tea.Model, tea.Cmd) {
	header := fmt.Sprintf("[%s result] id=%s state=%s", msg.verb, msg.task.ID, msg.task.Status.State)
	m.appendMessage(roleSystem, header)
	if msg.task.ContextID != "" {
		m.appendMessage(roleSystem, fmt.Sprintf("  contextId: %s", msg.task.ContextID))
	}
	if msg.task.Status.Message != nil {
		if txt := presenter.TextFromParts(msg.task.Status.Message.Parts); txt != "" {
			m.appendMessage(roleSystem, "  "+txt)
		}
	}
	// If /get or /cancel targeted the current task and updated its
	// state, pull the latest IDs into m.state so continuation stays
	// consistent.
	if msg.task.ID == m.state.TaskID() {
		m.state.Update(msg.task.TaskInfo())
	}
	return m, nil
}
