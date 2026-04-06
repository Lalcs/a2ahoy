package presenter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestPrintStreamEvent_Task(t *testing.T) {
	task := &a2a.Task{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status: a2a.TaskStatus{
			State: a2a.TaskStateWorking,
		},
	}

	var buf bytes.Buffer
	if err := PrintStreamEvent(&buf, task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "[task] id=task-1 status=TASK_STATE_WORKING") {
		t.Errorf("unexpected output:\n%s", got)
	}
}

func TestPrintStreamEvent_Message(t *testing.T) {
	msg := &a2a.Message{
		Role:  a2a.MessageRoleAgent,
		Parts: a2a.ContentParts{a2a.NewTextPart("streaming response")},
	}

	var buf bytes.Buffer
	if err := PrintStreamEvent(&buf, msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "[ROLE_AGENT] streaming response") {
		t.Errorf("unexpected output:\n%s", got)
	}
}

func TestPrintStreamEvent_StatusUpdate_NoMessage(t *testing.T) {
	event := &a2a.TaskStatusUpdateEvent{
		TaskID:    "task-1",
		ContextID: "ctx-1",
		Status: a2a.TaskStatus{
			State: a2a.TaskStateCompleted,
		},
	}

	var buf bytes.Buffer
	if err := PrintStreamEvent(&buf, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "[status] TASK_STATE_COMPLETED") {
		t.Errorf("unexpected output:\n%s", got)
	}
	// Should not contain " - " separator when there's no message
	if strings.Contains(got, " - ") {
		t.Errorf("unexpected separator in output without message:\n%s", got)
	}
}

func TestPrintStreamEvent_StatusUpdate_WithMessage(t *testing.T) {
	event := &a2a.TaskStatusUpdateEvent{
		TaskID:    "task-1",
		ContextID: "ctx-1",
		Status: a2a.TaskStatus{
			State: a2a.TaskStateWorking,
			Message: &a2a.Message{
				Role:  a2a.MessageRoleAgent,
				Parts: a2a.ContentParts{a2a.NewTextPart("Processing your request")},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintStreamEvent(&buf, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"[status] TASK_STATE_WORKING",
		" - ",
		"Processing your request",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintStreamEvent_ArtifactUpdate_NewWithName(t *testing.T) {
	event := &a2a.TaskArtifactUpdateEvent{
		TaskID:    "task-1",
		ContextID: "ctx-1",
		Append:    false,
		Artifact: &a2a.Artifact{
			Name:  "output.txt",
			Parts: a2a.ContentParts{a2a.NewTextPart("file content")},
		},
	}

	var buf bytes.Buffer
	if err := PrintStreamEvent(&buf, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"[artifact] output.txt",
		"file content",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintStreamEvent_ArtifactUpdate_Append(t *testing.T) {
	event := &a2a.TaskArtifactUpdateEvent{
		TaskID:    "task-1",
		ContextID: "ctx-1",
		Append:    true,
		Artifact: &a2a.Artifact{
			Name:  "output.txt",
			Parts: a2a.ContentParts{a2a.NewTextPart("appended chunk")},
		},
	}

	var buf bytes.Buffer
	if err := PrintStreamEvent(&buf, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	// When appending, the artifact name header should NOT be printed
	if strings.Contains(got, "[artifact]") {
		t.Errorf("unexpected artifact header for append:\n%s", got)
	}
	if !strings.Contains(got, "appended chunk") {
		t.Errorf("missing content in output:\n%s", got)
	}
}

func TestPrintStreamEvent_ArtifactUpdate_NilArtifact(t *testing.T) {
	event := &a2a.TaskArtifactUpdateEvent{
		TaskID:    "task-1",
		ContextID: "ctx-1",
		Artifact:  nil,
	}

	var buf bytes.Buffer
	if err := PrintStreamEvent(&buf, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if got != "" {
		t.Errorf("expected empty output for nil artifact, got:\n%s", got)
	}
}

func TestPrintStreamEvent_ArtifactUpdate_NoNameNoAppend(t *testing.T) {
	event := &a2a.TaskArtifactUpdateEvent{
		TaskID:    "task-1",
		ContextID: "ctx-1",
		Append:    false,
		Artifact: &a2a.Artifact{
			Parts: a2a.ContentParts{a2a.NewTextPart("content only")},
		},
	}

	var buf bytes.Buffer
	if err := PrintStreamEvent(&buf, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	// No artifact header because Name is empty
	if strings.Contains(got, "[artifact]") {
		t.Errorf("unexpected artifact header for nameless artifact:\n%s", got)
	}
	if !strings.Contains(got, "content only") {
		t.Errorf("missing content in output:\n%s", got)
	}
}
