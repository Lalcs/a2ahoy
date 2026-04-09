package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Lalcs/a2ahoy/internal/updater"
	"github.com/Lalcs/a2ahoy/internal/version"
	pflag "github.com/spf13/pflag"
)

// resetGlobalFlags resets all package-level flag variables to their zero
// values. This MUST be called before every test that invokes rootCmd.Execute()
// because cobra does not reset parsed flags between runs in the same process.
func resetGlobalFlags(t *testing.T) {
	t.Helper()
	flagGCPAuth = false
	flagJSON = false
	flagVertexAI = false
	flagV03RESTMount = false
	flagNoColor = false
	flagHeaders = nil
	flagBearerToken = ""
	flagUpdateCheckOnly = false
	flagUpdateForce = false
	flagChatSimple = false
	// Prevent env var leakage from ambient environment.
	t.Setenv(bearerTokenEnvVar, "")
	// Reset Changed state on subcommand-local flags so tests are
	// order-independent. Cobra does not clear this between Execute() calls.
	for _, cmd := range rootCmd.Commands() {
		cmd.Flags().Visit(func(f *pflag.Flag) { f.Changed = false })
	}
}

// v1CardJSON returns a minimal A2A spec v1.0 agent card JSON whose
// supportedInterfaces URL points to the given server URL.
func v1CardJSON(serverURL string) string {
	return fmt.Sprintf(`{
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
		"skills": [{"id":"echo","name":"Echo","description":"Echoes the input"}]
	}`, serverURL)
}

// jsonRPCResponse builds a JSON-RPC 2.0 success response wrapping the
// given result value, using the id from the request.
func jsonRPCResponse(id string, result json.RawMessage) []byte {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	b, _ := json.Marshal(resp)
	return b
}

// taskResultJSON returns a valid StreamResponse-wrapped Task JSON with
// the given id and state. SendMessage returns this envelope format.
func taskResultJSON(taskID, contextID, state string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"task": {
			"id": %q,
			"contextId": %q,
			"status": {"state": %q},
			"history": [
				{"role": "ROLE_USER", "parts": [{"text": "hello"}]},
				{"role": "ROLE_AGENT", "parts": [{"text": "hi there"}]}
			]
		}
	}`, taskID, contextID, state))
}

// rawTaskJSON returns a raw Task JSON (no StreamResponse wrapper) for
// GetTask and CancelTask responses.
func rawTaskJSON(taskID, contextID, state string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"id": %q,
		"contextId": %q,
		"status": {"state": %q}
	}`, taskID, contextID, state))
}

