package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// v1CardJSON returns a minimal A2A spec v1.0 agent card JSON pointing at the
// given URL. v1.0 servers expose `supportedInterfaces` as the canonical way
// to advertise transports.
func v1CardJSON(url string) string {
	return fmt.Sprintf(`{
		"name": "Test v1 Agent",
		"description": "A v1 test agent",
		"version": "1.0",
		"capabilities": {},
		"defaultInputModes": ["text/plain"],
		"defaultOutputModes": ["text/plain"],
		"supportedInterfaces": [{
			"url": %q,
			"protocolBinding": "JSONRPC",
			"protocolVersion": "1.0"
		}],
		"skills": []
	}`, url)
}

// v03CardJSON returns a minimal A2A spec v0.3 agent card JSON pointing at the
// given URL. This mimics the output of Python a2a-sdk 0.3.x servers (e.g.,
// the Google ADK), which use top-level `url` + `preferredTransport` instead
// of `supportedInterfaces`.
func v03CardJSON(url string) string {
	return fmt.Sprintf(`{
		"name": "Test v0.3 Agent",
		"description": "A v0.3 test agent",
		"version": "1.0",
		"protocolVersion": "0.3.0",
		"url": %q,
		"preferredTransport": "JSONRPC",
		"capabilities": {},
		"defaultInputModes": ["text/plain"],
		"defaultOutputModes": ["text/plain"],
		"skills": []
	}`, url)
}

// newCardServer starts an httptest server that serves the given agent card
// JSON at /.well-known/agent-card.json. The card body is provided by a
// closure so it can include the server's own URL.
func newCardServer(t *testing.T, cardBody func(url string) string) *httptest.Server {
	t.Helper()
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, cardBody(ts.URL))
	})
	ts = httptest.NewServer(mux)
	return ts
}

func TestNew_WithoutGCPAuth(t *testing.T) {
	ts := newCardServer(t, v1CardJSON)
	defer ts.Close()

	ctx := context.Background()
	a2aClient, card, err := New(ctx, Options{
		BaseURL: ts.URL,
		GCPAuth: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer a2aClient.Destroy()

	if card.Name != "Test v1 Agent" {
		t.Errorf("got card name %q, want %q", card.Name, "Test v1 Agent")
	}
	if card.Description != "A v1 test agent" {
		t.Errorf("got card description %q, want %q", card.Description, "A v1 test agent")
	}
	if card.Version != "1.0" {
		t.Errorf("got card version %q, want %q", card.Version, "1.0")
	}
}

func TestNew_InvalidURL(t *testing.T) {
	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL: "http://localhost:1", // port 1 should refuse connections
		GCPAuth: false,
	})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNew_CardResolutionFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL: ts.URL,
		GCPAuth: false,
	})
	if err == nil {
		t.Fatal("expected error for card resolution failure")
	}
}

func TestNew_InvalidCardJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer ts.Close()

	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL: ts.URL,
		GCPAuth: false,
	})
	if err == nil {
		t.Fatal("expected error for invalid card JSON")
	}
}

// TestNew_V03Format_Regression is a regression test for A2AHOY-1.
//
// Before the fix, calling New() against a v0.3-format agent card failed
// with "agent card has no supported interfaces" because the default v1
// JSON parser silently dropped the v0.3-only `url`/`preferredTransport`
// fields, leaving SupportedInterfaces empty. After the fix, the v0 compat
// parser populates SupportedInterfaces from those fields and the v0.3
// JSON-RPC transport is registered via WithCompatTransport.
func TestNew_V03Format_Regression(t *testing.T) {
	ts := newCardServer(t, v03CardJSON)
	defer ts.Close()

	ctx := context.Background()
	a2aClient, card, err := New(ctx, Options{
		BaseURL: ts.URL,
		GCPAuth: false,
	})
	if err != nil {
		t.Fatalf("unexpected error (regression A2AHOY-1): %v", err)
	}
	defer a2aClient.Destroy()

	if card.Name != "Test v0.3 Agent" {
		t.Errorf("got card name %q, want %q", card.Name, "Test v0.3 Agent")
	}
	if len(card.SupportedInterfaces) == 0 {
		t.Fatal("SupportedInterfaces must not be empty after v0 compat parse")
	}
}

func TestResolveCard_V03Format(t *testing.T) {
	ts := newCardServer(t, v03CardJSON)
	defer ts.Close()

	ctx := context.Background()
	card, err := ResolveCard(ctx, Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.Name != "Test v0.3 Agent" {
		t.Errorf("got card name %q, want %q", card.Name, "Test v0.3 Agent")
	}
	if len(card.SupportedInterfaces) == 0 {
		t.Fatal("SupportedInterfaces must not be empty after v0 compat parse")
	}
	if got := card.SupportedInterfaces[0].URL; got != ts.URL {
		t.Errorf("got interface URL %q, want %q", got, ts.URL)
	}
}

func TestResolveCard_V1Format(t *testing.T) {
	ts := newCardServer(t, v1CardJSON)
	defer ts.Close()

	ctx := context.Background()
	card, err := ResolveCard(ctx, Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.Name != "Test v1 Agent" {
		t.Errorf("got card name %q, want %q", card.Name, "Test v1 Agent")
	}
	if len(card.SupportedInterfaces) != 1 {
		t.Fatalf("got %d supported interfaces, want 1", len(card.SupportedInterfaces))
	}
	if got := card.SupportedInterfaces[0].URL; got != ts.URL {
		t.Errorf("got interface URL %q, want %q", got, ts.URL)
	}
}

func TestResolveCard_InvalidURL(t *testing.T) {
	ctx := context.Background()
	_, err := ResolveCard(ctx, Options{
		BaseURL: "http://localhost:1", // port 1 should refuse connections
	})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestResolveCard_CardResolutionFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ctx := context.Background()
	_, err := ResolveCard(ctx, Options{BaseURL: ts.URL})
	if err == nil {
		t.Fatal("expected error for card resolution failure")
	}
}

func TestResolveCard_InvalidCardJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer ts.Close()

	ctx := context.Background()
	_, err := ResolveCard(ctx, Options{BaseURL: ts.URL})
	if err == nil {
		t.Fatal("expected error for invalid card JSON")
	}
}
