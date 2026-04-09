package vertexai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	}, nil)
	// Preset the card so tests that invoke SendMessage/GetTask/
	// CancelTask/SendStreamingMessage directly (without going through
	// FetchCard first) route to the test server's /a2a/v1/* routes.
	// FetchCard-based tests overwrite c.card with the server response.
	c.card = &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			{URL: server.URL + "/a2a/v1"},
		},
	}
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
		_ = json.NewEncoder(w).Encode(card)
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
		_, _ = fmt.Fprint(w, "not found")
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

// TestClient_FetchCard_V03Format verifies that the Vertex AI client correctly
// decodes A2A v0.3-format agent cards. Vertex AI Agent Engine emits cards
// with root-level url/preferredTransport/protocolVersion fields rather than
// the v1.0 SupportedInterfaces array; the a2av0 compat parser promotes
// these into a single-entry SupportedInterfaces[0] (verified here).
func TestClient_FetchCard_V03Format(t *testing.T) {
	const cardJSON = `{
		"protocolVersion": "0.3.0",
		"name": "v03-agent",
		"description": "A v0.3 test agent",
		"version": "1.0.0",
		"url": "https://example.com/a2a",
		"preferredTransport": "HTTP+JSON",
		"capabilities": {"streaming": true},
		"defaultInputModes": ["text/plain"],
		"defaultOutputModes": ["application/json"],
		"skills": []
	}`

	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, cardJSON)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	got, err := c.FetchCard(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "v03-agent" {
		t.Errorf("name: got %q, want %q", got.Name, "v03-agent")
	}
	if len(got.SupportedInterfaces) != 1 {
		t.Fatalf("SupportedInterfaces: got %d entries, want 1", len(got.SupportedInterfaces))
	}
	iface := got.SupportedInterfaces[0]
	if iface.URL != "https://example.com/a2a" {
		t.Errorf("interface URL: got %q, want %q", iface.URL, "https://example.com/a2a")
	}
	if iface.ProtocolBinding != a2a.TransportProtocolHTTPJSON {
		t.Errorf("ProtocolBinding: got %q, want %q", iface.ProtocolBinding, a2a.TransportProtocolHTTPJSON)
	}
	if string(iface.ProtocolVersion) != "0.3.0" {
		t.Errorf("ProtocolVersion: got %q, want %q", iface.ProtocolVersion, "0.3.0")
	}
	if !got.Capabilities.Streaming {
		t.Error("Capabilities.Streaming: got false, want true")
	}
}

// TestClient_FetchCard_V03WithAdditionalInterfaces verifies that v0.3
// additionalInterfaces are expanded into SupportedInterfaces alongside the
// root-level primary, skipping entries whose URL duplicates the primary.
func TestClient_FetchCard_V03WithAdditionalInterfaces(t *testing.T) {
	const cardJSON = `{
		"protocolVersion": "0.3.0",
		"name": "multi-transport",
		"description": "Multi-transport v0.3 agent",
		"version": "1.0.0",
		"url": "https://example.com/a2a",
		"preferredTransport": "JSONRPC",
		"additionalInterfaces": [
			{"url": "https://example.com/a2a", "transport": "JSONRPC"},
			{"url": "https://example.com/rest", "transport": "HTTP+JSON"}
		],
		"capabilities": {},
		"defaultInputModes": ["text/plain"],
		"defaultOutputModes": ["text/plain"],
		"skills": []
	}`

	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, cardJSON)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	got, err := c.FetchCard(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Primary (JSONRPC at /a2a) + rest (HTTP+JSON) = 2 entries.
	// The additionalInterfaces entry with URL https://example.com/a2a is a
	// duplicate of the primary URL and must be skipped by the compat parser.
	if len(got.SupportedInterfaces) != 2 {
		t.Fatalf("SupportedInterfaces: got %d entries, want 2", len(got.SupportedInterfaces))
	}
	if got.SupportedInterfaces[0].ProtocolBinding != a2a.TransportProtocolJSONRPC {
		t.Errorf("interfaces[0] binding: got %q, want JSONRPC", got.SupportedInterfaces[0].ProtocolBinding)
	}
	if got.SupportedInterfaces[1].URL != "https://example.com/rest" {
		t.Errorf("interfaces[1] URL: got %q, want https://example.com/rest", got.SupportedInterfaces[1].URL)
	}
	if got.SupportedInterfaces[1].ProtocolBinding != a2a.TransportProtocolHTTPJSON {
		t.Errorf("interfaces[1] binding: got %q, want HTTP+JSON", got.SupportedInterfaces[1].ProtocolBinding)
	}
}

