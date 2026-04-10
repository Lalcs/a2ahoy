package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Lalcs/a2ahoy/internal/auth"
	"github.com/Lalcs/a2ahoy/internal/vertexai"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"golang.org/x/oauth2"
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
		_, _ = fmt.Fprint(w, cardBody(ts.URL))
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
	defer func() { _ = a2aClient.Destroy() }()

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
		_, _ = w.Write([]byte(`{invalid json`))
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
	defer func() { _ = a2aClient.Destroy() }()

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
		_, _ = w.Write([]byte(`{invalid json`))
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
		_, _ = fmt.Fprint(w, cardBody(ts.URL))
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
	defer func() { _ = a2aClient.Destroy() }()

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
	defer func() { _ = a2aClient.Destroy() }()

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
	defer func() { _ = a2aClient.Destroy() }()

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

// TestNew_V03HTTPJSON_V03RESTMountGating is an end-to-end regression test
// for the opt-in v0.3 REST mount point compatibility rewrite. It serves a
// v0.3 agent card whose preferredTransport is "HTTP+JSON" and asserts that:
//
//   - with Options.V03RESTMount == false (default), the URL is preserved
//     as-is so native a2a-go v2 spec-compliant peers are addressed as
//     the server advertised them;
//   - with Options.V03RESTMount == true, applyV03RESTMountPrefix rewrites
//     the interface URL so subsequent REST calls resolve under /v1 — the
//     workaround required by Python a2a-sdk / ADK / Vertex AI non-Vertex
//     route peers.
//
// The two cases share the card fixture so any drift in the server's
// preferredTransport / protocolVersion combination is caught by both at
// once.
func TestNew_V03HTTPJSON_V03RESTMountGating(t *testing.T) {
	cardBody := func(url string) string {
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
	}

	// One server covers both subtests: cardBody is stateless and each
	// run reads the card independently, so there is no cross-test
	// interference from sharing the underlying httptest.Server.
	ts := newCardServer(t, cardBody)
	defer ts.Close()

	tests := []struct {
		name         string
		v03RESTMount bool
		wantSuffix   string
	}{
		{
			name:         "default (opt-out) preserves raw URL",
			v03RESTMount: false,
			wantSuffix:   "", // == ts.URL, no /v1 suffix
		},
		{
			name:         "opt-in appends /v1 for v0.3 HTTP+JSON",
			v03RESTMount: true,
			wantSuffix:   "/v1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			a2aClient, card, err := New(ctx, Options{
				BaseURL:      ts.URL,
				V03RESTMount: tc.v03RESTMount,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = a2aClient.Destroy() }()

			if len(card.SupportedInterfaces) == 0 {
				t.Fatal("SupportedInterfaces must not be empty")
			}
			want := ts.URL + tc.wantSuffix
			got := card.SupportedInterfaces[0].URL
			if got != want {
				t.Errorf("SupportedInterfaces[0].URL: got %q, want %q", got, want)
			}
		})
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

// ---------------------------------------------------------------------------
// VertexAI path tests
// ---------------------------------------------------------------------------

// invalidCredFile creates a temporary file with invalid JSON content and
// sets GOOGLE_APPLICATION_CREDENTIALS to it so GCP credential lookups fail
// deterministically.
func invalidCredFile(t *testing.T) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "bad-creds.json")
	if err := os.WriteFile(tmp, []byte("not-json"), 0o644); err != nil {
		t.Fatalf("write bad creds: %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", tmp)
}

func TestNew_VertexAI_ParseEndpointError(t *testing.T) {
	invalidCredFile(t)
	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL:  "", // empty → ParseEndpoint error
		VertexAI: true,
	})
	if err == nil {
		t.Fatal("expected error for empty VertexAI URL")
	}
}