// a2aTestServer returns an httptest.Server that serves a valid v1 agent
// card and handles JSON-RPC A2A protocol requests (send, stream, get, cancel).
func a2aTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	var ts *httptest.Server
	mux := http.NewServeMux()

	// Card endpoint.
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, v1CardJSON(ts.URL))
	})

	// JSON-RPC endpoint at the root URL (matches supportedInterfaces URL).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
			ID      string          `json:"id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad json-rpc", http.StatusBadRequest)
			return
		}

		switch req.Method {
		case "SendMessage":
			w.Header().Set("Content-Type", "application/json")
			result := taskResultJSON("task-send-1", "ctx-1", "TASK_STATE_COMPLETED")
			w.Write(jsonRPCResponse(req.ID, result))

		case "SendStreamingMessage":
			// Return SSE events. Each SSE data line contains a full JSON-RPC
			// response. The a2a-go library parses "data: <json>\n\n" lines.
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "no flusher", http.StatusInternalServerError)
				return
			}

			// Emit a status update event.
			statusResult := json.RawMessage(`{
				"statusUpdate": {
					"id": "task-stream-1",
					"contextId": "ctx-stream",
					"status": {"state": "TASK_STATE_WORKING"}
				}
			}`)
			line1 := jsonRPCResponse(req.ID, statusResult)
			fmt.Fprintf(w, "data: %s\n\n", line1)
			flusher.Flush()

			// Emit a final task event.
			taskResult := json.RawMessage(`{
				"task": {
					"id": "task-stream-1",
					"contextId": "ctx-stream",
					"status": {"state": "TASK_STATE_COMPLETED"},
					"history": [
						{"role": "ROLE_AGENT", "parts": [{"text": "streamed reply"}]}
					]
				}
			}`)
			line2 := jsonRPCResponse(req.ID, taskResult)
			fmt.Fprintf(w, "data: %s\n\n", line2)
			flusher.Flush()

		case "GetTask":
			w.Header().Set("Content-Type", "application/json")
			result := rawTaskJSON("task-get-1", "ctx-get", "TASK_STATE_COMPLETED")
			w.Write(jsonRPCResponse(req.ID, result))

		case "CancelTask":
			w.Header().Set("Content-Type", "application/json")
			result := rawTaskJSON("task-cancel-1", "ctx-cancel", "TASK_STATE_CANCELED")
			w.Write(jsonRPCResponse(req.ID, result))

		case "ListTasks":
			w.Header().Set("Content-Type", "application/json")
			result := json.RawMessage(`{
				"tasks": [
					{"id": "task-list-1", "contextId": "ctx-list-1", "status": {"state": "TASK_STATE_COMPLETED"}},
					{"id": "task-list-2", "contextId": "ctx-list-2", "status": {"state": "TASK_STATE_WORKING"}}
				],
				"totalSize": 2,
				"pageSize": 50,
				"nextPageToken": ""
			}`)
			w.Write(jsonRPCResponse(req.ID, result))

		default:
			w.Header().Set("Content-Type", "application/json")
			errResp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"error":{"code":-32601,"message":"method not found"}}`, req.ID)
			w.Write([]byte(errResp))
		}
	})

	ts = httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// ---------------------------------------------------------------------------
// Execute
// ---------------------------------------------------------------------------

func TestExecute(t *testing.T) {
	resetGlobalFlags(t)
	rootCmd.SetArgs([]string{})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	if err := Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runCard
// ---------------------------------------------------------------------------

func TestRunCard_HumanReadable(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"card", ts.URL})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runCard human-readable failed: %v", err)
	}
}

func TestRunCard_JSON(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--json", "card", ts.URL})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runCard --json failed: %v", err)
	}
}