// TestClient_FetchCard_V03SupportsExtendedCard verifies that the v0.3
// root-level supportsAuthenticatedExtendedCard field is promoted into
// Capabilities.ExtendedAgentCard by the compat parser.
func TestClient_FetchCard_V03SupportsExtendedCard(t *testing.T) {
	const cardJSON = `{
		"protocolVersion": "0.3.0",
		"name": "extended-card-agent",
		"description": "Agent with extended card",
		"version": "1.0.0",
		"url": "https://example.com/a2a",
		"preferredTransport": "HTTP+JSON",
		"supportsAuthenticatedExtendedCard": true,
		"capabilities": {"streaming": false},
		"defaultInputModes": ["text/plain"],
		"defaultOutputModes": ["text/plain"],
		"skills": []
	}`

	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, cardJSON)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	got, err := c.FetchCard(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Capabilities.ExtendedAgentCard {
		t.Error("Capabilities.ExtendedAgentCard: got false, want true (promoted from supportsAuthenticatedExtendedCard)")
	}
}

// TestClient_FetchCard_UpdatesBaseURL verifies that FetchCard stores the
// parsed card on the client so that baseURL() returns the URL the server
// advertised in SupportedInterfaces[0].URL. Subsequent SendMessage/GetTask/
// CancelTask calls will then route to that URL rather than the preset
// fixture URL from newTestClient.
func TestClient_FetchCard_UpdatesBaseURL(t *testing.T) {
	const cardJSON = `{
		"protocolVersion": "0.3.0",
		"name": "base-url-test",
		"description": "test",
		"version": "1.0.0",
		"url": "https://agent.example.com/custom/path",
		"preferredTransport": "HTTP+JSON",
		"capabilities": {},
		"defaultInputModes": ["text/plain"],
		"defaultOutputModes": ["text/plain"],
		"skills": []
	}`

	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, cardJSON)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	// Sanity check: baseURL() must differ from the target before
	// FetchCard runs, otherwise the assertion below is vacuous.
	const wantURL = "https://agent.example.com/custom/path"
	if c.baseURL() == wantURL {
		t.Fatalf("baseURL already equals target URL before FetchCard")
	}

	_, err := c.FetchCard(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.baseURL() != wantURL {
		t.Errorf("baseURL after FetchCard: got %q, want %q", c.baseURL(), wantURL)
	}
}

// TestClient_FetchCard_VertexAIFixture is a regression test using the exact
// JSON shape observed from a real Vertex AI Agent Engine (fund_researcher).
// Before A2AHOY-26 the root-level url/preferredTransport/protocolVersion
// and supportsAuthenticatedExtendedCard fields were silently dropped by a
// plain json.Unmarshal into a2a.AgentCard.
func TestClient_FetchCard_VertexAIFixture(t *testing.T) {
	const cardJSON = `{
		"protocolVersion": "0.3.0",
		"version": "1.0.0",
		"skills": [
			{
				"tags": ["Finance", "Fund", "Research"],
				"description": "投資信託の基準価額やリターン率をWebから検索・取得する",
				"name": "投資信託リサーチ",
				"id": "research_fund"
			}
		],
		"supportsAuthenticatedExtendedCard": true,
		"name": "fund-researcher",
		"capabilities": {"streaming": true},
		"url": "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/338393595527/locations/us-central1/reasoningEngines/5063154838641049600/a2a",
		"description": "投資信託リサーチ AI エージェント",
		"preferredTransport": "HTTP+JSON",
		"defaultOutputModes": ["application/json"],
		"defaultInputModes": ["text/plain"]
	}`

	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, cardJSON)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	got, err := c.FetchCard(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != "fund-researcher" {
		t.Errorf("Name: got %q, want fund-researcher", got.Name)
	}
	if len(got.SupportedInterfaces) != 1 {
		t.Fatalf("SupportedInterfaces: got %d, want 1", len(got.SupportedInterfaces))
	}
	const wantURL = "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/338393595527/locations/us-central1/reasoningEngines/5063154838641049600/a2a"
	iface := got.SupportedInterfaces[0]
	if iface.URL != wantURL {
		t.Errorf("interface URL: got %q, want %q", iface.URL, wantURL)
	}
	if iface.ProtocolBinding != a2a.TransportProtocolHTTPJSON {
		t.Errorf("ProtocolBinding: got %q, want HTTP+JSON", iface.ProtocolBinding)
	}
	if string(iface.ProtocolVersion) != "0.3.0" {
		t.Errorf("ProtocolVersion: got %q, want 0.3.0", iface.ProtocolVersion)
	}
	if !got.Capabilities.Streaming {
		t.Error("Capabilities.Streaming: got false, want true")
	}
	if !got.Capabilities.ExtendedAgentCard {
		t.Error("Capabilities.ExtendedAgentCard: got false, want true (from supportsAuthenticatedExtendedCard)")
	}
	if len(got.Skills) != 1 || got.Skills[0].ID != "research_fund" {
		t.Errorf("Skills: got %+v", got.Skills)
	}

	// baseURL() should derive the card URL.
	if c.baseURL() != wantURL {
		t.Errorf("baseURL: got %q, want %q", c.baseURL(), wantURL)
	}
}

