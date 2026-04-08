package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Lalcs/a2ahoy/internal/auth"
	"github.com/a2aproject/a2a-go/v2/a2a"
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

// newHeaderCaptureServer starts an httptest server that serves the given
// card JSON at /.well-known/agent-card.json and records the incoming request
// headers into the provided map.
func newHeaderCaptureServer(t *testing.T, cardBody func(url string) string, captured *http.Header) *httptest.Server {
	t.Helper()
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		*captured = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, cardBody(ts.URL))
	})
	ts = httptest.NewServer(mux)
	return ts
}

func TestNew_WithHeaders(t *testing.T) {
	var captured http.Header
	ts := newHeaderCaptureServer(t, v1CardJSON, &captured)
	defer ts.Close()

	ctx := context.Background()
	a2aClient, _, err := New(ctx, Options{
		BaseURL: ts.URL,
		Headers: []string{"X-Tenant-ID=tenant-1", "X-Custom-Auth=secret"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer a2aClient.Destroy()

	if got := captured.Get("X-Tenant-Id"); got != "tenant-1" {
		t.Errorf("X-Tenant-Id: got %q, want %q", got, "tenant-1")
	}
	if got := captured.Get("X-Custom-Auth"); got != "secret" {
		t.Errorf("X-Custom-Auth: got %q, want %q", got, "secret")
	}
}

func TestNew_WithHeaders_InvalidEntry(t *testing.T) {
	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL: "http://example.invalid",
		Headers: []string{"missing-equals"},
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !errors.Is(err, auth.ErrInvalidHeader) {
		t.Errorf("expected ErrInvalidHeader, got: %v", err)
	}
}

func TestResolveCard_WithHeaders(t *testing.T) {
	var captured http.Header
	ts := newHeaderCaptureServer(t, v1CardJSON, &captured)
	defer ts.Close()

	ctx := context.Background()
	card, err := ResolveCard(ctx, Options{
		BaseURL: ts.URL,
		Headers: []string{"X-Tenant-ID=123", "A2A-Extensions=ext1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("card should not be nil")
	}

	if got := captured.Get("X-Tenant-Id"); got != "123" {
		t.Errorf("X-Tenant-Id: got %q, want %q", got, "123")
	}
	if got := captured.Get("A2A-Extensions"); got != "ext1" {
		t.Errorf("A2A-Extensions: got %q, want %q", got, "ext1")
	}
}

func TestResolveCard_WithHeaders_InvalidEntry(t *testing.T) {
	ctx := context.Background()
	_, err := ResolveCard(ctx, Options{
		BaseURL: "http://example.invalid",
		Headers: []string{"=empty-key"},
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !errors.Is(err, auth.ErrInvalidHeader) {
		t.Errorf("expected ErrInvalidHeader, got: %v", err)
	}
}

func TestNew_WithBearerToken(t *testing.T) {
	var captured http.Header
	ts := newHeaderCaptureServer(t, v1CardJSON, &captured)
	defer ts.Close()

	ctx := context.Background()
	a2aClient, _, err := New(ctx, Options{
		BaseURL:     ts.URL,
		BearerToken: "test-bearer-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer a2aClient.Destroy()

	if got := captured.Get("Authorization"); got != "Bearer test-bearer-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer test-bearer-token")
	}
}

func TestNew_WithBearerToken_EmptyTokenIgnored(t *testing.T) {
	var captured http.Header
	ts := newHeaderCaptureServer(t, v1CardJSON, &captured)
	defer ts.Close()

	ctx := context.Background()
	a2aClient, _, err := New(ctx, Options{
		BaseURL:     ts.URL,
		BearerToken: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer a2aClient.Destroy()

	// Empty BearerToken must be treated as "not set" and must not inject
	// an Authorization header.
	if got := captured.Get("Authorization"); got != "" {
		t.Errorf("Authorization should be empty, got %q", got)
	}
}

func TestResolveCard_WithBearerToken(t *testing.T) {
	var captured http.Header
	ts := newHeaderCaptureServer(t, v1CardJSON, &captured)
	defer ts.Close()

	ctx := context.Background()
	card, err := ResolveCard(ctx, Options{
		BaseURL:     ts.URL,
		BearerToken: "resolve-card-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("card should not be nil")
	}

	if got := captured.Get("Authorization"); got != "Bearer resolve-card-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer resolve-card-token")
	}
}

// TestApplyV03RESTMountPrefix covers the v0.3 REST mount point
// compatibility rewrite. Canonical v0.3 peers (Python a2a-sdk, ADK,
// Vertex AI Agent Engine non-Vertex route) publish cards whose HTTP+JSON
// URL lacks "/v1" but whose routes actually mount under "/v1/*", because
// their REST clients hardcode "/v1" on top of the URL. a2a-go v2 instead
// follows the v0.3 spec example literally and joins "/message:send"
// directly onto iface.URL. The helper rewrites the URL of HTTP+JSON v0.3
// interfaces so the joined request URL hits /v1/..., bridging the gap.
func TestApplyV03RESTMountPrefix(t *testing.T) {
	tests := []struct {
		name  string
		in    *a2a.AgentCard
		check func(*testing.T, *a2a.AgentCard)
	}{
		{
			name: "HTTP+JSON v0.3 with trailing slash gets /v1 appended",
			in: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{
						URL:             "http://localhost:9999/",
						ProtocolBinding: a2a.TransportProtocolHTTPJSON,
						ProtocolVersion: "0.3.0",
					},
				},
			},
			check: func(t *testing.T, card *a2a.AgentCard) {
				if got, want := card.SupportedInterfaces[0].URL, "http://localhost:9999/v1"; got != want {
					t.Errorf("URL: got %q, want %q", got, want)
				}
			},
		},
		{
			name: "HTTP+JSON v0.3 without trailing slash gets /v1 appended",
			in: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{
						URL:             "http://localhost:9999",
						ProtocolBinding: a2a.TransportProtocolHTTPJSON,
						ProtocolVersion: "0.3.0",
					},
				},
			},
			check: func(t *testing.T, card *a2a.AgentCard) {
				if got, want := card.SupportedInterfaces[0].URL, "http://localhost:9999/v1"; got != want {
					t.Errorf("URL: got %q, want %q", got, want)
				}
			},
		},
		{
			name: "HTTP+JSON v0.3 with URL already ending in /v1 is idempotent",
			in: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{
						URL:             "http://localhost:9999/v1",
						ProtocolBinding: a2a.TransportProtocolHTTPJSON,
						ProtocolVersion: "0.3.0",
					},
				},
			},
			check: func(t *testing.T, card *a2a.AgentCard) {
				if got, want := card.SupportedInterfaces[0].URL, "http://localhost:9999/v1"; got != want {
					t.Errorf("URL: got %q, want %q (should be unchanged)", got, want)
				}
			},
		},
		{
			name: "HTTP+JSON v0.3 with URL ending in /v1/ is idempotent",
			in: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{
						URL:             "http://localhost:9999/v1/",
						ProtocolBinding: a2a.TransportProtocolHTTPJSON,
						ProtocolVersion: "0.3.0",
					},
				},
			},
			check: func(t *testing.T, card *a2a.AgentCard) {
				if got, want := card.SupportedInterfaces[0].URL, "http://localhost:9999/v1/"; got != want {
					t.Errorf("URL: got %q, want %q (should be unchanged)", got, want)
				}
			},
		},
		{
			name: "HTTP+JSON v0.3 with short protocol version 0.3 matches",
			in: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{
						URL:             "http://localhost:9999/",
						ProtocolBinding: a2a.TransportProtocolHTTPJSON,
						ProtocolVersion: "0.3",
					},
				},
			},
			check: func(t *testing.T, card *a2a.AgentCard) {
				if got, want := card.SupportedInterfaces[0].URL, "http://localhost:9999/v1"; got != want {
					t.Errorf("URL: got %q, want %q", got, want)
				}
			},
		},
		{
			name: "JSONRPC transport is left untouched",
			in: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{
						URL:             "http://localhost:9999/",
						ProtocolBinding: a2a.TransportProtocolJSONRPC,
						ProtocolVersion: "0.3.0",
					},
				},
			},
			check: func(t *testing.T, card *a2a.AgentCard) {
				if got, want := card.SupportedInterfaces[0].URL, "http://localhost:9999/"; got != want {
					t.Errorf("URL: got %q, want %q (JSONRPC should be unchanged)", got, want)
				}
			},
		},
		{
			name: "HTTP+JSON v1.0 is left untouched",
			in: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{
						URL:             "http://localhost:9999/",
						ProtocolBinding: a2a.TransportProtocolHTTPJSON,
						ProtocolVersion: "1.0",
					},
				},
			},
			check: func(t *testing.T, card *a2a.AgentCard) {
				if got, want := card.SupportedInterfaces[0].URL, "http://localhost:9999/"; got != want {
					t.Errorf("URL: got %q, want %q (v1.0 should be unchanged)", got, want)
				}
			},
		},
		{
			name: "mixed interfaces: only HTTP+JSON v0.3 is rewritten",
			in: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{
						URL:             "http://a/",
						ProtocolBinding: a2a.TransportProtocolHTTPJSON,
						ProtocolVersion: "0.3.0",
					},
					{
						URL:             "http://b/",
						ProtocolBinding: a2a.TransportProtocolJSONRPC,
						ProtocolVersion: "0.3.0",
					},
					{
						URL:             "http://c/",
						ProtocolBinding: a2a.TransportProtocolHTTPJSON,
						ProtocolVersion: "1.0",
					},
				},
			},
			check: func(t *testing.T, card *a2a.AgentCard) {
				want := []string{"http://a/v1", "http://b/", "http://c/"}
				for i, w := range want {
					if got := card.SupportedInterfaces[i].URL; got != w {
						t.Errorf("interface %d URL: got %q, want %q", i, got, w)
					}
				}
			},
		},
		{
			name: "nil card does not panic",
			in:   nil,
			check: func(t *testing.T, card *a2a.AgentCard) {
				if card != nil {
					t.Errorf("expected nil card, got %v", card)
				}
			},
		},
		{
			name: "nil interface entry is skipped",
			in: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					nil,
					{
						URL:             "http://localhost:9999/",
						ProtocolBinding: a2a.TransportProtocolHTTPJSON,
						ProtocolVersion: "0.3.0",
					},
				},
			},
			check: func(t *testing.T, card *a2a.AgentCard) {
				if card.SupportedInterfaces[0] != nil {
					t.Errorf("expected first interface to remain nil")
				}
				if got, want := card.SupportedInterfaces[1].URL, "http://localhost:9999/v1"; got != want {
					t.Errorf("URL: got %q, want %q", got, want)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applyV03RESTMountPrefix(tc.in)
			tc.check(t, tc.in)
		})
	}
}