func TestRunCard_InvalidURL(t *testing.T) {
	resetGlobalFlags(t)
	rootCmd.SetArgs([]string{"card", "http://127.0.0.1:1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

// TestRunCard_ValidationError verifies that the card command exits non-zero
// when the agent card has validation errors.
func TestRunCard_ValidationError(t *testing.T) {
	resetGlobalFlags(t)

	// Serve a card with an empty name which triggers a validation error.
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"name": "",
			"description": "bad card",
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
	t.Cleanup(ts.Close)

	rootCmd.SetArgs([]string{"card", ts.URL})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected validation error for card with empty name")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

// TestRunCard_ValidationError_JSON verifies that validation errors are
// surfaced even in --json mode.
func TestRunCard_ValidationError_JSON(t *testing.T) {
	resetGlobalFlags(t)

	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"name": "",
			"description": "bad card",
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
	t.Cleanup(ts.Close)

	rootCmd.SetArgs([]string{"--json", "card", ts.URL})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected validation error for card with empty name in --json mode")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runSend
// ---------------------------------------------------------------------------

func TestRunSend_HumanReadable(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"send", ts.URL, "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runSend human-readable failed: %v", err)
	}
}

func TestRunSend_JSON(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--json", "send", ts.URL, "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runSend --json failed: %v", err)
	}
}

func TestRunSend_InvalidURL(t *testing.T) {
	resetGlobalFlags(t)
	rootCmd.SetArgs([]string{"send", "http://127.0.0.1:1", "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

func TestRunSend_ServerError(t *testing.T) {
	resetGlobalFlags(t)

	// Server serves a valid card but returns a JSON-RPC error for SendMessage.
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, v1CardJSON(ts.URL))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID string `json:"id"`
		}
		json.Unmarshal(body, &req)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%q,"error":{"code":-32603,"message":"internal error"}}`, req.ID)
	})
	ts = httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	rootCmd.SetArgs([]string{"send", ts.URL, "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for server error response")
	}
	if !strings.Contains(err.Error(), "SendMessage failed") {
		t.Errorf("expected SendMessage failed error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runStream
// ---------------------------------------------------------------------------

func TestRunStream_HumanReadable(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"stream", ts.URL, "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runStream human-readable failed: %v", err)
	}
}

func TestRunStream_JSON(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--json", "stream", ts.URL, "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runStream --json failed: %v", err)
	}
}

func TestRunStream_InvalidURL(t *testing.T) {
	resetGlobalFlags(t)
	rootCmd.SetArgs([]string{"stream", "http://127.0.0.1:1", "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

func TestRunStream_ServerError(t *testing.T) {
	resetGlobalFlags(t)

	// Server serves a valid card but returns SSE with a JSON-RPC error.
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, v1CardJSON(ts.URL))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID string `json:"id"`
		}
		json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		errResp := fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"error":{"code":-32603,"message":"stream error"}}`, req.ID)
		fmt.Fprintf(w, "data: %s\n\n", errResp)
		flusher.Flush()
	})
	ts = httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	rootCmd.SetArgs([]string{"stream", ts.URL, "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for SSE error")
	}
	if !strings.Contains(err.Error(), "stream error") {
		t.Errorf("expected stream error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runGet
// ---------------------------------------------------------------------------

func TestRunGet_HumanReadable(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"get", ts.URL, "task-get-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runGet human-readable failed: %v", err)
	}
}

func TestRunGet_JSON(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--json", "get", ts.URL, "task-get-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runGet --json failed: %v", err)
	}
}

func TestRunGet_WithHistoryLength(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"get", "--history-length", "5", ts.URL, "task-get-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runGet --history-length failed: %v", err)
	}
}

