package presenter

import (
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/fatih/color"
)

// Brand color styles based on docs/logo-concept.md:
//   Blue  #1A73E8 — structural elements (headers, labels, tags)
//   Green #34A853 — success states, agent identity
//   Yellow #FBBC04 — caution, in-progress states
//   Red   (ANSI)  — error, failure states

var (
	headerStyle = color.New(color.FgHiBlue, color.Bold)
	labelStyle  = color.New(color.FgHiBlue)
	greenStyle  = color.New(color.FgHiGreen)
	yellowStyle = color.New(color.FgHiYellow)
	redStyle    = color.New(color.FgHiRed)
)

// styledHeader formats a section header (e.g., "=== Agent Card ===").
func styledHeader(s string) string { return headerStyle.Sprint(s) }

// styledDivider formats a sub-section divider (e.g., "--- Capabilities ---").
func styledDivider(s string) string { return headerStyle.Sprint(s) }

// styledLabel formats a field label (e.g., "Name:", "Status:").
func styledLabel(s string) string { return labelStyle.Sprint(s) }

// styledTag formats an event/role tag (e.g., "[task]", "[ROLE_AGENT]").
func styledTag(s string) string { return labelStyle.Sprint(s) }

// styledSuccess formats a success value (e.g., agent name).
func styledSuccess(s string) string { return greenStyle.Sprint(s) }

// styledWarning formats a caution message (e.g., "[WARN]" or
// "update available"). Yellow is reserved for non-blocking issues that
// the user should see but can continue past.
func styledWarning(s string) string { return yellowStyle.Sprint(s) }

// styledError formats a failure message (e.g., "[ERROR]" or
// "invalid latest tag"). Red is reserved for blocking conditions.
func styledError(s string) string { return redStyle.Sprint(s) }

// styledTaskState returns the colored string for a task state.
func styledTaskState(state a2a.TaskState) string {
	switch state {
	case a2a.TaskStateCompleted:
		return greenStyle.Sprint(state)
	case a2a.TaskStateWorking, a2a.TaskStateSubmitted,
		a2a.TaskStateInputRequired, a2a.TaskStateAuthRequired:
		return yellowStyle.Sprint(state)
	case a2a.TaskStateFailed, a2a.TaskStateCanceled, a2a.TaskStateRejected:
		return redStyle.Sprint(state)
	default:
		return string(state)
	}
}
