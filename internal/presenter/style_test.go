package presenter

import (
	"os"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/fatih/color"
)

// TestMain disables color output for the entire presenter test suite.
// This ensures existing tests using strings.Contains continue to work
// without ANSI escape codes interfering with assertions.
func TestMain(m *testing.M) {
	color.NoColor = true
	os.Exit(m.Run())
}

func TestStyledTaskState_Completed(t *testing.T) {
	got := styledTaskState(a2a.TaskStateCompleted)
	if !strings.Contains(got, "TASK_STATE_COMPLETED") {
		t.Errorf("expected TASK_STATE_COMPLETED, got %q", got)
	}
}

func TestStyledTaskState_Working(t *testing.T) {
	got := styledTaskState(a2a.TaskStateWorking)
	if !strings.Contains(got, "TASK_STATE_WORKING") {
		t.Errorf("expected TASK_STATE_WORKING, got %q", got)
	}
}

func TestStyledTaskState_Failed(t *testing.T) {
	got := styledTaskState(a2a.TaskStateFailed)
	if !strings.Contains(got, "TASK_STATE_FAILED") {
		t.Errorf("expected TASK_STATE_FAILED, got %q", got)
	}
}

func TestStyledTaskState_AllStates(t *testing.T) {
	tests := []struct {
		state a2a.TaskState
		want  string
	}{
		{a2a.TaskStateCompleted, "TASK_STATE_COMPLETED"},
		{a2a.TaskStateWorking, "TASK_STATE_WORKING"},
		{a2a.TaskStateSubmitted, "TASK_STATE_SUBMITTED"},
		{a2a.TaskStateInputRequired, "TASK_STATE_INPUT_REQUIRED"},
		{a2a.TaskStateAuthRequired, "TASK_STATE_AUTH_REQUIRED"},
		{a2a.TaskStateFailed, "TASK_STATE_FAILED"},
		{a2a.TaskStateCanceled, "TASK_STATE_CANCELED"},
		{a2a.TaskStateRejected, "TASK_STATE_REJECTED"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := styledTaskState(tt.state)
			if got != tt.want {
				t.Errorf("styledTaskState(%s) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestStyledTaskState_Unspecified(t *testing.T) {
	got := styledTaskState(a2a.TaskStateUnspecified)
	if got != "" {
		t.Errorf("styledTaskState(unspecified) = %q, want empty string", got)
	}
}

func TestStyledHelpers_NoColor(t *testing.T) {
	// With color.NoColor = true (set in TestMain), styled functions
	// should return the input string unchanged.
	if got := styledHeader("=== Test ==="); got != "=== Test ===" {
		t.Errorf("styledHeader = %q, want %q", got, "=== Test ===")
	}
	if got := styledDivider("--- Test ---"); got != "--- Test ---" {
		t.Errorf("styledDivider = %q, want %q", got, "--- Test ---")
	}
	if got := styledLabel("Name:"); got != "Name:" {
		t.Errorf("styledLabel = %q, want %q", got, "Name:")
	}
	if got := styledTag("[task]"); got != "[task]" {
		t.Errorf("styledTag = %q, want %q", got, "[task]")
	}
	if got := styledSuccess("agent"); got != "agent" {
		t.Errorf("styledSuccess = %q, want %q", got, "agent")
	}
}

func TestStyledHelpers_WithColor(t *testing.T) {
	// Temporarily enable color to verify ANSI codes are present.
	color.NoColor = false
	defer func() { color.NoColor = true }()

	got := styledHeader("=== Test ===")
	if !strings.Contains(got, "=== Test ===") {
		t.Errorf("styledHeader should contain original text, got %q", got)
	}
	// ANSI escape codes start with \x1b[
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("styledHeader should contain ANSI codes when color enabled, got %q", got)
	}
}