func TestRunGet_InvalidURL(t *testing.T) {
	resetGlobalFlags(t)
	rootCmd.SetArgs([]string{"get", "http://127.0.0.1:1", "task-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

func TestRunGet_ServerError(t *testing.T) {
	resetGlobalFlags(t)

	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, v1CardJSON(ts.URL))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID string `json:"id"`
		}
		json.Unmarshal(body, &req)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%q,"error":{"code":-32001,"message":"task not found"}}`, req.ID)
	})
	ts = httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	rootCmd.SetArgs([]string{"get", ts.URL, "nonexistent"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for get with server error")
	}
	if !strings.Contains(err.Error(), "GetTask failed") {
		t.Errorf("expected GetTask failed error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runCancel
// ---------------------------------------------------------------------------

func TestRunCancel_HumanReadable(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"cancel", ts.URL, "task-cancel-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runCancel human-readable failed: %v", err)
	}
}

func TestRunCancel_JSON(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--json", "cancel", ts.URL, "task-cancel-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runCancel --json failed: %v", err)
	}
}

func TestRunCancel_InvalidURL(t *testing.T) {
	resetGlobalFlags(t)
	rootCmd.SetArgs([]string{"cancel", "http://127.0.0.1:1", "task-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

func TestRunCancel_ServerError(t *testing.T) {
	resetGlobalFlags(t)

	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, v1CardJSON(ts.URL))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID string `json:"id"`
		}
		json.Unmarshal(body, &req)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%q,"error":{"code":-32002,"message":"already completed"}}`, req.ID)
	})
	ts = httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	rootCmd.SetArgs([]string{"cancel", ts.URL, "task-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for cancel with server error")
	}
	if !strings.Contains(err.Error(), "CancelTask failed") {
		t.Errorf("expected CancelTask failed error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runList
// ---------------------------------------------------------------------------

func TestRunList_HumanReadable(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	var buf strings.Builder
	rootCmd.SetArgs([]string{"list", ts.URL})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runList human-readable failed: %v", err)
	}

	got := buf.String()
	checks := []string{
		"Tasks (2 of 2 total)",
		"task-list-1",
		"task-list-2",
		"TASK_STATE_COMPLETED",
		"TASK_STATE_WORKING",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestRunList_JSON(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	var buf strings.Builder
	rootCmd.SetArgs([]string{"--json", "list", ts.URL})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runList --json failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"task-list-1"`) {
		t.Errorf("missing task-list-1 in JSON output:\n%s", got)
	}
	if !strings.Contains(got, `"totalSize"`) {
		t.Errorf("missing totalSize in JSON output:\n%s", got)
	}
}

func TestRunList_WithFilters(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{
		"list", ts.URL,
		"--context-id", "ctx-list-1",
		"--status", "TASK_STATE_COMPLETED",
		"--page-size", "10",
	})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runList with filters failed: %v", err)
	}
}

func TestRunList_WithHistoryLength(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"list", ts.URL, "--history-length", "5"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runList --history-length failed: %v", err)
	}
}

func TestRunList_WithStatusAfter(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"list", ts.URL, "--status-after", "2026-01-01T00:00:00Z"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runList --status-after failed: %v", err)
	}
}

func TestRunList_InvalidStatusAfter(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"list", ts.URL, "--status-after", "not-a-date"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --status-after")
	}
	if !strings.Contains(err.Error(), "invalid --status-after") {
		t.Errorf("expected invalid --status-after error, got: %v", err)
	}
}

func TestRunList_InvalidURL(t *testing.T) {
	resetGlobalFlags(t)
	rootCmd.SetArgs([]string{"list", "http://127.0.0.1:1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

func TestRunList_ServerError(t *testing.T) {
	resetGlobalFlags(t)

	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, v1CardJSON(ts.URL))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID string `json:"id"`
		}
		json.Unmarshal(body, &req)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%q,"error":{"code":-32603,"message":"internal error"}}`, req.ID)
	})
	ts = httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	rootCmd.SetArgs([]string{"list", ts.URL})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for list with server error")
	}
	if !strings.Contains(err.Error(), "ListTasks failed") {
		t.Errorf("expected ListTasks failed error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runUpdate
// ---------------------------------------------------------------------------

// fakeFetcher implements updater.Fetcher for testing.
type fakeFetcher struct {
	release *updater.Release
	err     error
}

func (f *fakeFetcher) FetchLatestRelease(_ context.Context) (*updater.Release, error) {
	return f.release, f.err
}

// fakeAssetServer starts an httptest server that serves a fake binary
// for download.
func fakeAssetServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Write a fake binary (just some bytes).
		w.Write([]byte("#!/bin/sh\necho fake\n"))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestRunUpdate_UpToDate(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v1.0.0"

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v1.0.0",
				Name:    "v1.0.0",
			},
		}
	}

	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runUpdate up-to-date failed: %v", err)
	}
}

func TestRunUpdate_FetchError(t *testing.T) {
	resetGlobalFlags(t)

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			err: fmt.Errorf("network unreachable"),
		}
	}

	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for fetch failure")
	}
	if !strings.Contains(err.Error(), "failed to fetch latest release") {
		t.Errorf("expected fetch error, got: %v", err)
	}
}

func TestRunUpdate_Ahead(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v2.0.0"

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v1.0.0",
				Name:    "v1.0.0",
			},
		}
	}

	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runUpdate ahead failed: %v", err)
	}
}

func TestRunUpdate_InvalidLatest(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v1.0.0"

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "not-semver",
				Name:    "not-semver",
			},
		}
	}

	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid latest version")
	}
	if !strings.Contains(err.Error(), "cannot determine latest version") {
		t.Errorf("expected invalid version error, got: %v", err)
	}
}

