package vertexai

import (
	"encoding/json"
	"strings"
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

func TestBuildStreamRequest_OmitsConfiguration(t *testing.T) {
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-001"

	req := buildStreamRequest(msg)

	if req.Configuration != nil {
		t.Errorf("stream request should omit Configuration, got %+v", req.Configuration)
	}
	if req.Message.MessageID != "msg-001" {
		t.Errorf("messageId: got %q, want %q", req.Message.MessageID, "msg-001")
	}
	if req.Message.Role != "ROLE_USER" {
		t.Errorf("role: got %q, want %q", req.Message.Role, "ROLE_USER")
	}
}

func TestBuildStreamRequest_GeneratesMessageID(t *testing.T) {
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	// msg.ID is empty from NewMessage

	req := buildStreamRequest(msg)

	if req.Message.MessageID == "" {
		t.Error("messageId should be auto-generated when empty")
	}
}

func TestToA2AEvent_Task(t *testing.T) {
	wse := &wireStreamEvent{
		Task: &wireTask{
			ID:        "task-1",
			ContextID: "ctx-1",
			Status:    wireStatus{State: "TASK_STATE_COMPLETED"},
		},
	}

	event := toA2AEvent(wse)
	task, ok := event.(*a2a.Task)
	if !ok {
		t.Fatalf("got %T, want *a2a.Task", event)
	}
	if task.ID != "task-1" {
		t.Errorf("ID: got %q, want %q", task.ID, "task-1")
	}
	if task.Status.State != a2a.TaskStateCompleted {
		t.Errorf("State: got %q, want %q", task.Status.State, a2a.TaskStateCompleted)
	}
}

func TestToA2AEvent_Message(t *testing.T) {
	wse := &wireStreamEvent{
		Msg: &wireMessage{
			MessageID: "msg-1",
			Role:      "ROLE_AGENT",
			Content:   []*a2a.Part{a2a.NewTextPart("hello")},
		},
	}

	event := toA2AEvent(wse)
	msg, ok := event.(*a2a.Message)
	if !ok {
		t.Fatalf("got %T, want *a2a.Message", event)
	}
	if msg.ID != "msg-1" {
		t.Errorf("ID: got %q, want %q", msg.ID, "msg-1")
	}
	if msg.Role != a2a.MessageRoleAgent {
		t.Errorf("Role: got %q, want %q", msg.Role, a2a.MessageRoleAgent)
	}
}

func TestToA2AEvent_StatusUpdate(t *testing.T) {
	wse := &wireStreamEvent{
		StatusUpdate: &wireStatusUpdateEvent{
			TaskID:    "task-1",
			ContextID: "ctx-1",
			Status: wireStatus{
				State: "TASK_STATE_WORKING",
				Message: &wireMessage{
					MessageID: "status-msg",
					Role:      "ROLE_AGENT",
					Content:   []*a2a.Part{a2a.NewTextPart("in progress")},
				},
			},
			Final:    false,
			Metadata: map[string]any{"k": "v"},
		},
	}

	event := toA2AEvent(wse)
	su, ok := event.(*a2a.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("got %T, want *a2a.TaskStatusUpdateEvent", event)
	}
	if su.TaskID != "task-1" {
		t.Errorf("TaskID: got %q, want %q", su.TaskID, "task-1")
	}
	if su.ContextID != "ctx-1" {
		t.Errorf("ContextID: got %q, want %q", su.ContextID, "ctx-1")
	}
	if su.Status.State != a2a.TaskStateWorking {
		t.Errorf("Status.State: got %q, want %q", su.Status.State, a2a.TaskStateWorking)
	}
	if su.Status.Message == nil {
		t.Fatal("Status.Message should be converted, got nil")
	}
	if su.Status.Message.ID != "status-msg" {
		t.Errorf("Status.Message.ID: got %q, want %q", su.Status.Message.ID, "status-msg")
	}
	if got := su.Metadata["k"]; got != "v" {
		t.Errorf("Metadata[k]: got %v, want %q", got, "v")
	}
}

func TestToA2AEvent_ArtifactUpdate(t *testing.T) {
	wse := &wireStreamEvent{
		ArtifactUpdate: &wireArtifactUpdateEvent{
			TaskID:    "task-1",
			ContextID: "ctx-1",
			Artifact: &wireArtifact{
				ArtifactID:  "art-1",
				Name:        "result",
				Description: "the result",
				Parts:       []*a2a.Part{a2a.NewTextPart("chunk1")},
			},
			Append:    true,
			LastChunk: false,
		},
	}

	event := toA2AEvent(wse)
	au, ok := event.(*a2a.TaskArtifactUpdateEvent)
	if !ok {
		t.Fatalf("got %T, want *a2a.TaskArtifactUpdateEvent", event)
	}
	if au.TaskID != "task-1" {
		t.Errorf("TaskID: got %q, want %q", au.TaskID, "task-1")
	}
	if !au.Append {
		t.Error("Append: got false, want true")
	}
	if au.LastChunk {
		t.Error("LastChunk: got true, want false")
	}
	if au.Artifact == nil {
		t.Fatal("Artifact is nil")
	}
	if au.Artifact.ID != "art-1" {
		t.Errorf("Artifact.ID: got %q, want %q", au.Artifact.ID, "art-1")
	}
	if au.Artifact.Name != "result" {
		t.Errorf("Artifact.Name: got %q, want %q", au.Artifact.Name, "result")
	}
	if au.Artifact.Description != "the result" {
		t.Errorf("Artifact.Description: got %q, want %q", au.Artifact.Description, "the result")
	}
	if len(au.Artifact.Parts) != 1 {
		t.Fatalf("Artifact.Parts: got %d, want 1", len(au.Artifact.Parts))
	}
}

func TestToA2AEvent_Empty(t *testing.T) {
	wse := &wireStreamEvent{}
	if event := toA2AEvent(wse); event != nil {
		t.Errorf("empty event should return nil, got %T", event)
	}
}

func TestParseStreamEvent_ValidTask(t *testing.T) {
	data := []byte(`{"task":{"id":"task-1","status":{"state":"TASK_STATE_COMPLETED"}}}`)
	event, err := parseStreamEvent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	task, ok := event.(*a2a.Task)
	if !ok {
		t.Fatalf("got %T, want *a2a.Task", event)
	}
	if task.ID != "task-1" {
		t.Errorf("ID: got %q, want %q", task.ID, "task-1")
	}
}

func TestParseStreamEvent_InvalidJSON(t *testing.T) {
	_, err := parseStreamEvent([]byte(`{not valid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode stream event") {
		t.Errorf("error should mention decode failure: %v", err)
	}
}

func TestParseStreamEvent_UnknownVariant(t *testing.T) {
	event, err := parseStreamEvent([]byte(`{"someFutureField":{"foo":"bar"}}`))
	if err != nil {
		t.Fatalf("unknown variant should not error, got: %v", err)
	}
	if event != nil {
		t.Errorf("unknown variant should return nil, got %T", event)
	}
}

func TestParseStreamEvent_EmptyObject(t *testing.T) {
	event, err := parseStreamEvent([]byte(`{}`))
	if err != nil {
		t.Fatalf("empty object should not error, got: %v", err)
	}
	if event != nil {
		t.Errorf("empty object should return nil, got %T", event)
	}
}