func TestNew_VertexAI_AuthError(t *testing.T) {
	invalidCredFile(t)
	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL:  "https://us-central1-aiplatform.googleapis.com/v1/projects/1/locations/us-central1/reasoningEngines/1",
		VertexAI: true,
	})
	if err == nil {
		t.Fatal("expected error for invalid GCP credentials")
	}
}

func TestResolveCard_VertexAI_ParseEndpointError(t *testing.T) {
	invalidCredFile(t)
	ctx := context.Background()
	_, err := ResolveCard(ctx, Options{
		BaseURL:  "",
		VertexAI: true,
	})
	if err == nil {
		t.Fatal("expected error for empty VertexAI URL")
	}
}

func TestResolveCard_VertexAI_AuthError(t *testing.T) {
	invalidCredFile(t)
	ctx := context.Background()
	_, err := ResolveCard(ctx, Options{
		BaseURL:  "https://us-central1-aiplatform.googleapis.com/v1/projects/1/locations/us-central1/reasoningEngines/1",
		VertexAI: true,
	})
	if err == nil {
		t.Fatal("expected error for invalid GCP credentials")
	}
}

func TestNew_GCPAuth_Error(t *testing.T) {
	invalidCredFile(t)
	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL: "http://example.com",
		GCPAuth: true,
	})
	if err == nil {
		t.Fatal("expected error for GCP auth failure")
	}
}

func TestResolveCard_GCPAuth_Error(t *testing.T) {
	invalidCredFile(t)
	ctx := context.Background()
	_, err := ResolveCard(ctx, Options{
		BaseURL: "http://example.com",
		GCPAuth: true,
	})
	if err == nil {
		t.Fatal("expected error for GCP auth failure")
	}
}

// ---------------------------------------------------------------------------
// applyVertexAIHeaders tests
// ---------------------------------------------------------------------------

