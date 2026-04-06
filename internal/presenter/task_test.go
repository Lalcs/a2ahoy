package presenter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestPrintSendResult_Task(t *testing.T) {
	task := &a2a.Task{
		ID:        "task-123",
		ContextID: "ctx-456",
		Status: a2a.TaskStatus{
			State: a2a.TaskStateCompleted,
		},
	}

	var buf bytes.Buffer
	if err := PrintSendResult(&buf, task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"=== Task ===",
		"ID:        task-123",
		"ContextID: ctx-456",
		"Status:    TASK_STATE_COMPLETED",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintSendResult_TaskWithTimestamp(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	task := &a2a.Task{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status: a2a.TaskStatus{
			State:     a2a.TaskStateWorking,
			Timestamp: &ts,
		},
	}

	var buf bytes.Buffer
	if err := PrintSendResult(&buf, task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Timestamp: 2025-06-15T10:30:00Z") {
		t.Errorf("missing timestamp in output:\n%s", got)
	}
}

func TestPrintSendResult_TaskWithStatusMessage(t *testing.T) {
	task := &a2a.Task{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status: a2a.TaskStatus{
			State: a2a.TaskStateCompleted,
			Message: &a2a.Message{
				Role:  a2a.MessageRoleAgent,
				Parts: a2a.ContentParts{a2a.NewTextPart("Task completed successfully")},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintSendResult(&buf, task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"--- Status Message ---",
		"Task completed successfully",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintSendResult_TaskWithHistory(t *testing.T) {
	task := &a2a.Task{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status: a2a.TaskStatus{
			State: a2a.TaskStateCompleted,
		},
		History: []*a2a.Message{
			{
				Role:  a2a.MessageRoleUser,
				Parts: a2a.ContentParts{a2a.NewTextPart("Hello")},
			},
			{
				Role:  a2a.MessageRoleAgent,
				Parts: a2a.ContentParts{a2a.NewTextPart("Hi there!")},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintSendResult(&buf, task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"--- History (2 messages) ---",
		"[ROLE_USER] Hello",
		"[ROLE_AGENT] Hi there!",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintSendResult_TaskWithArtifacts(t *testing.T) {
	task := &a2a.Task{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status: a2a.TaskStatus{
			State: a2a.TaskStateCompleted,
		},
		Artifacts: []*a2a.Artifact{
			{
				Name:        "report.txt",
				Description: "Generated report",
				Parts:       a2a.ContentParts{a2a.NewTextPart("Report content here")},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintSendResult(&buf, task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"--- Artifacts (1) ---",
		"Name: report.txt",
		"Description: Generated report",
		"Report content here",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintSendResult_Message(t *testing.T) {
	msg := &a2a.Message{
		Role:  a2a.MessageRoleAgent,
		Parts: a2a.ContentParts{a2a.NewTextPart("Hello from agent")},
	}

	var buf bytes.Buffer
	if err := PrintSendResult(&buf, msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "[ROLE_AGENT] Hello from agent") {
		t.Errorf("unexpected output:\n%s", got)
	}
}

func TestPrintParts_TextPart(t *testing.T) {
	parts := a2a.ContentParts{a2a.NewTextPart("hello world")}

	var buf bytes.Buffer
	printParts(&buf, parts)

	got := buf.String()
	if !strings.Contains(got, "hello world") {
		t.Errorf("missing text in output:\n%s", got)
	}
}

func TestPrintParts_DataPart(t *testing.T) {
	parts := a2a.ContentParts{a2a.NewDataPart(map[string]any{"key": "value"})}

	var buf bytes.Buffer
	printParts(&buf, parts)

	got := buf.String()
	if !strings.Contains(got, "[data]") {
		t.Errorf("missing data marker in output:\n%s", got)
	}
}

func TestPrintParts_URLPart(t *testing.T) {
	parts := a2a.ContentParts{a2a.NewFileURLPart("https://example.com/file.txt", "text/plain")}

	var buf bytes.Buffer
	printParts(&buf, parts)

	got := buf.String()
	if !strings.Contains(got, "[url] https://example.com/file.txt") {
		t.Errorf("missing URL in output:\n%s", got)
	}
}

func TestPrintParts_RawPart(t *testing.T) {
	data := []byte("binary data here")
	parts := a2a.ContentParts{a2a.NewRawPart(data)}

	var buf bytes.Buffer
	printParts(&buf, parts)

	got := buf.String()
	expected := "[raw bytes: 16 bytes]"
	if !strings.Contains(got, expected) {
		t.Errorf("missing %q in output:\n%s", expected, got)
	}
}

func TestPrintParts_UnknownPart(t *testing.T) {
	// Part with nil Content hits the default case
	parts := a2a.ContentParts{{Content: nil}}

	var buf bytes.Buffer
	printParts(&buf, parts)

	got := buf.String()
	if !strings.Contains(got, "[unknown part type]") {
		t.Errorf("missing unknown part marker in output:\n%s", got)
	}
}

func TestPrintParts_MultipleParts(t *testing.T) {
	parts := a2a.ContentParts{
		a2a.NewTextPart("first"),
		a2a.NewTextPart("second"),
	}

	var buf bytes.Buffer
	printParts(&buf, parts)

	got := buf.String()
	if !strings.Contains(got, "first") || !strings.Contains(got, "second") {
		t.Errorf("missing parts in output:\n%s", got)
	}
}

func TestPrintSendResult_ArtifactWithoutNameAndDescription(t *testing.T) {
	task := &a2a.Task{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status:    a2a.TaskStatus{State: a2a.TaskStateCompleted},
		Artifacts: []*a2a.Artifact{
			{
				Parts: a2a.ContentParts{a2a.NewTextPart("artifact content")},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintSendResult(&buf, task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "Name:") {
		t.Errorf("unexpected Name field in output:\n%s", got)
	}
	if strings.Contains(got, "Description:") {
		t.Errorf("unexpected Description field in output:\n%s", got)
	}
	if !strings.Contains(got, "artifact content") {
		t.Errorf("missing artifact content in output:\n%s", got)
	}
}