// TestClient_BaseURL_NilSafe verifies that baseURL() returns "" without
// panicking for nil card / empty interfaces / nil first interface — the
// states the client is in before FetchCard runs or when a card response
// lacks any usable transport. URL builders propagate "" so the resulting
// request fails fast with an obvious error instead of hitting a stale URL.
func TestClient_BaseURL_NilSafe(t *testing.T) {
	tests := []struct {
		name string
		card *a2a.AgentCard
	}{
		{name: "nil card", card: nil},
		{name: "empty SupportedInterfaces", card: &a2a.AgentCard{}},
		{
			name: "nil first interface",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{nil},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{card: tc.card}
			if got := c.baseURL(); got != "" {
				t.Errorf("baseURL: got %q, want empty", got)
			}
		})
	}
}

// TestClient_CardMutationReflectedInRequestURLs is the regression guard
// for the "store card reference, derive URLs on demand" design used by
// internal/client.New to apply applyV03RESTMountPrefix to Vertex AI
// endpoints. Mutating the stored card must immediately affect subsequent
// sendURL()/streamURL()/taskURL()/cancelTaskURL() outputs; a future
// refactor that reintroduces a cached URL field would break this and be
// caught here.
func TestClient_CardMutationReflectedInRequestURLs(t *testing.T) {
	const cardJSON = `{
		"protocolVersion": "0.3.0",
		"name": "fund-researcher",
		"description": "test",
		"version": "1.0.0",
		"url": "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/1/locations/us-central1/reasoningEngines/1/a2a",
		"preferredTransport": "HTTP+JSON",
		"capabilities": {},
		"defaultInputModes": ["text/plain"],
		"defaultOutputModes": ["text/plain"],
		"skills": []
	}`

	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, cardJSON)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	card, err := c.FetchCard(context.Background())
	if err != nil {
		t.Fatalf("FetchCard: %v", err)
	}

	const preRewriteURL = "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/1/locations/us-central1/reasoningEngines/1/a2a"
	if c.baseURL() != preRewriteURL {
		t.Fatalf("baseURL after FetchCard: got %q, want %q", c.baseURL(), preRewriteURL)
	}

	// Simulate applyV03RESTMountPrefix rewriting the card URL in place.
	// internal/client.TestApplyV03RESTMountPrefix covers the rewrite rule;
	// this test covers the propagation to request URLs.
	const postRewriteURL = preRewriteURL + "/v1"
	card.SupportedInterfaces[0].URL = postRewriteURL

	if c.baseURL() != postRewriteURL {
		t.Errorf("baseURL after mutation: got %q, want %q", c.baseURL(), postRewriteURL)
	}
	wantSendURL := postRewriteURL + "/message:send"
	if got := c.sendURL(); got != wantSendURL {
		t.Errorf("sendURL() after mutation: got %q, want %q", got, wantSendURL)
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
		_ = json.NewEncoder(w).Encode(resp)
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
		_, _ = fmt.Fprint(w, `{"error": "bad request"}`)
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
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
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
	}, nil)

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
		_ = json.NewEncoder(w).Encode(a2a.AgentCard{Name: "test-agent"})
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
		_ = json.NewEncoder(w).Encode(a2a.AgentCard{Name: "test-agent"})
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
		_ = json.NewEncoder(w).Encode(resp)
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
		_ = json.NewEncoder(w).Encode(wireTask{ID: "task-001"})
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
		_, _ = fmt.Fprint(w, "task not found")
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
		_ = json.NewEncoder(w).Encode(resp)
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
		_, _ = fmt.Fprint(w, "FAILED_PRECONDITION: TASK_NOT_CANCELABLE")
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

// streamBodyHandler returns an HTTP handler that writes the given raw SSE
// body verbatim to the response (no framing or encoding applied by the
// test helper itself). Useful for simulating the exact on-the-wire output
// of sse-starlette, including multi-line data: fields and CR/LF variants.
func streamBodyHandler(t *testing.T, rawBody string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/message:stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rawBody))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	return mux
}

// collectStream drains SendStreamingMessage and returns the collected
// events plus the first error encountered (nil if none).
func collectStream(t *testing.T, c *Client) ([]a2a.Event, error) {
	t.Helper()
	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("stream test"))
	msg.ID = "msg-test"

	var events []a2a.Event
	var firstErr error
	for event, err := range c.SendStreamingMessage(context.Background(), &a2a.SendMessageRequest{Message: msg}) {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		events = append(events, event)
	}
	return events, firstErr
}

