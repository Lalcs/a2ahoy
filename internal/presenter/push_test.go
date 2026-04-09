package presenter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestPrintTaskPushConfig_Full(t *testing.T) {
	config := &a2a.TaskPushConfig{
		TaskID: "task-123",
		Tenant: "tenant-abc",
		Config: a2a.PushConfig{
			ID:    "config-456",
			URL:   "https://example.com/notify",
			Token: "secret-token",
			Auth: &a2a.PushAuthInfo{
				Scheme:      "Bearer",
				Credentials: "my-cred",
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintTaskPushConfig(&buf, config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"Push Notification Config",
		"task-123",
		"config-456",
		"https://example.com/notify",
		"secret-token",
		"Bearer",
		"my-cred",
		"tenant-abc",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintTaskPushConfig_Minimal(t *testing.T) {
	config := &a2a.TaskPushConfig{
		TaskID: "task-789",
		Config: a2a.PushConfig{
			URL: "https://example.com/hook",
		},
	}

	var buf bytes.Buffer
	if err := PrintTaskPushConfig(&buf, config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "task-789") {
		t.Errorf("missing task ID in output:\n%s", got)
	}
	if !strings.Contains(got, "https://example.com/hook") {
		t.Errorf("missing URL in output:\n%s", got)
	}
	// Optional fields should not appear
	if strings.Contains(got, "Token:") {
		t.Errorf("should not show Token when empty:\n%s", got)
	}
	if strings.Contains(got, "Auth:") {
		t.Errorf("should not show Auth when nil:\n%s", got)
	}
	if strings.Contains(got, "Tenant:") {
		t.Errorf("should not show Tenant when empty:\n%s", got)
	}
}

func TestPrintTaskPushConfigs_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintTaskPushConfigs(&buf, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "No push notification configs found.") {
		t.Errorf("expected empty message in output:\n%s", got)
	}
}

func TestPrintTaskPushConfigs_Multiple(t *testing.T) {
	configs := []*a2a.TaskPushConfig{
		{
			TaskID: "task-001",
			Config: a2a.PushConfig{
				ID:  "cfg-a",
				URL: "https://example.com/a",
			},
		},
		{
			TaskID: "task-001",
			Config: a2a.PushConfig{
				ID:  "cfg-b",
				URL: "https://example.com/b",
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintTaskPushConfigs(&buf, configs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"Push Notification Configs (2)",
		"cfg-a",
		"cfg-b",
		"https://example.com/a",
		"https://example.com/b",
		"task-001",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintTaskPushConfigs_NoID(t *testing.T) {
	configs := []*a2a.TaskPushConfig{
		{
			TaskID: "task-001",
			Config: a2a.PushConfig{
				URL: "https://example.com/hook",
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintTaskPushConfigs(&buf, configs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "(none)") {
		t.Errorf("expected '(none)' for empty config ID:\n%s", got)
	}
}

func TestPrintTaskPushConfigDeleted(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintTaskPushConfigDeleted(&buf, "task-123", "config-456"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "config-456") {
		t.Errorf("missing config ID in output:\n%s", got)
	}
	if !strings.Contains(got, "task-123") {
		t.Errorf("missing task ID in output:\n%s", got)
	}
	if !strings.Contains(got, "OK") {
		t.Errorf("missing OK marker in output:\n%s", got)
	}
}
