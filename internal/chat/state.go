// Package chat implements the interactive chat REPL used by the
// `a2ahoy chat` subcommand. It exposes both a rich TUI mode (Bubble Tea)
// and a simple line-mode fallback (bufio.Scanner) through the same
// package.
//
// The core pure types (State, SlashCmd, etc.) are shared between the two
// modes so that parsing and request-building behaviour is identical.
package chat

import (
	"fmt"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// State tracks the conversation's current task and context identifiers.
// Both the TUI and simple-mode runners use this to carry IDs across turns
// so that follow-up messages implicitly continue the same task.
//
// The zero value is "fresh" — no active task, no context. A fresh State
// causes BuildChatRequest to use a2a.NewMessage for the first turn, and
// a non-fresh State causes it to use a2a.NewMessageForTask.
//
// Fields are unexported so external callers cannot bypass the merge
// semantics of Update. Read access is provided through the TaskID and
// ContextID accessors.
type State struct {
	currentTaskID    a2a.TaskID
	currentContextID string
}

// TaskID returns the State's current task identifier (empty when fresh).
func (s *State) TaskID() a2a.TaskID { return s.currentTaskID }

// ContextID returns the State's current context identifier (empty when fresh).
func (s *State) ContextID() string { return s.currentContextID }

// IsFresh reports whether the State has no active task or context.
// A fresh State signals that the next message should start a new
// conversation (via a2a.NewMessage) rather than continue one.
func (s *State) IsFresh() bool {
	return s.currentTaskID == "" && s.currentContextID == ""
}

// Reset clears the State, returning it to the fresh (initial) condition.
// Invoked by the `/new` slash command.
func (s *State) Reset() {
	s.currentTaskID = ""
	s.currentContextID = ""
}

// Update merges a TaskInfo into the State. Only non-empty fields in info
// overwrite the existing values. This defensive merge matters because a
// mid-stream TaskStatusUpdateEvent may report only the TaskID without
// echoing the ContextID (or vice versa), and we want to preserve whatever
// we have already seen.
func (s *State) Update(info a2a.TaskInfo) {
	if info.TaskID != "" {
		s.currentTaskID = info.TaskID
	}
	if info.ContextID != "" {
		s.currentContextID = info.ContextID
	}
}

// TaskInfo returns the current State as an a2a.TaskInfo. Since
// a2a.TaskInfo itself satisfies TaskInfoProvider, the result can be
// passed directly to a2a.NewMessageForTask.
func (s *State) TaskInfo() a2a.TaskInfo {
	return a2a.TaskInfo{
		TaskID:    s.currentTaskID,
		ContextID: s.currentContextID,
	}
}

// ResolveTaskID returns an explicit task id when arg is non-empty,
// otherwise the State's current task id. When neither is available it
// returns an error formatted as `no active task; usage: /<verb> <task-id>`,
// matching what users see from both the TUI and simple-mode runners.
//
// Centralising this here ensures /get and /cancel report identical
// errors regardless of which UI the user is in.
func (s *State) ResolveTaskID(arg, verb string) (a2a.TaskID, error) {
	if arg != "" {
		return a2a.TaskID(arg), nil
	}
	if s.currentTaskID != "" {
		return s.currentTaskID, nil
	}
	return "", fmt.Errorf("no active task; usage: /%s <task-id>", verb)
}