func TestRunUpdate_CheckOnly(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v1.0.0"

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v2.0.0",
				Name:    "v2.0.0",
			},
		}
	}

	rootCmd.SetArgs([]string{"update", "--check-only"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runUpdate --check-only failed: %v", err)
	}
}

func TestRunUpdate_DevBuild_CheckOnly(t *testing.T) {
	resetGlobalFlags(t)

	// dev builds always want to install; --check-only stops before download.
	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "dev"

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v1.0.0",
				Name:    "v1.0.0",
			},
		}
	}

	rootCmd.SetArgs([]string{"update", "--check-only"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runUpdate dev+check-only failed: %v", err)
	}
}

func TestRunUpdate_MissingAsset(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v1.0.0"

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v2.0.0",
				Name:    "v2.0.0",
				// No assets at all — FindAssetForPlatform will fail.
				Assets: []updater.Asset{},
			},
		}
	}

	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing platform asset")
	}
	if !strings.Contains(err.Error(), "no asset named") {
		t.Errorf("expected 'no asset named' error, got: %v", err)
	}
}

func TestRunUpdate_FullInstall(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v1.0.0"

	// Create a fake "current binary" in a temp dir.
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "a2ahoy")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}

	// Set up a download server.
	dlSrv := fakeAssetServer(t)

	assetName := fmt.Sprintf("a2ahoy-%s-%s", runtime.GOOS, runtime.GOARCH)

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v2.0.0",
				Name:    "v2.0.0",
				Assets: []updater.Asset{
					{
						Name:               assetName,
						BrowserDownloadURL: dlSrv.URL + "/" + assetName,
						Size:               20,
					},
				},
			},
		}
	}

	oldInstaller := makeUpdateInstaller
	t.Cleanup(func() { makeUpdateInstaller = oldInstaller })
	makeUpdateInstaller = func() *updater.Installer {
		return updater.NewInstallerForTest(fakeBin)
	}

	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runUpdate full install failed: %v", err)
	}

	// Verify the binary was replaced.
	content, err := os.ReadFile(fakeBin)
	if err != nil {
		t.Fatalf("read updated binary: %v", err)
	}
	if string(content) == "#!/bin/sh\necho old\n" {
		t.Error("binary was not replaced")
	}
}