func TestApplyVertexAIHeaders_Empty(t *testing.T) {
	ep, _ := vertexai.ParseEndpoint("https://example.com/v1beta1/projects/1/locations/us/reasoningEngines/1")
	vc := vertexai.NewClient(ep, func() (string, error) { return "tok", nil }, nil)

	// Empty headers → no-op, no error.
	if err := applyVertexAIHeaders(vc, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := applyVertexAIHeaders(vc, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyVertexAIHeaders_Valid(t *testing.T) {
	ep, _ := vertexai.ParseEndpoint("https://example.com/v1beta1/projects/1/locations/us/reasoningEngines/1")
	vc := vertexai.NewClient(ep, func() (string, error) { return "tok", nil }, nil)

	if err := applyVertexAIHeaders(vc, []string{"X-Custom=val"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyVertexAIHeaders_InvalidEntry(t *testing.T) {
	ep, _ := vertexai.ParseEndpoint("https://example.com/v1beta1/projects/1/locations/us/reasoningEngines/1")
	vc := vertexai.NewClient(ep, func() (string, error) { return "tok", nil }, nil)

	err := applyVertexAIHeaders(vc, []string{"no-equals"})
	if err == nil {
		t.Fatal("expected error for invalid header entry")
	}
	if !errors.Is(err, auth.ErrInvalidHeader) {
		t.Errorf("expected ErrInvalidHeader, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// appendBearerResolveOpts edge case
// ---------------------------------------------------------------------------

func TestAppendBearerResolveOpts_NonEmpty(t *testing.T) {
	opts := appendBearerResolveOpts(nil, "my-token")
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
}

func TestAppendBearerResolveOpts_Empty(t *testing.T) {
	opts := appendBearerResolveOpts(nil, "")
	if len(opts) != 0 {
		t.Fatalf("expected 0 options for empty token, got %d", len(opts))
	}
}

// ---------------------------------------------------------------------------
// VertexAI success path tests (inject mock GCP interceptor)
// ---------------------------------------------------------------------------

// staticTokenSource is a simple oauth2.TokenSource that always returns the
// given access token string.
type staticTokenSource string

func (s staticTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: string(s)}, nil
}

// withMockGCPAccessToken overrides the GCP access-token interceptor factory
// with one that returns a fake interceptor backed by the given token.
// The original factory is restored via t.Cleanup.
func withMockGCPAccessToken(t *testing.T, token string) {
	t.Helper()
	orig := newGCPAccessTokenInterceptorFn
	t.Cleanup(func() { newGCPAccessTokenInterceptorFn = orig })
	newGCPAccessTokenInterceptorFn = func(ctx context.Context) (*auth.GCPAccessTokenInterceptor, error) {
		return auth.NewGCPAccessTokenInterceptorFromSource(staticTokenSource(token)), nil
	}
}

// withMockGCPAuth overrides the GCP ID-token interceptor factory with one
// that returns a fake interceptor backed by the given token.
func withMockGCPAuth(t *testing.T, token string) {
	t.Helper()
	orig := newGCPAuthInterceptorFn
	t.Cleanup(func() { newGCPAuthInterceptorFn = orig })
	newGCPAuthInterceptorFn = func(ctx context.Context, audience string) (*auth.GCPAuthInterceptor, error) {
		return auth.NewGCPAuthInterceptorFromSource(staticTokenSource(token)), nil
	}
}

// vertexAICardServer returns an httptest server that serves a v0.3-format
// agent card at /a2a/v1/card.
func vertexAICardServer(t *testing.T) *httptest.Server {
	t.Helper()
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/v1/card", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"protocolVersion": "0.3.0",
			"name": "Test VertexAI Agent",
			"description": "test",
			"version": "1.0",
			"url": %q,
			"preferredTransport": "HTTP+JSON",
			"capabilities": {},
			"defaultInputModes": ["text/plain"],
			"defaultOutputModes": ["text/plain"],
			"skills": []
		}`, ts.URL+"/a2a/v1")
	})
	ts = httptest.NewServer(mux)
	return ts
}

func TestNew_VertexAI_Success(t *testing.T) {
	ts := vertexAICardServer(t)
	defer ts.Close()

	withMockGCPAccessToken(t, "fake-access-token")

	ctx := context.Background()
	client, card, err := New(ctx, Options{
		BaseURL:  ts.URL,
		VertexAI: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = client.Destroy() }()

	if card.Name != "Test VertexAI Agent" {
		t.Errorf("card name: got %q, want %q", card.Name, "Test VertexAI Agent")
	}
}

func TestNew_VertexAI_V03RESTMount(t *testing.T) {
	ts := vertexAICardServer(t)
	defer ts.Close()

	withMockGCPAccessToken(t, "fake-access-token")

	ctx := context.Background()
	_, card, err := New(ctx, Options{
		BaseURL:      ts.URL,
		VertexAI:     true,
		V03RESTMount: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(card.SupportedInterfaces) == 0 {
		t.Fatal("expected at least one interface")
	}
	// Card URL already ends with /v1, so rewrite is idempotent.
	got := card.SupportedInterfaces[0].URL
	if got != ts.URL+"/a2a/v1" {
		t.Errorf("URL after V03RESTMount: got %q, want %q", got, ts.URL+"/a2a/v1")
	}
}

func TestNew_VertexAI_FetchCardError(t *testing.T) {
	// Server that returns 500 for card
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	withMockGCPAccessToken(t, "fake-access-token")

	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL:  ts.URL,
		VertexAI: true,
	})
	if err == nil {
		t.Fatal("expected error for FetchCard failure")
	}
}

func TestNew_VertexAI_HeaderError(t *testing.T) {
	withMockGCPAccessToken(t, "fake-access-token")

	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL:  "https://us-central1-aiplatform.googleapis.com/v1/projects/1/locations/us/reasoningEngines/1",
		VertexAI: true,
		Headers:  []string{"no-equals"},
	})
	if err == nil {
		t.Fatal("expected error for invalid headers")
	}
}

func TestResolveCard_VertexAI_Success(t *testing.T) {
	ts := vertexAICardServer(t)
	defer ts.Close()

	withMockGCPAccessToken(t, "fake-access-token")

	ctx := context.Background()
	card, err := ResolveCard(ctx, Options{
		BaseURL:  ts.URL,
		VertexAI: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.Name != "Test VertexAI Agent" {
		t.Errorf("card name: got %q, want %q", card.Name, "Test VertexAI Agent")
	}
}

// ---------------------------------------------------------------------------
// GCPAuth success path tests (inject mock GCP interceptor)
// ---------------------------------------------------------------------------

func TestNew_GCPAuth_Success(t *testing.T) {
	var captured http.Header
	ts := newHeaderCaptureServer(t, v1CardJSON, &captured)
	defer ts.Close()

	withMockGCPAuth(t, "fake-id-token")

	ctx := context.Background()
	client, card, err := New(ctx, Options{
		BaseURL: ts.URL,
		GCPAuth: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = client.Destroy() }()

	if card.Name != "Test v1 Agent" {
		t.Errorf("card name: got %q, want %q", card.Name, "Test v1 Agent")
	}
	// Verify the Bearer token was sent for card resolution
	if got := captured.Get("Authorization"); got != "Bearer fake-id-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer fake-id-token")
	}
}

func TestResolveCard_GCPAuth_Success(t *testing.T) {
	var captured http.Header
	ts := newHeaderCaptureServer(t, v1CardJSON, &captured)
	defer ts.Close()

	withMockGCPAuth(t, "fake-id-token")

	ctx := context.Background()
	card, err := ResolveCard(ctx, Options{
		BaseURL: ts.URL,
		GCPAuth: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.Name != "Test v1 Agent" {
		t.Errorf("card name: got %q, want %q", card.Name, "Test v1 Agent")
	}
}

// failingTokenSource is an oauth2.TokenSource that always fails, used to
// test the GetToken error path after interceptor construction succeeds.
type failingTokenSource struct{}

func (failingTokenSource) Token() (*oauth2.Token, error) {
	return nil, fmt.Errorf("token refresh failed")
}

func TestNew_GCPAuth_GetTokenError(t *testing.T) {
	ts := newCardServer(t, v1CardJSON)
	defer ts.Close()

	orig := newGCPAuthInterceptorFn
	t.Cleanup(func() { newGCPAuthInterceptorFn = orig })
	newGCPAuthInterceptorFn = func(ctx context.Context, audience string) (*auth.GCPAuthInterceptor, error) {
		return auth.NewGCPAuthInterceptorFromSource(failingTokenSource{}), nil
	}

	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL: ts.URL,
		GCPAuth: true,
	})
	if err == nil {
		t.Fatal("expected error for GetToken failure")
	}
}

// ---------------------------------------------------------------------------
// newStandard: NewFromCard error path
// ---------------------------------------------------------------------------

// TestNew_NewFromCardError exercises the error branch in newStandard where
// a2aclient.NewFromCard fails. This is triggered when the resolved card
// contains interfaces with an unknown/unsupported protocol binding that
// none of the registered transports can handle.
func TestNew_NewFromCardError(t *testing.T) {
	// Serve a card whose only interface uses an unregistered protocol
	// binding, so NewFromCard cannot find a suitable transport.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"name": "Unsupported Agent",
			"description": "Agent with unsupported transport",
			"version": "1.0",
			"capabilities": {},
			"defaultInputModes": ["text/plain"],
			"defaultOutputModes": ["text/plain"],
			"supportedInterfaces": [{
				"url": %q,
				"protocolBinding": "UNSUPPORTED_BINDING",
				"protocolVersion": "99.0"
			}],
			"skills": []
		}`, "http://localhost:1")
	}))
	defer ts.Close()

	ctx := context.Background()
	_, _, err := New(ctx, Options{BaseURL: ts.URL})
	if err == nil {
		t.Fatal("expected error for unsupported transport in NewFromCard")
	}
}

// ---------------------------------------------------------------------------
// resolveStandardCard: NewBearerTokenInterceptor error path
// ---------------------------------------------------------------------------

// TestNew_BearerTokenInterceptorError exercises the error path in
// resolveStandardCard where newBearerTokenInterceptorFn fails. In
// production this is unreachable because the guard (BearerToken != "")
// prevents passing an empty token. This test uses the factory seam to
// force a failure.
func TestNew_BearerTokenInterceptorError(t *testing.T) {
	orig := newBearerTokenInterceptorFn
	t.Cleanup(func() { newBearerTokenInterceptorFn = orig })
	newBearerTokenInterceptorFn = func(token string) (*auth.BearerTokenInterceptor, error) {
		return nil, fmt.Errorf("injected bearer error")
	}

	ts := newCardServer(t, v1CardJSON)
	defer ts.Close()

	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL:     ts.URL,
		BearerToken: "some-token",
	})
	if err == nil {
		t.Fatal("expected error for bearer token interceptor failure")
	}
}

// --- Device Code Auth tests ---

// withMockDeviceCode overrides the device code interceptor factory with one
// that returns a fake interceptor backed by the given token.
func withMockDeviceCode(t *testing.T, token string) {
	t.Helper()
	orig := newDeviceCodeInterceptorFn
	t.Cleanup(func() { newDeviceCodeInterceptorFn = orig })
	newDeviceCodeInterceptorFn = func(ctx context.Context, cfg auth.DeviceCodeConfig, promptOut io.Writer, httpClient *http.Client) (*auth.DeviceCodeInterceptor, error) {
		return auth.NewDeviceCodeInterceptorFromToken(token), nil
	}
}

// withFailingDeviceCode overrides the device code interceptor factory to
// return an error.
func withFailingDeviceCode(t *testing.T) {
	t.Helper()
	orig := newDeviceCodeInterceptorFn
	t.Cleanup(func() { newDeviceCodeInterceptorFn = orig })
	newDeviceCodeInterceptorFn = func(ctx context.Context, cfg auth.DeviceCodeConfig, promptOut io.Writer, httpClient *http.Client) (*auth.DeviceCodeInterceptor, error) {
		return nil, fmt.Errorf("mock device code error")
	}
}

// v1CardWithDeviceCodeJSON returns a v1 card JSON with an OAuth2 security
// scheme containing a DeviceCodeOAuthFlow.
func v1CardWithDeviceCodeJSON(url string) string {
	return fmt.Sprintf(`{
		"name": "Test Device Auth Agent",
		"description": "A v1 test agent with device code auth",
		"version": "1.0",
		"capabilities": {},
		"defaultInputModes": ["text/plain"],
		"defaultOutputModes": ["text/plain"],
		"supportedInterfaces": [{
			"url": %q,
			"protocolBinding": "JSONRPC",
			"protocolVersion": "1.0"
		}],
		"securitySchemes": {
			"myoauth": {
				"oauth2SecurityScheme": {
					"flows": {
						"deviceCode": {
							"deviceAuthorizationUrl": "https://auth.example.com/device",
							"tokenUrl": "https://auth.example.com/token",
							"scopes": {
								"read": "Read access",
								"write": "Write access"
							}
						}
					}
				}
			}
		},
		"securityRequirements": [{"myoauth": ["read"]}],
		"skills": []
	}`, url)
}

func TestNew_DeviceAuth_Success(t *testing.T) {
	ts := newCardServer(t, v1CardWithDeviceCodeJSON)
	defer ts.Close()

	withMockDeviceCode(t, "fake-device-token")

	ctx := context.Background()
	a2aClient, card, err := New(ctx, Options{
		BaseURL:            ts.URL,
		DeviceAuth:         true,
		DeviceAuthClientID: "test-client",
		PromptOutput:       io.Discard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = a2aClient.Destroy() }()

	if card.Name != "Test Device Auth Agent" {
		t.Errorf("got card name %q, want %q", card.Name, "Test Device Auth Agent")
	}
}

func TestNew_DeviceAuth_InterceptorError(t *testing.T) {
	ts := newCardServer(t, v1CardWithDeviceCodeJSON)
	defer ts.Close()

	withFailingDeviceCode(t)

	ctx := context.Background()
	_, _, err := New(ctx, Options{
		BaseURL:            ts.URL,
		DeviceAuth:         true,
		DeviceAuthClientID: "test-client",
		PromptOutput:       io.Discard,
	})
	if err == nil {
		t.Fatal("expected error for device code interceptor failure")
	}
	if !strings.Contains(err.Error(), "device code auth failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNew_DeviceAuth_NoFlowInCard(t *testing.T) {
	// Use a regular card without DeviceCodeOAuthFlow.
	ts := newCardServer(t, v1CardJSON)
	defer ts.Close()

	withMockDeviceCode(t, "fake-token")

	ctx := context.Background()
	// Card has no DeviceCodeOAuthFlow, and no explicit URLs provided.
	// The mock interceptor receives empty URLs in cfg → the interceptor
	// factory would normally fail on validation, but we mock it.
	// The test verifies that the flow still works (the mock ignores cfg).
	a2aClient, _, err := New(ctx, Options{
		BaseURL:            ts.URL,
		DeviceAuth:         true,
		DeviceAuthClientID: "test-client",
		PromptOutput:       io.Discard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = a2aClient.Destroy() }()
}

func TestNew_DeviceAuth_OverrideURLs(t *testing.T) {
	ts := newCardServer(t, v1CardJSON)
	defer ts.Close()

	var capturedCfg auth.DeviceCodeConfig
	orig := newDeviceCodeInterceptorFn
	t.Cleanup(func() { newDeviceCodeInterceptorFn = orig })
	newDeviceCodeInterceptorFn = func(ctx context.Context, cfg auth.DeviceCodeConfig, promptOut io.Writer, httpClient *http.Client) (*auth.DeviceCodeInterceptor, error) {
		capturedCfg = cfg
		return auth.NewDeviceCodeInterceptorFromToken("tok"), nil
	}

	ctx := context.Background()
	a2aClient, _, err := New(ctx, Options{
		BaseURL:            ts.URL,
		DeviceAuth:         true,
		DeviceAuthClientID: "override-client",
		DeviceAuthURL:      "https://override.example.com/device",
		DeviceAuthTokenURL: "https://override.example.com/token",
		DeviceAuthScopes:   []string{"custom"},
		PromptOutput:       io.Discard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = a2aClient.Destroy() }()

	if capturedCfg.ClientID != "override-client" {
		t.Errorf("ClientID: got %q, want %q", capturedCfg.ClientID, "override-client")
	}
	if capturedCfg.DeviceAuthorizationURL != "https://override.example.com/device" {
		t.Errorf("DeviceAuthorizationURL: got %q, want %q", capturedCfg.DeviceAuthorizationURL, "https://override.example.com/device")
	}
	if capturedCfg.TokenURL != "https://override.example.com/token" {
		t.Errorf("TokenURL: got %q, want %q", capturedCfg.TokenURL, "https://override.example.com/token")
	}
	if len(capturedCfg.Scopes) != 1 || capturedCfg.Scopes[0] != "custom" {
		t.Errorf("Scopes: got %v, want [custom]", capturedCfg.Scopes)
	}
}

func TestNew_DeviceAuth_CardAutoDetectsURLs(t *testing.T) {
	ts := newCardServer(t, v1CardWithDeviceCodeJSON)
	defer ts.Close()

	var capturedCfg auth.DeviceCodeConfig
	orig := newDeviceCodeInterceptorFn
	t.Cleanup(func() { newDeviceCodeInterceptorFn = orig })
	newDeviceCodeInterceptorFn = func(ctx context.Context, cfg auth.DeviceCodeConfig, promptOut io.Writer, httpClient *http.Client) (*auth.DeviceCodeInterceptor, error) {
		capturedCfg = cfg
		return auth.NewDeviceCodeInterceptorFromToken("tok"), nil
	}

	ctx := context.Background()
	a2aClient, _, err := New(ctx, Options{
		BaseURL:            ts.URL,
		DeviceAuth:         true,
		DeviceAuthClientID: "my-client",
		PromptOutput:       io.Discard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = a2aClient.Destroy() }()

	if capturedCfg.ClientID != "my-client" {
		t.Errorf("ClientID: got %q, want %q", capturedCfg.ClientID, "my-client")
	}
	if capturedCfg.DeviceAuthorizationURL != "https://auth.example.com/device" {
		t.Errorf("DeviceAuthorizationURL: got %q, want %q", capturedCfg.DeviceAuthorizationURL, "https://auth.example.com/device")
	}
	if capturedCfg.TokenURL != "https://auth.example.com/token" {
		t.Errorf("TokenURL: got %q, want %q", capturedCfg.TokenURL, "https://auth.example.com/token")
	}
	if len(capturedCfg.Scopes) == 0 {
		t.Error("Scopes should be auto-detected from card")
	}
}

func TestFindDeviceCodeFlow_Success(t *testing.T) {
	card := &a2a.AgentCard{
		SecuritySchemes: a2a.NamedSecuritySchemes{
			"myoauth": a2a.OAuth2SecurityScheme{
				Flows: a2a.DeviceCodeOAuthFlow{
					DeviceAuthorizationURL: "https://auth.example.com/device",
					TokenURL:               "https://auth.example.com/token",
					Scopes:                 map[string]string{"read": "Read", "write": "Write"},
				},
			},
		},
	}

	cfg, err := findDeviceCodeFlow(card)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DeviceAuthorizationURL != "https://auth.example.com/device" {
		t.Errorf("DeviceAuthorizationURL: got %q, want %q", cfg.DeviceAuthorizationURL, "https://auth.example.com/device")
	}
	if cfg.TokenURL != "https://auth.example.com/token" {
		t.Errorf("TokenURL: got %q, want %q", cfg.TokenURL, "https://auth.example.com/token")
	}
	if len(cfg.Scopes) != 2 {
		t.Errorf("Scopes: got %d, want 2", len(cfg.Scopes))
	}
}

func TestFindDeviceCodeFlow_NoOAuth2(t *testing.T) {
	card := &a2a.AgentCard{
		SecuritySchemes: a2a.NamedSecuritySchemes{
			"apikey": a2a.APIKeySecurityScheme{Name: "X-API-Key"},
		},
	}

	_, err := findDeviceCodeFlow(card)
	if err == nil {
		t.Fatal("expected error when no OAuth2 scheme exists")
	}
}

func TestFindDeviceCodeFlow_NoDeviceCode(t *testing.T) {
	card := &a2a.AgentCard{
		SecuritySchemes: a2a.NamedSecuritySchemes{
			"myoauth": a2a.OAuth2SecurityScheme{
				Flows: a2a.AuthorizationCodeOAuthFlow{
					AuthorizationURL: "https://auth.example.com/authorize",
					TokenURL:         "https://auth.example.com/token",
				},
			},
		},
	}

	_, err := findDeviceCodeFlow(card)
	if err == nil {
		t.Fatal("expected error when no DeviceCode flow exists")
	}
}

func TestFindDeviceCodeFlow_NilCard(t *testing.T) {
	_, err := findDeviceCodeFlow(nil)
	if err == nil {
		t.Fatal("expected error for nil card")
	}
}

func TestFindDeviceCodeFlow_NoSchemes(t *testing.T) {
	card := &a2a.AgentCard{}
	_, err := findDeviceCodeFlow(card)
	if err == nil {
		t.Fatal("expected error for card with no security schemes")
	}
}