// TestClient_SendStreamingMessage_MultiLineData verifies the parser handles
// the real-world output of sse-starlette, which splits indented multi-line
// JSON into one "data:" line per source line (per the SSE spec).
func TestClient_SendStreamingMessage_MultiLineData(t *testing.T) {
	// Simulates the exact format Vertex AI sends: MessageToJson with
	// indent=2, then sse-starlette splitting each newline into its own
	// "data:" line.
	rawBody := "data: {\n" +
		"data:   \"task\": {\n" +
		"data:     \"id\": \"task-ml\",\n" +
		"data:     \"contextId\": \"ctx-ml\",\n" +
		"data:     \"status\": {\n" +
		"data:       \"state\": \"TASK_STATE_WORKING\"\n" +
		"data:     }\n" +
		"data:   }\n" +
		"data: }\n" +
		"\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	events, err := collectStream(t, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	task, ok := events[0].(*a2a.Task)
	if !ok {
		t.Fatalf("event[0]: got %T, want *a2a.Task", events[0])
	}
	if task.ID != "task-ml" {
		t.Errorf("task.ID: got %q, want %q", task.ID, "task-ml")
	}
	if task.Status.State != a2a.TaskStateWorking {
		t.Errorf("task.Status.State: got %q, want %q", task.Status.State, a2a.TaskStateWorking)
	}
}

// TestClient_SendStreamingMessage_CommentLines verifies ":" comment lines
// (SSE keep-alive / heartbeats) are skipped without disrupting event parsing.
func TestClient_SendStreamingMessage_CommentLines(t *testing.T) {
	rawBody := ": keep-alive\n" +
		": another comment\n" +
		"data: {\"task\":{\"id\":\"task-c\",\"status\":{\"state\":\"TASK_STATE_COMPLETED\"}}}\n" +
		"\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	events, err := collectStream(t, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	task := events[0].(*a2a.Task)
	if task.ID != "task-c" {
		t.Errorf("task.ID: got %q, want %q", task.ID, "task-c")
	}
}

// TestClient_SendStreamingMessage_CRLF verifies CR/LF line endings are
// supported (bufio.Scanner splits on \n and strips trailing \r).
func TestClient_SendStreamingMessage_CRLF(t *testing.T) {
	rawBody := "data: {\"task\":{\"id\":\"task-crlf\",\"status\":{\"state\":\"TASK_STATE_COMPLETED\"}}}\r\n\r\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	events, err := collectStream(t, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	task := events[0].(*a2a.Task)
	if task.ID != "task-crlf" {
		t.Errorf("task.ID: got %q, want %q", task.ID, "task-crlf")
	}
}

// TestClient_SendStreamingMessage_EventVariants is a smoke test that each
// StreamResponse oneof variant (task/msg/statusUpdate/artifactUpdate) reaches
// the caller as the correct a2a.Event type across the HTTP boundary.
// Detailed field conversion is covered by wire_test.go's TestToA2AEvent_*.
func TestClient_SendStreamingMessage_EventVariants(t *testing.T) {
	rawBody := `data: {"task":{"id":"t1","status":{"state":"TASK_STATE_SUBMITTED"}}}` + "\n\n" +
		`data: {"msg":{"messageId":"m1","role":"ROLE_AGENT","content":[{"text":"hi"}]}}` + "\n\n" +
		`data: {"statusUpdate":{"taskId":"t1","contextId":"c1","status":{"state":"TASK_STATE_WORKING"}}}` + "\n\n" +
		`data: {"artifactUpdate":{"taskId":"t1","contextId":"c1","artifact":{"artifactId":"a1","parts":[{"text":"chunk"}]}}}` + "\n\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	events, err := collectStream(t, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("events: got %d, want 4", len(events))
	}
	if _, ok := events[0].(*a2a.Task); !ok {
		t.Errorf("event[0]: got %T, want *a2a.Task", events[0])
	}
	if _, ok := events[1].(*a2a.Message); !ok {
		t.Errorf("event[1]: got %T, want *a2a.Message", events[1])
	}
	if _, ok := events[2].(*a2a.TaskStatusUpdateEvent); !ok {
		t.Errorf("event[2]: got %T, want *a2a.TaskStatusUpdateEvent", events[2])
	}
	if _, ok := events[3].(*a2a.TaskArtifactUpdateEvent); !ok {
		t.Errorf("event[3]: got %T, want *a2a.TaskArtifactUpdateEvent", events[3])
	}
}

// TestClient_SendStreamingMessage_MultipleEvents verifies that multiple
// events separated by blank lines are each yielded in order.
func TestClient_SendStreamingMessage_MultipleEvents(t *testing.T) {
	rawBody := `data: {"task":{"id":"task-m","status":{"state":"TASK_STATE_WORKING"}}}` + "\n\n" +
		`data: {"statusUpdate":{"taskId":"task-m","contextId":"ctx-m","status":{"state":"TASK_STATE_WORKING"}}}` + "\n\n" +
		`data: {"task":{"id":"task-m","status":{"state":"TASK_STATE_COMPLETED"}}}` + "\n\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	events, err := collectStream(t, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events: got %d, want 3", len(events))
	}

	if task, ok := events[0].(*a2a.Task); !ok || task.Status.State != a2a.TaskStateWorking {
		t.Errorf("event[0]: got %T state=%v, want Task working", events[0], task)
	}
	if _, ok := events[1].(*a2a.TaskStatusUpdateEvent); !ok {
		t.Errorf("event[1]: got %T, want *a2a.TaskStatusUpdateEvent", events[1])
	}
	if task, ok := events[2].(*a2a.Task); !ok || task.Status.State != a2a.TaskStateCompleted {
		t.Errorf("event[2]: got %T state=%v, want Task completed", events[2], task)
	}
}

