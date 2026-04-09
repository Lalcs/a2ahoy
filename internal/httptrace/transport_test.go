package httptrace

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerboseTransport_DumpsRequestAndResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	client := WrapClient(ts.Client(), &buf)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/test", strings.NewReader("ping"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Verify response passed through unmodified.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "pong" {
		t.Errorf("body: got %q, want %q", body, "pong")
	}

	// Verify dumps appeared in the output buffer.
	out := buf.String()
	if !strings.Contains(out, "--- REQUEST ---") {
		t.Error("output missing REQUEST separator")
	}
	if !strings.Contains(out, "--- RESPONSE ---") {
		t.Error("output missing RESPONSE separator")
	}
	if !strings.Contains(out, "POST /test") {
		t.Error("output missing request method/path")
	}
	if !strings.Contains(out, "ping") {
		t.Error("output missing request body")
	}
	if !strings.Contains(out, "pong") {
		t.Error("output missing response body")
	}
}

func TestVerboseTransport_PropagatesError(t *testing.T) {
	var buf bytes.Buffer
	client := WrapClient(nil, &buf)

	// Request to a closed server should fail.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ts.URL
	ts.Close()

	_, err := client.Get(url) //nolint:bodyclose
	if err == nil {
		t.Fatal("expected error for closed server")
	}

	out := buf.String()
	if !strings.Contains(out, "--- REQUEST ---") {
		t.Error("output missing REQUEST separator on error path")
	}
	if !strings.Contains(out, "--- ERROR ---") {
		t.Error("output missing ERROR separator")
	}
}

func TestVerboseTransport_StreamingResponseDumpsHeadersOnly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Write SSE data that should NOT appear in the dump.
		_, _ = w.Write([]byte("data: hello\n\n"))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	client := WrapClient(ts.Client(), &buf)

	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Body must still be readable (not consumed by DumpResponse).
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "data: hello") {
		t.Errorf("body should contain SSE data, got %q", body)
	}

	out := buf.String()
	if !strings.Contains(out, "--- RESPONSE ---") {
		t.Error("output missing RESPONSE separator")
	}
	// Headers should be present.
	if !strings.Contains(out, "text/event-stream") {
		t.Error("output missing Content-Type header for SSE response")
	}
	// Body should NOT be in the dump (headers-only for streaming).
	if strings.Contains(out, "data: hello") {
		t.Error("output should not contain SSE body data")
	}
}

func TestWrapClient_NilClient(t *testing.T) {
	var buf bytes.Buffer
	client := WrapClient(nil, &buf)
	if client == nil {
		t.Fatal("WrapClient(nil) returned nil")
	}
	vt, ok := client.Transport.(*VerboseTransport)
	if !ok {
		t.Fatal("transport is not *VerboseTransport")
	}
	if vt.Inner != nil {
		t.Error("inner transport should be nil (falls back to DefaultTransport)")
	}
}

func TestWrapClient_PreservesTimeout(t *testing.T) {
	original := &http.Client{Timeout: 42}
	var buf bytes.Buffer
	wrapped := WrapClient(original, &buf)

	if wrapped.Timeout != 42 {
		t.Errorf("timeout: got %v, want 42", wrapped.Timeout)
	}
	// Original must not be mutated.
	if _, ok := original.Transport.(*VerboseTransport); ok {
		t.Error("original client transport was mutated")
	}
}
