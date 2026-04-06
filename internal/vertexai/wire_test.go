package vertexai

import (
	"encoding/json"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestBuildSendRequest_TextMessage(t *testing.T) {
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-001"

	req := buildSendRequest(msg)

	if req.Message.MessageID != "msg-001" {
		t.Errorf("messageId: got %q, want %q", req.Message.MessageID, "msg-001")
	}
	if req.Message.Role != "ROLE_USER" {
		t.Errorf("role: got %q, want %q", req.Message.Role, "ROLE_USER")
	}
	if len(req.Message.Content) != 1 {
		t.Fatalf("content length: got %d, want 1", len(req.Message.Content))
	}
	if req.Configuration == nil || !req.Configuration.Blocking {
		t.Error("configuration.blocking should be true")
	}
}

func TestBuildSendRequest_GeneratesMessageID(t *testing.T) {
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	// msg.ID is empty by default from NewMessage

	req := buildSendRequest(msg)

	if req.Message.MessageID == "" {
		t.Error("messageId should be auto-generated when empty")
	}
}

func TestBuildSendRequest_JSONFormat(t *testing.T) {
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-001"

	req := buildSendRequest(msg)

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Verify the JSON uses "content" (not "parts") and proper role format.
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	msgRaw, ok := raw["message"].(map[string]any)
	if !ok {
		t.Fatal("missing 'message' field in JSON")
	}

	if _, ok := msgRaw["content"]; !ok {
		t.Error("JSON should use 'content' field, not 'parts'")
	}
	if _, ok := msgRaw["parts"]; ok {
		t.Error("JSON should NOT have 'parts' field")
	}
	if msgRaw["role"] != "ROLE_USER" {
		t.Errorf("role: got %v, want ROLE_USER", msgRaw["role"])
	}
}

func TestToA2ATask_BasicConversion(t *testing.T) {
	wt := wireTask{
		ID:        "task-123",
		ContextID: "ctx-456",
		Status: wireStatus{
			State: "TASK_STATE_COMPLETED",
		},
		History: []wireMessage{
			{
				MessageID: "msg-001",
				Role:      "ROLE_USER",
				Content:   []*a2a.Part{a2a.NewTextPart("hello")},
			},
			{
				MessageID: "msg-002",
				Role:      "ROLE_AGENT",
				Content:   []*a2a.Part{a2a.NewTextPart("hi there")},
			},
		},
		Artifacts: []wireArtifact{
			{
				ArtifactID: "art-001",
				Parts:      []*a2a.Part{a2a.NewTextPart("result text")},
			},
		},
	}

	task := toA2ATask(wt)

	if string(task.ID) != "task-123" {
		t.Errorf("task ID: got %q, want %q", task.ID, "task-123")
	}
	if task.ContextID != "ctx-456" {
		t.Errorf("contextID: got %q, want %q", task.ContextID, "ctx-456")
	}
	if task.Status.State != a2a.TaskStateCompleted {
		t.Errorf("state: got %q, want %q", task.Status.State, a2a.TaskStateCompleted)
	}
	if len(task.History) != 2 {
		t.Fatalf("history length: got %d, want 2", len(task.History))
	}
	if task.History[0].Role != a2a.MessageRoleUser {
		t.Errorf("history[0].role: got %q, want %q", task.History[0].Role, a2a.MessageRoleUser)
	}
	if task.History[1].Role != a2a.MessageRoleAgent {
		t.Errorf("history[1].role: got %q, want %q", task.History[1].Role, a2a.MessageRoleAgent)
	}
	if len(task.Artifacts) != 1 {
		t.Fatalf("artifacts length: got %d, want 1", len(task.Artifacts))
	}
	if string(task.Artifacts[0].ID) != "art-001" {
		t.Errorf("artifact ID: got %q, want %q", task.Artifacts[0].ID, "art-001")
	}
}

func TestToA2ATask_WithStatusMessage(t *testing.T) {
	wt := wireTask{
		ID:        "task-001",
		ContextID: "ctx-001",
		Status: wireStatus{
			State: "TASK_STATE_FAILED",
			Message: &wireMessage{
				MessageID: "status-msg",
				Role:      "ROLE_AGENT",
				Content:   []*a2a.Part{a2a.NewTextPart("something went wrong")},
			},
		},
	}

	task := toA2ATask(wt)

	if task.Status.Message == nil {
		t.Fatal("status message should not be nil")
	}
	if task.Status.Message.ID != "status-msg" {
		t.Errorf("status message ID: got %q, want %q", task.Status.Message.ID, "status-msg")
	}
}

func TestWireMessageToA2A(t *testing.T) {
	wm := &wireMessage{
		MessageID: "msg-123",
		Role:      "ROLE_USER",
		Content:   []*a2a.Part{a2a.NewTextPart("test")},
		ContextID: "ctx-001",
	}

	msg := wireMessageToA2A(wm)

	if msg.ID != "msg-123" {
		t.Errorf("ID: got %q, want %q", msg.ID, "msg-123")
	}
	if msg.Role != a2a.MessageRoleUser {
		t.Errorf("role: got %q, want %q", msg.Role, a2a.MessageRoleUser)
	}
	if len(msg.Parts) != 1 {
		t.Fatalf("parts length: got %d, want 1", len(msg.Parts))
	}
	if msg.ContextID != "ctx-001" {
		t.Errorf("contextID: got %q, want %q", msg.ContextID, "ctx-001")
	}
}
