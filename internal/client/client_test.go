package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew_WithoutGCPAuth(t *testing.T) {
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"name": "Test Agent",
			"description": "A test agent",
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
		}`, ts.URL)
	})
	ts = httptest.NewServer(mux)
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

	if card.Name != "Test Agent" {
		t.Errorf("got card name %q, want %q", card.Name, "Test Agent")
	}
	if card.Description != "A test agent" {
		t.Errorf("got card description %q, want %q", card.Description, "A test agent")
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