func TestRunUpdate_ForceReinstall(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v1.0.0"

	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "a2ahoy")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}

	dlSrv := fakeAssetServer(t)
	assetName := fmt.Sprintf("a2ahoy-%s-%s", runtime.GOOS, runtime.GOARCH)

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v1.0.0",
				Name:    "v1.0.0",
				Assets: []updater.Asset{
					{
						Name:               assetName,
						BrowserDownloadURL: dlSrv.URL + "/" + assetName,
						Size:               20,
					},
				},
			},
		}
	}

	oldInstaller := makeUpdateInstaller
	t.Cleanup(func() { makeUpdateInstaller = oldInstaller })
	makeUpdateInstaller = func() *updater.Installer {
		return updater.NewInstallerForTest(fakeBin)
	}

	rootCmd.SetArgs([]string{"update", "--force"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runUpdate --force failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PersistentPreRunE - no-color flag
// ---------------------------------------------------------------------------

func TestRootCommand_NoColorFlag(t *testing.T) {
	resetGlobalFlags(t)

	rootCmd.SetArgs([]string{"--no-color", "version"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// v03-rest-mount flag integration
// ---------------------------------------------------------------------------

func TestRunSend_WithV03RESTMount(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--v03-rest-mount", "send", ts.URL, "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	// The v03-rest-mount flag only modifies v0.3 HTTP+JSON interfaces.
	// Our test card uses JSONRPC v1.0 so the rewrite is a no-op.
	// This test verifies the flag is accepted without error.
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runSend --v03-rest-mount failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// bearer-token flag passthrough
// ---------------------------------------------------------------------------

func TestRunSend_WithBearerToken(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--bearer-token", "test-token", "send", ts.URL, "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runSend --bearer-token failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// header flag passthrough
// ---------------------------------------------------------------------------

func TestRunSend_WithHeaders(t *testing.T) {
	resetGlobalFlags(t)

	// Verify custom headers are forwarded. We check a header on the test
	// server to make sure it arrives.
	var ts *httptest.Server
	var receivedHeader string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, v1CardJSON(ts.URL))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID string `json:"id"`
		}
		json.Unmarshal(body, &req)
		result := taskResultJSON("task-1", "ctx-1", "TASK_STATE_COMPLETED")
		w.Write(jsonRPCResponse(req.ID, result))
	})
	ts = httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	rootCmd.SetArgs([]string{"--header", "X-Custom=hello-world", "send", ts.URL, "hi"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runSend --header failed: %v", err)
	}
	if receivedHeader != "hello-world" {
		t.Errorf("custom header not received: got %q, want %q", receivedHeader, "hello-world")
	}
}

func TestRunCard_WithBearerToken(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--bearer-token", "test-token", "card", ts.URL})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runCard --bearer-token failed: %v", err)
	}
}

func TestRunCard_WithHeaders(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--header", "X-Test=value", "card", ts.URL})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runCard --header failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Writer-error paths
// ---------------------------------------------------------------------------

// errWriter is an io.Writer that always returns an error. It is used to
// trigger the presenter error branches in run* functions.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write error")
}

func TestRunCard_PrintJSONError(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--json", "card", ts.URL})
	rootCmd.SetOut(errWriter{})
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when writer fails")
	}
}

func TestRunStream_PrintJSONError(t *testing.T) {
	resetGlobalFlags(t)
	ts := a2aTestServer(t)

	rootCmd.SetArgs([]string{"--json", "stream", ts.URL, "hello"})
	rootCmd.SetOut(errWriter{})
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when writer fails during stream JSON")
	}
}

// ---------------------------------------------------------------------------
// Stream context cancellation
// ---------------------------------------------------------------------------