// TestNew_V03HTTPJSON_AppendsV1Prefix is an end-to-end regression test
// for the v0.3 REST mount point compatibility rewrite. It serves a v0.3
// agent card whose preferredTransport is "HTTP+JSON" and asserts that
// applyV03RESTMountPrefix rewrites the interface URL so subsequent REST
// calls resolve under /v1.
func TestNew_V03HTTPJSON_AppendsV1Prefix(t *testing.T) {
	ts := newCardServer(t, func(url string) string {
		return fmt.Sprintf(`{
			"name": "Test v0.3 HTTP+JSON Agent",
			"description": "A v0.3 HTTP+JSON test agent",
			"version": "1.0",
			"protocolVersion": "0.3.0",
			"url": %q,
			"preferredTransport": "HTTP+JSON",
			"capabilities": {},
			"defaultInputModes": ["text/plain"],
			"defaultOutputModes": ["text/plain"],
			"skills": []
		}`, url)
	})
	defer ts.Close()

	ctx := context.Background()
	a2aClient, card, err := New(ctx, Options{BaseURL: ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer a2aClient.Destroy()

	if len(card.SupportedInterfaces) == 0 {
		t.Fatal("SupportedInterfaces must not be empty")
	}
	want := ts.URL + "/v1"
	got := card.SupportedInterfaces[0].URL
	if got != want {
		t.Errorf("SupportedInterfaces[0].URL: got %q, want %q (compatibility rewrite did not apply)", got, want)
	}
}

func TestResolveCard_WithBearerToken_EmptyTokenIgnored(t *testing.T) {
	var captured http.Header
	ts := newHeaderCaptureServer(t, v1CardJSON, &captured)
	defer ts.Close()

	ctx := context.Background()
	_, err := ResolveCard(ctx, Options{
		BaseURL:     ts.URL,
		BearerToken: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := captured.Get("Authorization"); got != "" {
		t.Errorf("Authorization should be empty, got %q", got)
	}
}
