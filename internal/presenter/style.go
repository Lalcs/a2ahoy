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
	headerStyleAttrs = []color.Attribute{color.FgHiBlue, color.Bold}
	labelStyleAttrs  = []color.Attribute{color.FgHiBlue}
	greenStyleAttrs  = []color.Attribute{color.FgHiGreen}
	yellowStyleAttrs = []color.Attribute{color.FgHiYellow}
	redStyleAttrs    = []color.Attribute{color.FgHiRed}
)

func sprintStyle(s string, attrs ...color.Attribute) string {
	styler := color.New(attrs...)
	if !color.NoColor {
		// Respect an explicit runtime override even when NO_COLOR was present
		// during package initialization.
		styler.EnableColor()
	}
	return styler.Sprint(s)
}

// styledHeader formats a section header (e.g., "=== Agent Card ===").
func styledHeader(s string) string { return sprintStyle(s, headerStyleAttrs...) }

// styledDivider formats a sub-section divider (e.g., "--- Capabilities ---").
func styledDivider(s string) string { return sprintStyle(s, headerStyleAttrs...) }

// styledLabel formats a field label (e.g., "Name:", "Status:").
func styledLabel(s string) string { return sprintStyle(s, labelStyleAttrs...) }

// styledTag formats an event/role tag (e.g., "[task]", "[ROLE_AGENT]").
func styledTag(s string) string { return sprintStyle(s, labelStyleAttrs...) }

// styledSuccess formats a success value (e.g., agent name).
func styledSuccess(s string) string { return sprintStyle(s, greenStyleAttrs...) }

// styledWarning formats a caution message (e.g., "[WARN]" or
// "update available"). Yellow is reserved for non-blocking issues that
// the user should see but can continue past.
func styledWarning(s string) string { return sprintStyle(s, yellowStyleAttrs...) }

// styledError formats a failure message (e.g., "[ERROR]" or
// "invalid latest tag"). Red is reserved for blocking conditions.
func styledError(s string) string { return sprintStyle(s, redStyleAttrs...) }

// styledTaskState returns the colored string for a task state.
func styledTaskState(state a2a.TaskState) string {
	switch state {
	case a2a.TaskStateCompleted:
		return sprintStyle(string(state), greenStyleAttrs...)
	case a2a.TaskStateWorking, a2a.TaskStateSubmitted,
		a2a.TaskStateInputRequired, a2a.TaskStateAuthRequired:
		return sprintStyle(string(state), yellowStyleAttrs...)
	case a2a.TaskStateFailed, a2a.TaskStateCanceled, a2a.TaskStateRejected:
		return sprintStyle(string(state), redStyleAttrs...)
	default:
		return string(state)
	}
}