// TestClient_SendStreamingMessage_UnknownVariant verifies that an unknown
// oneof variant is silently skipped rather than treated as an error.
// This guarantees forward compatibility with future proto additions.
func TestClient_SendStreamingMessage_UnknownVariant(t *testing.T) {
	rawBody := `data: {"someFutureField":{"foo":"bar"}}` + "\n\n" +
		`data: {"task":{"id":"task-u","status":{"state":"TASK_STATE_COMPLETED"}}}` + "\n\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	events, err := collectStream(t, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1 (unknown variant should be skipped)", len(events))
	}
	if task := events[0].(*a2a.Task); task.ID != "task-u" {
		t.Errorf("task.ID: got %q, want %q", task.ID, "task-u")
	}
}

// TestClient_SendStreamingMessage_NoTrailingBlankLine verifies that an
// event at the end of the stream is flushed even if the server did not
// emit a trailing blank line before closing the connection.
func TestClient_SendStreamingMessage_NoTrailingBlankLine(t *testing.T) {
	rawBody := `data: {"task":{"id":"task-f","status":{"state":"TASK_STATE_COMPLETED"}}}` + "\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	events, err := collectStream(t, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1 (final event should flush)", len(events))
	}
	if task := events[0].(*a2a.Task); task.ID != "task-f" {
		t.Errorf("task.ID: got %q, want %q", task.ID, "task-f")
	}
}

// TestClient_SendStreamingMessage_StreamRequestBody verifies that
// SendStreamingMessage sends a request body *without* the blocking: true
// configuration that buildSendRequest adds.
// TestClient_Destroy verifies Destroy() returns nil (no-op).
func TestClient_Destroy(t *testing.T) {
	ep := &Endpoint{base: "https://example.com"}
	c := NewClient(ep, func() (string, error) {
		return "test-token", nil
	}, nil)
	if err := c.Destroy(); err != nil {
		t.Errorf("Destroy() = %v, want nil", err)
	}
}

// TestClient_FetchCard_DoError verifies that FetchCard returns an error
// when the underlying HTTP request fails (e.g. cancelled context).
func TestClient_FetchCard_DoError(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.FetchCard(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "card request failed") {
		t.Errorf("error should mention card request failed: %v", err)
	}
}

// TestClient_FetchCard_DecodeError verifies that FetchCard returns an error
// when the response body is not valid JSON (card parse failure).
func TestClient_FetchCard_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{not valid json`)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	_, err := c.FetchCard(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid card JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode agent card") {
		t.Errorf("error should mention decode failure: %v", err)
	}
}

// newFailingTokenClient creates a test client whose token function always fails.
func newFailingTokenClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	ep := &Endpoint{base: server.URL}
	c := NewClient(ep, func() (string, error) {
		return "", fmt.Errorf("token unavailable")
	}, nil)
	c.card = &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			{URL: server.URL + "/a2a/v1"},
		},
	}
	return c, server
}

// TestClient_SendMessage_TokenError verifies that SendMessage returns an error
// when the token function fails.
func TestClient_SendMessage_TokenError(t *testing.T) {
	c, server := newFailingTokenClient(t, http.NewServeMux())
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	_, err := c.SendMessage(context.Background(), &a2a.SendMessageRequest{Message: msg})
	if err == nil {
		t.Fatal("expected error for token failure")
	}
	if !strings.Contains(err.Error(), "access token") {
		t.Errorf("error should mention access token: %v", err)
	}
}

// TestClient_SendMessage_DoError verifies that SendMessage returns an error
// when the HTTP request fails (e.g. cancelled context).
func TestClient_SendMessage_DoError(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	_, err := c.SendMessage(ctx, &a2a.SendMessageRequest{Message: msg})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "send request failed") {
		t.Errorf("error should mention send request failed: %v", err)
	}
}

// TestClient_SendMessage_DecodeError verifies that SendMessage returns an error
// when the response body is not valid JSON.
func TestClient_SendMessage_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/message:send", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{not valid}`)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	_, err := c.SendMessage(context.Background(), &a2a.SendMessageRequest{Message: msg})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("error should mention decode failure: %v", err)
	}
}

// TestClient_SendStreamingMessage_TokenError verifies that SendStreamingMessage
// yields an error when the token function fails.
func TestClient_SendStreamingMessage_TokenError(t *testing.T) {
	c, server := newFailingTokenClient(t, http.NewServeMux())
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-test"

	var gotErr error
	for _, err := range c.SendStreamingMessage(context.Background(), &a2a.SendMessageRequest{Message: msg}) {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error for token failure")
	}
	if !strings.Contains(gotErr.Error(), "access token") {
		t.Errorf("error should mention access token: %v", gotErr)
	}
}