// TestRunStream_Interrupted tests the ctx.Err() != nil branch by
// overriding newStreamContext with a context that is cancelled after
// the A2A client is created but before the streaming read completes.
func TestRunStream_Interrupted(t *testing.T) {
	resetGlobalFlags(t)

	// Server that sends one SSE event then hangs.
	started := make(chan struct{})
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, v1CardJSON(ts.URL))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID string `json:"id"`
		}
		json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		statusResult := json.RawMessage(`{
			"statusUpdate": {
				"id": "task-int",
				"contextId": "ctx-int",
				"status": {"state": "TASK_STATE_WORKING"}
			}
		}`)
		line := jsonRPCResponse(req.ID, statusResult)
		fmt.Fprintf(w, "data: %s\n\n", line)
		flusher.Flush()

		// Signal that we sent the first event.
		close(started)
		// Block until the client disconnects.
		<-r.Context().Done()
	})
	ts = httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Create a context we can cancel programmatically.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	oldStreamCtx := newStreamContext
	t.Cleanup(func() { newStreamContext = oldStreamCtx })
	newStreamContext = func() (context.Context, context.CancelFunc) {
		return ctx, cancel
	}

	// Cancel the context once the server has sent the first event.
	go func() {
		<-started
		cancel()
	}()

	rootCmd.SetArgs([]string{"stream", ts.URL, "hello"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	// The stream should return nil (not error) when interrupted.
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected nil error for interrupted stream, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Update: detectPlatform error and default factories
// ---------------------------------------------------------------------------

func TestRunUpdate_UnsupportedPlatform(t *testing.T) {
	resetGlobalFlags(t)

	oldDetect := detectPlatform
	t.Cleanup(func() { detectPlatform = oldDetect })
	detectPlatform = func() (updater.SupportedPlatform, error) {
		return updater.SupportedPlatform{}, fmt.Errorf("unsupported OS %q for self-update", "plan9")
	}

	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "unsupported OS") {
		t.Errorf("expected platform error, got: %v", err)
	}
}

// defaultMakeUpdateFetcher, defaultMakeUpdateInstaller,
// defaultDetectPlatform, and defaultNewStreamContext capture the
// original package-level factory functions at init time so
// TestDefaultFactoryFunctions can exercise them even after other
// tests have overridden the package-level vars.
var (
	defaultMakeUpdateFetcher   = makeUpdateFetcher
	defaultMakeUpdateInstaller = makeUpdateInstaller
	defaultDetectPlatform      = detectPlatform
	defaultNewStreamContext    = newStreamContext
)

func TestDefaultFactoryFunctions(t *testing.T) {
	// Exercise the original factory closures from the var declarations
	// in update.go and stream.go. These are captured at init time before
	// any test can override the package-level variables.
	if defaultMakeUpdateFetcher() == nil {
		t.Error("default fetcher factory returned nil")
	}
	if defaultMakeUpdateInstaller() == nil {
		t.Error("default installer factory returned nil")
	}
	plat, err := defaultDetectPlatform()
	if err != nil {
		t.Fatalf("default platform detector failed: %v", err)
	}
	if plat.OS == "" || plat.Arch == "" {
		t.Error("default platform detector returned empty fields")
	}
	ctx, cancel := defaultNewStreamContext()
	if ctx == nil {
		t.Error("default stream context factory returned nil context")
	}
	cancel()
}

// ---------------------------------------------------------------------------
// runUpdate - installer error paths
// ---------------------------------------------------------------------------

func TestRunUpdate_PrepareError(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v1.0.0"

	assetName := fmt.Sprintf("a2ahoy-%s-%s", runtime.GOOS, runtime.GOARCH)

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v2.0.0",
				Name:    "v2.0.0",
				Assets: []updater.Asset{
					{
						Name:               assetName,
						BrowserDownloadURL: "http://127.0.0.1:1/" + assetName,
						Size:               20,
					},
				},
			},
		}
	}

	// Point the installer at a non-existent path to trigger Prepare error.
	oldInstaller := makeUpdateInstaller
	t.Cleanup(func() { makeUpdateInstaller = oldInstaller })
	makeUpdateInstaller = func() *updater.Installer {
		return updater.NewInstallerForTest("/nonexistent/path/a2ahoy")
	}

	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for Prepare failure")
	}
}

func TestRunUpdate_InstallError(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v1.0.0"

	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "a2ahoy")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}

	// Download server that returns 500.
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(dlSrv.Close)

	assetName := fmt.Sprintf("a2ahoy-%s-%s", runtime.GOOS, runtime.GOARCH)

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v2.0.0",
				Name:    "v2.0.0",
				Assets: []updater.Asset{
					{
						Name:               assetName,
						BrowserDownloadURL: dlSrv.URL + "/" + assetName,
						Size:               20,
					},
				},
			},
		}
	}

	oldInstaller := makeUpdateInstaller
	t.Cleanup(func() { makeUpdateInstaller = oldInstaller })
	makeUpdateInstaller = func() *updater.Installer {
		return updater.NewInstallerForTest(fakeBin)
	}

	rootCmd.SetArgs([]string{"update"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for Install failure")
	}
}

func TestRunUpdate_ForceReinstall_CheckOnly(t *testing.T) {
	resetGlobalFlags(t)

	original := version.Version
	t.Cleanup(func() { version.Version = original })
	version.Version = "v1.0.0"

	oldFetcher := makeUpdateFetcher
	t.Cleanup(func() { makeUpdateFetcher = oldFetcher })
	makeUpdateFetcher = func() updater.Fetcher {
		return &fakeFetcher{
			release: &updater.Release{
				TagName: "v1.0.0",
				Name:    "v1.0.0",
			},
		}
	}

	// --force + --check-only: Force says install, but check-only stops it.
	rootCmd.SetArgs([]string{"update", "--force", "--check-only"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("runUpdate --force --check-only failed: %v", err)
	}
}
