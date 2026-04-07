package vertexai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	ep := &Endpoint{base: server.URL}
	c := NewClient(ep, func() (string, error) {
		return "test-token", nil
	})
	return c, server
}

func TestClient_FetchCard(t *testing.T) {
	card := a2a.AgentCard{
		Name:        "test-agent",
		Description: "A test agent",
		Version:     "1.0.0",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("authorization: got %q, want %q", auth, "Bearer test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	got, err := c.FetchCard(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "test-agent" {
		t.Errorf("name: got %q, want %q", got.Name, "test-agent")
	}
}

func TestClient_FetchCard_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found")
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	_, err := c.FetchCard(context.Background())
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestClient_SendMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/message:send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify request body format.
		var req sendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Message.Role != "ROLE_USER" {
			t.Errorf("role: got %q, want ROLE_USER", req.Message.Role)
		}
		if req.Configuration == nil || !req.Configuration.Blocking {
			t.Error("expected blocking: true")
		}

		// Return a Vertex AI response.
		resp := sendResponse{
			Task: wireTask{
				ID:        "task-001",
				ContextID: "ctx-001",
				Status: wireStatus{
					State: "TASK_STATE_COMPLETED",
				},
				Artifacts: []wireArtifact{
					{
						ArtifactID: "art-001",
						Parts:      []*a2a.Part{a2a.NewTextPart("response text")},
					},
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
						Content:   []*a2a.Part{a2a.NewTextPart("response text")},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-001"
	result, err := c.SendMessage(context.Background(), &a2a.SendMessageRequest{Message: msg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, ok := result.(*a2a.Task)
	if !ok {
		t.Fatalf("expected *a2a.Task, got %T", result)
	}
	if string(task.ID) != "task-001" {
		t.Errorf("task ID: got %q, want %q", task.ID, "task-001")
	}
	if task.Status.State != a2a.TaskStateCompleted {
		t.Errorf("state: got %q, want %q", task.Status.State, a2a.TaskStateCompleted)
	}
	if len(task.Artifacts) != 1 {
		t.Fatalf("artifacts: got %d, want 1", len(task.Artifacts))
	}
	if len(task.History) != 2 {
		t.Fatalf("history: got %d, want 2", len(task.History))
	}
}

func TestClient_SendMessage_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/message:send", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error": "bad request"}`)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	_, err := c.SendMessage(context.Background(), &a2a.SendMessageRequest{Message: msg})
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should mention 400: %v", err)
	}
}

func TestClient_SendStreamingMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/message:stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		events := []sendResponse{
			{
				Task: wireTask{
					ID:        "task-001",
					ContextID: "ctx-001",
					Status:    wireStatus{State: "TASK_STATE_WORKING"},
				},
			},
			{
				Task: wireTask{
					ID:        "task-001",
					ContextID: "ctx-001",
					Status:    wireStatus{State: "TASK_STATE_COMPLETED"},
					Artifacts: []wireArtifact{
						{
							ArtifactID: "art-001",
							Parts:      []*a2a.Part{a2a.NewTextPart("streamed result")},
						},
					},
				},
			},
		}

		for _, evt := range events {
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("stream test"))
	msg.ID = "msg-001"

	var events []a2a.Event
	for event, err := range c.SendStreamingMessage(context.Background(), &a2a.SendMessageRequest{Message: msg}) {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, event)
	}

	if len(events) != 2 {
		t.Fatalf("events: got %d, want 2", len(events))
	}

	// First event: working state.
	task1, ok := events[0].(*a2a.Task)
	if !ok {
		t.Fatalf("event[0]: expected *a2a.Task, got %T", events[0])
	}
	if task1.Status.State != a2a.TaskStateWorking {
		t.Errorf("event[0] state: got %q, want %q", task1.Status.State, a2a.TaskStateWorking)
	}

	// Second event: completed with artifact.
	task2, ok := events[1].(*a2a.Task)
	if !ok {
		t.Fatalf("event[1]: expected *a2a.Task, got %T", events[1])
	}
	if task2.Status.State != a2a.TaskStateCompleted {
		t.Errorf("event[1] state: got %q, want %q", task2.Status.State, a2a.TaskStateCompleted)
	}
	if len(task2.Artifacts) != 1 {
		t.Fatalf("event[1] artifacts: got %d, want 1", len(task2.Artifacts))
	}
}

func TestClient_TokenError(t *testing.T) {
	ep := &Endpoint{base: "https://example.com"}
	c := NewClient(ep, func() (string, error) {
		return "", fmt.Errorf("token error")
	})

	_, err := c.FetchCard(context.Background())
	if err == nil {
		t.Fatal("expected error when token fails")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error should mention token: %v", err)
	}
}

func TestClient_FetchCard_WithExtraHeaders(t *testing.T) {
	var captured http.Header
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a2a.AgentCard{Name: "test-agent"})
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	c.SetExtraHeaders([]HeaderEntry{
		{Key: "X-Tenant-ID", Value: "tenant-1"},
		{Key: "X-Custom-Auth", Value: "secret"},
	})

	if _, err := c.FetchCard(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := captured.Get("X-Tenant-ID"); got != "tenant-1" {
		t.Errorf("X-Tenant-ID: got %q, want %q", got, "tenant-1")
	}
	if got := captured.Get("X-Custom-Auth"); got != "secret" {
		t.Errorf("X-Custom-Auth: got %q, want %q", got, "secret")
	}
	// extraHeaders are applied after Authorization; non-conflicting entries
	// must leave the existing Bearer token intact.
	if got := captured.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer test-token")
	}
}

// TestClient_NewRequest_HeaderOverridesAuth verifies that --header can
// intentionally override the Authorization header in the Vertex AI client.
// This is a deliberate design choice (Set, not Add) so users can use
// custom auth tokens in test environments.
func TestClient_NewRequest_HeaderOverridesAuth(t *testing.T) {
	var captured http.Header
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a2a.AgentCard{Name: "test-agent"})
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	c.SetExtraHeaders([]HeaderEntry{
		{Key: "Authorization", Value: "Bearer override"},
	})

	if _, err := c.FetchCard(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := captured.Get("Authorization"); got != "Bearer override" {
		t.Errorf("Authorization should be overridden: got %q, want %q", got, "Bearer override")
	}
}

func TestClient_GetTask(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/tasks/task-001", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("authorization: got %q, want %q", got, "Bearer test-token")
		}
		resp := wireTask{
			ID:        "task-001",
			ContextID: "ctx-001",
			Status:    wireStatus{State: "TASK_STATE_COMPLETED"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	task, err := c.GetTask(context.Background(), &a2a.GetTaskRequest{
		ID: a2a.TaskID("task-001"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(task.ID) != "task-001" {
		t.Errorf("ID: got %q, want %q", task.ID, "task-001")
	}
	if task.ContextID != "ctx-001" {
		t.Errorf("ContextID: got %q, want %q", task.ContextID, "ctx-001")
	}
	if task.Status.State != a2a.TaskStateCompleted {
		t.Errorf("Status.State: got %q, want %q", task.Status.State, a2a.TaskStateCompleted)
	}
}

func TestClient_GetTask_HistoryLength(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/tasks/task-001", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(wireTask{ID: "task-001"})
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	h := 5
	if _, err := c.GetTask(context.Background(), &a2a.GetTaskRequest{
		ID:            a2a.TaskID("task-001"),
		HistoryLength: &h,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQuery != "historyLength=5" {
		t.Errorf("query: got %q, want %q", gotQuery, "historyLength=5")
	}
}

func TestClient_GetTask_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/tasks/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "task not found")
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	_, err := c.GetTask(context.Background(), &a2a.GetTaskRequest{
		ID: a2a.TaskID("missing"),
	})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestClient_CancelTask(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/tasks/task-001:cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("authorization: got %q, want %q", got, "Bearer test-token")
		}
		if r.ContentLength != 0 {
			t.Errorf("expected empty body, ContentLength=%d", r.ContentLength)
		}
		resp := wireTask{
			ID:        "task-001",
			ContextID: "ctx-001",
			Status:    wireStatus{State: "TASK_STATE_CANCELED"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	task, err := c.CancelTask(context.Background(), &a2a.CancelTaskRequest{
		ID: a2a.TaskID("task-001"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(task.ID) != "task-001" {
		t.Errorf("ID: got %q, want %q", task.ID, "task-001")
	}
	if task.ContextID != "ctx-001" {
		t.Errorf("ContextID: got %q, want %q", task.ContextID, "ctx-001")
	}
	if task.Status.State != a2a.TaskStateCanceled {
		t.Errorf("Status.State: got %q, want %q", task.Status.State, a2a.TaskStateCanceled)
	}
}

func TestClient_CancelTask_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/tasks/missing:cancel", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "FAILED_PRECONDITION: TASK_NOT_CANCELABLE")
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	_, err := c.CancelTask(context.Background(), &a2a.CancelTaskRequest{
		ID: a2a.TaskID("missing"),
	})
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should mention 400: %v", err)
	}
}