// TestClient_SendStreamingMessage_HTTPError verifies that SendStreamingMessage
// yields an error when the server returns a non-200 status.
func TestClient_SendStreamingMessage_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/message:stream", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, "internal error")
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	_, err := collectStream(t, c)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention 500: %v", err)
	}
}

// TestClient_SendStreamingMessage_DoError verifies that SendStreamingMessage
// yields an error when the HTTP request fails (e.g. cancelled context).
func TestClient_SendStreamingMessage_DoError(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-test"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var gotErr error
	for _, err := range c.SendStreamingMessage(ctx, &a2a.SendMessageRequest{Message: msg}) {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(gotErr.Error(), "stream request failed") {
		t.Errorf("error should mention stream request failed: %v", gotErr)
	}
}

// TestClient_SendStreamingMessage_ParseError verifies that SendStreamingMessage
// yields an error when a data field contains invalid JSON.
func TestClient_SendStreamingMessage_ParseError(t *testing.T) {
	rawBody := "data: {not valid json}\n\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	_, err := collectStream(t, c)
	if err == nil {
		t.Fatal("expected error for invalid JSON in stream data")
	}
	if !strings.Contains(err.Error(), "failed to decode stream event") {
		t.Errorf("error should mention decode failure: %v", err)
	}
}

// TestClient_SendStreamingMessage_FieldOnlyLine verifies that an SSE line
// without a colon (field name only, no value) is handled without crashing.
// Per the SSE spec, such lines set the field with an empty value.
func TestClient_SendStreamingMessage_FieldOnlyLine(t *testing.T) {
	// "retry" is a field-only line (no colon), followed by a valid event.
	rawBody := "retry\n" +
		`data: {"task":{"id":"task-fo","status":{"state":"TASK_STATE_COMPLETED"}}}` + "\n\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	events, err := collectStream(t, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	task := events[0].(*a2a.Task)
	if task.ID != "task-fo" {
		t.Errorf("task.ID: got %q, want %q", task.ID, "task-fo")
	}
}

// TestClient_SendStreamingMessage_EarlyReturn verifies that the consumer
// can stop iterating early (dispatch returns false) and the stream tears
// down cleanly.
func TestClient_SendStreamingMessage_EarlyReturn(t *testing.T) {
	rawBody := `data: {"task":{"id":"t1","status":{"state":"TASK_STATE_WORKING"}}}` + "\n\n" +
		`data: {"task":{"id":"t2","status":{"state":"TASK_STATE_COMPLETED"}}}` + "\n\n"

	c, server := newTestClient(t, streamBodyHandler(t, rawBody))
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-test"

	var events []a2a.Event
	for event, err := range c.SendStreamingMessage(context.Background(), &a2a.SendMessageRequest{Message: msg}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, event)
		break // stop after first event
	}

	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1 (early return)", len(events))
	}
	task := events[0].(*a2a.Task)
	if task.ID != "t1" {
		t.Errorf("task.ID: got %q, want %q", task.ID, "t1")
	}
}

// TestClient_GetTask_TokenError verifies that GetTask returns an error
// when the token function fails.
func TestClient_GetTask_TokenError(t *testing.T) {
	c, server := newFailingTokenClient(t, http.NewServeMux())
	defer server.Close()

	_, err := c.GetTask(context.Background(), &a2a.GetTaskRequest{
		ID: a2a.TaskID("task-001"),
	})
	if err == nil {
		t.Fatal("expected error for token failure")
	}
	if !strings.Contains(err.Error(), "access token") {
		t.Errorf("error should mention access token: %v", err)
	}
}

// TestClient_GetTask_DoError verifies that GetTask returns an error
// when the HTTP request fails (e.g. cancelled context).
func TestClient_GetTask_DoError(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetTask(ctx, &a2a.GetTaskRequest{
		ID: a2a.TaskID("task-001"),
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "task get request failed") {
		t.Errorf("error should mention task get request failed: %v", err)
	}
}

// TestClient_GetTask_DecodeError verifies that GetTask returns an error
// when the response body is not valid JSON.
func TestClient_GetTask_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/tasks/task-001", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{not valid}`)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	_, err := c.GetTask(context.Background(), &a2a.GetTaskRequest{
		ID: a2a.TaskID("task-001"),
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "failed to decode task response") {
		t.Errorf("error should mention decode failure: %v", err)
	}
}

// TestClient_CancelTask_TokenError verifies that CancelTask returns an error
// when the token function fails.
func TestClient_CancelTask_TokenError(t *testing.T) {
	c, server := newFailingTokenClient(t, http.NewServeMux())
	defer server.Close()

	_, err := c.CancelTask(context.Background(), &a2a.CancelTaskRequest{
		ID: a2a.TaskID("task-001"),
	})
	if err == nil {
		t.Fatal("expected error for token failure")
	}
	if !strings.Contains(err.Error(), "access token") {
		t.Errorf("error should mention access token: %v", err)
	}
}

