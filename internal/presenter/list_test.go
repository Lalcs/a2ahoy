package presenter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestPrintListTasks_EmptyList(t *testing.T) {
	resp := &a2a.ListTasksResponse{
		Tasks: nil,
	}

	var buf bytes.Buffer
	if err := PrintListTasks(&buf, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "No tasks found.") {
		t.Errorf("expected 'No tasks found.' in output:\n%s", got)
	}
}

func TestPrintListTasks_SingleTask(t *testing.T) {
	resp := &a2a.ListTasksResponse{
		Tasks: []*a2a.Task{
			{
				ID:        "task-001",
				ContextID: "ctx-abc",
				Status:    a2a.TaskStatus{State: a2a.TaskStateCompleted},
			},
		},
		TotalSize: 1,
	}

	var buf bytes.Buffer
	if err := PrintListTasks(&buf, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"Tasks (1 of 1 total)",
		"task-001",
		"ctx-abc",
		"TASK_STATE_COMPLETED",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintListTasks_MultipleTasksWithPagination(t *testing.T) {
	resp := &a2a.ListTasksResponse{
		Tasks: []*a2a.Task{
			{
				ID:        "task-001",
				ContextID: "ctx-abc",
				Status:    a2a.TaskStatus{State: a2a.TaskStateCompleted},
			},
			{
				ID:        "task-002",
				ContextID: "ctx-def",
				Status:    a2a.TaskStatus{State: a2a.TaskStateWorking},
			},
			{
				ID:        "task-003",
				ContextID: "ctx-ghi",
				Status:    a2a.TaskStatus{State: a2a.TaskStateFailed},
			},
		},
		TotalSize:     12,
		NextPageToken: "next-page-abc",
	}

	var buf bytes.Buffer
	if err := PrintListTasks(&buf, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"Tasks (3 of 12 total)",
		"task-001",
		"task-002",
		"task-003",
		"TASK_STATE_COMPLETED",
		"TASK_STATE_WORKING",
		"TASK_STATE_FAILED",
		"Next page:",
		"next-page-abc",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintListTasks_NoNextPageToken(t *testing.T) {
	resp := &a2a.ListTasksResponse{
		Tasks: []*a2a.Task{
			{
				ID:        "task-001",
				ContextID: "ctx-abc",
				Status:    a2a.TaskStatus{State: a2a.TaskStateCompleted},
			},
		},
		TotalSize:     1,
		NextPageToken: "",
	}

	var buf bytes.Buffer
	if err := PrintListTasks(&buf, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "Next page:") {
		t.Errorf("should not show 'Next page:' when no token:\n%s", got)
	}
}