// TestClient_CancelTask_DoError verifies that CancelTask returns an error
// when the HTTP request fails (e.g. cancelled context).
func TestClient_CancelTask_DoError(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.CancelTask(ctx, &a2a.CancelTaskRequest{
		ID: a2a.TaskID("task-001"),
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "task cancel request failed") {
		t.Errorf("error should mention task cancel request failed: %v", err)
	}
}

// TestClient_CancelTask_DecodeError verifies that CancelTask returns an error
// when the response body is not valid JSON.
func TestClient_CancelTask_DecodeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/tasks/task-001:cancel", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{not valid}`)
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	_, err := c.CancelTask(context.Background(), &a2a.CancelTaskRequest{
		ID: a2a.TaskID("task-001"),
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "failed to decode cancel response") {
		t.Errorf("error should mention decode failure: %v", err)
	}
}

// TestClient_SendMessage_MarshalError verifies that SendMessage returns an
// error when the request body cannot be marshalled to JSON (e.g. a Part
// with un-serializable metadata).
func TestClient_SendMessage_MarshalError(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-001"
	// Inject an un-serializable value into a Part's Metadata.
	msg.Parts[0].SetMeta("bad", make(chan int))

	_, err := c.SendMessage(context.Background(), &a2a.SendMessageRequest{Message: msg})
	if err == nil {
		t.Fatal("expected error for un-marshallable request")
	}
	if !strings.Contains(err.Error(), "failed to marshal request") {
		t.Errorf("error should mention marshal failure: %v", err)
	}
}

// TestClient_SendStreamingMessage_MarshalError verifies that
// SendStreamingMessage yields an error when the request body cannot be
// marshalled to JSON.
func TestClient_SendStreamingMessage_MarshalError(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-001"
	// Inject an un-serializable value into a Part's Metadata.
	msg.Parts[0].SetMeta("bad", make(chan int))

	var gotErr error
	for _, err := range c.SendStreamingMessage(context.Background(), &a2a.SendMessageRequest{Message: msg}) {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error for un-marshallable request")
	}
	if !strings.Contains(gotErr.Error(), "failed to marshal request") {
		t.Errorf("error should mention marshal failure: %v", gotErr)
	}
}

// TestClient_NewRequest_InvalidURL verifies that newRequest returns an error
// when given an invalid URL (e.g. containing control characters).
func TestClient_NewRequest_InvalidURL(t *testing.T) {
	ep := &Endpoint{base: "https://example.com"}
	c := NewClient(ep, func() (string, error) {
		return "test-token", nil
	}, nil)

	_, err := c.newRequest(context.Background(), http.MethodGet, "http://example.com/path\x00invalid", nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "failed to create request") {
		t.Errorf("error should mention failed to create request: %v", err)
	}
}

// TestClient_FetchCard_ReadBodyError verifies that FetchCard returns an error
// when io.ReadAll on the response body fails. We inject a broken reader via
// a custom http.Transport RoundTripper.
func TestClient_FetchCard_ReadBodyError(t *testing.T) {
	ep := &Endpoint{base: "https://example.com"}
	c := NewClient(ep, func() (string, error) {
		return "test-token", nil
	}, nil)
	c.httpClient = &http.Client{
		Transport: &brokenBodyTransport{
			statusCode: http.StatusOK,
			errAfter:   5,
		},
	}

	_, err := c.FetchCard(context.Background())
	if err == nil {
		t.Fatal("expected error when body read fails")
	}
	if !strings.Contains(err.Error(), "failed to read agent card response") {
		t.Errorf("error should mention read failure: %v", err)
	}
}

// TestClient_SendStreamingMessage_ScannerError verifies that
// SendStreamingMessage yields a "stream read error" when the underlying
// reader returns an error mid-stream.
func TestClient_SendStreamingMessage_ScannerError(t *testing.T) {
	ep := &Endpoint{base: "https://example.com"}
	c := NewClient(ep, func() (string, error) {
		return "test-token", nil
	}, nil)
	c.card = &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			{URL: "https://example.com/a2a/v1"},
		},
	}
	c.httpClient = &http.Client{
		Transport: &brokenBodyTransport{
			statusCode: http.StatusOK,
			// Return a partial SSE body then error. The data line is not
			// terminated by a blank line, so the scanner will read it then
			// encounter the error on the next Scan().
			prefix:   "data: partial",
			errAfter: len("data: partial"),
		},
	}

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart("hello"))
	msg.ID = "msg-test"

	var gotErr error
	for _, err := range c.SendStreamingMessage(context.Background(), &a2a.SendMessageRequest{Message: msg}) {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error from scanner")
	}
	if !strings.Contains(gotErr.Error(), "stream read error") {
		t.Errorf("error should mention stream read error: %v", gotErr)
	}
}

// brokenBodyTransport is an http.RoundTripper that returns a response whose
// Body reader returns an error after reading errAfter bytes. If prefix is
// set, those bytes are returned first before the error.
type brokenBodyTransport struct {
	statusCode int
	prefix     string
	errAfter   int
}

func (t *brokenBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.statusCode,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       &brokenReader{data: []byte(t.prefix), errAfter: t.errAfter},
	}, nil
}

// brokenReader returns the prefix data, then an error on the next Read.
type brokenReader struct {
	data     []byte
	pos      int
	errAfter int
	done     bool
}

func (r *brokenReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, fmt.Errorf("simulated read error")
	}
	remaining := r.data[r.pos:]
	if len(remaining) == 0 {
		r.done = true
		return 0, fmt.Errorf("simulated read error")
	}
	n := copy(p, remaining)
	r.pos += n
	if r.pos >= len(r.data) {
		r.done = true
	}
	return n, nil
}

func (r *brokenReader) Close() error { return nil }

func TestClient_SendStreamingMessage_StreamRequestBody(t *testing.T) {
	var captured sendRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/message:stream", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `data: {"task":{"id":"t"}}`+"\n\n")
	})

	c, server := newTestClient(t, mux)
	defer server.Close()

	_, _ = collectStream(t, c)

	if captured.Configuration != nil {
		t.Errorf("stream request Configuration: got %+v, want nil (stream should not send blocking: true)", captured.Configuration)
	}
	if captured.Message.MessageID == "" {
		t.Error("stream request Message.MessageID: got empty, want auto-generated UUID")
	}
}

func TestClient_ListTasks_NotSupported(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	_, err := c.ListTasks(context.Background(), &a2a.ListTasksRequest{})
	if err == nil {
		t.Fatal("expected error for ListTasks on Vertex AI")
	}
	if !errors.Is(err, ErrListTasksNotSupported) {
		t.Errorf("expected ErrListTasksNotSupported, got: %v", err)
	}
}

func TestClient_SubscribeToTask_NotSupported(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	var gotErr error
	for _, err := range c.SubscribeToTask(context.Background(), &a2a.SubscribeToTaskRequest{ID: "task-1"}) {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected error for SubscribeToTask on Vertex AI")
	}
	if !errors.Is(gotErr, ErrSubscribeToTaskNotSupported) {
		t.Errorf("expected ErrSubscribeToTaskNotSupported, got: %v", gotErr)
	}
}

func TestClient_PushConfig_NotSupported(t *testing.T) {
	c, server := newTestClient(t, http.NewServeMux())
	defer server.Close()

	ctx := context.Background()

	if _, err := c.CreateTaskPushConfig(ctx, &a2a.CreateTaskPushConfigRequest{}); !errors.Is(err, ErrPushNotSupported) {
		t.Errorf("CreateTaskPushConfig: got %v, want ErrPushNotSupported", err)
	}
	if _, err := c.GetTaskPushConfig(ctx, &a2a.GetTaskPushConfigRequest{}); !errors.Is(err, ErrPushNotSupported) {
		t.Errorf("GetTaskPushConfig: got %v, want ErrPushNotSupported", err)
	}
	if _, err := c.ListTaskPushConfigs(ctx, &a2a.ListTaskPushConfigRequest{}); !errors.Is(err, ErrPushNotSupported) {
		t.Errorf("ListTaskPushConfigs: got %v, want ErrPushNotSupported", err)
	}
	if err := c.DeleteTaskPushConfig(ctx, &a2a.DeleteTaskPushConfigRequest{}); !errors.Is(err, ErrPushNotSupported) {
		t.Errorf("DeleteTaskPushConfig: got %v, want ErrPushNotSupported", err)
	}
}

// ---------------------------------------------------------------------------
// readErrorResponse
// ---------------------------------------------------------------------------

func TestReadErrorResponse_Short(t *testing.T) {
	body := "short error body"
	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	err := readErrorResponse(resp)
	want := "HTTP 400: short error body"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestReadErrorResponse_LongBodyDrained(t *testing.T) {
	// Build a body larger than the 4096-byte limit.
	full := strings.Repeat("x", 8192)
	r := strings.NewReader(full)
	resp := &http.Response{
		StatusCode: 502,
		Body:       io.NopCloser(r),
	}
	err := readErrorResponse(resp)

	// Error message should contain only the first 4096 bytes.
	prefix := strings.Repeat("x", 4096)
	want := "HTTP 502: " + prefix
	if err.Error() != want {
		t.Errorf("error message length: got %d, want %d", len(err.Error()), len(want))
	}

	// The underlying reader must be fully consumed (drained).
	if remaining := r.Len(); remaining != 0 {
		t.Errorf("body not fully drained: %d bytes remaining", remaining)
	}
}
