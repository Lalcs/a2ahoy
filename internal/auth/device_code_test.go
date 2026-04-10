package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// withInstantPoll overrides pollSleepFn to skip real sleeps in tests.
func withInstantPoll(t *testing.T) {
	t.Helper()
	orig := pollSleepFn
	t.Cleanup(func() { pollSleepFn = orig })
	pollSleepFn = func(ctx context.Context, _ time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
}

// newDeviceCodeServer creates a test server that handles both the device
// authorization endpoint (/device/code) and the token endpoint (/token).
// pendingCount controls how many "authorization_pending" responses the
// token endpoint returns before succeeding. slowDownAt, if > 0, causes
// a "slow_down" response on that specific poll attempt (1-indexed).
// finalError, if non-empty, replaces the eventual success with an error.
func newDeviceCodeServer(t *testing.T, pendingCount int, slowDownAt int, finalError string) *httptest.Server {
	t.Helper()

	var pollCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/device/code", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			http.Error(w, "bad content type", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:      "test-device-code",
			UserCode:        "TEST-CODE",
			VerificationURI: "https://example.com/device",
			ExpiresIn:       300,
			Interval:        0, // use default (5s), tests override via short interval
		})
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		count := int(pollCount.Add(1))

		// Check for slow_down at specific poll attempt.
		if slowDownAt > 0 && count == slowDownAt {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(tokenErrorResponse{Error: "slow_down"})
			return
		}

		// Return authorization_pending until pendingCount is exhausted.
		if count <= pendingCount {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(tokenErrorResponse{Error: "authorization_pending"})
			return
		}

		// Return final error if specified.
		if finalError != "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(tokenErrorResponse{Error: finalError})
			return
		}

		// Success.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
		})
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestNewDeviceCodeInterceptor_Success(t *testing.T) {
	withInstantPoll(t)
	ts := newDeviceCodeServer(t, 0, 0, "")

	var promptBuf bytes.Buffer
	interceptor, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		ts.Client(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	token, err := interceptor.GetToken()
	if err != nil {
		t.Fatalf("GetToken() error: %v", err)
	}
	if token != "test-access-token" {
		t.Errorf("token: got %q, want %q", token, "test-access-token")
	}

	// Verify prompt output contains success message.
	if !strings.Contains(promptBuf.String(), "Authentication successful!") {
		t.Errorf("prompt output missing success message: %s", promptBuf.String())
	}
}

func TestNewDeviceCodeInterceptor_PollPending(t *testing.T) {
	withInstantPoll(t)
	ts := newDeviceCodeServer(t, 3, 0, "")

	var promptBuf bytes.Buffer
	interceptor, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		ts.Client(),
	)
	if err != nil {
		t.Fatalf("unexpected error after pending polls: %v", err)
	}

	token, _ := interceptor.GetToken()
	if token != "test-access-token" {
		t.Errorf("token: got %q, want %q", token, "test-access-token")
	}
}

func TestNewDeviceCodeInterceptor_SlowDown(t *testing.T) {
	withInstantPoll(t)
	// Poll 1: pending, Poll 2: slow_down, Poll 3: pending, Poll 4: success.
	ts := newDeviceCodeServer(t, 3, 2, "")

	var promptBuf bytes.Buffer
	interceptor, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		ts.Client(),
	)
	if err != nil {
		t.Fatalf("unexpected error after slow_down: %v", err)
	}

	token, _ := interceptor.GetToken()
	if token != "test-access-token" {
		t.Errorf("token: got %q, want %q", token, "test-access-token")
	}
}

func TestNewDeviceCodeInterceptor_Expired(t *testing.T) {
	withInstantPoll(t)
	// Server always returns authorization_pending; expiresIn=1 causes timeout.
	mux := http.NewServeMux()
	mux.HandleFunc("/device/code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:      "test-device-code",
			UserCode:        "TEST-CODE",
			VerificationURI: "https://example.com/device",
			ExpiresIn:       1,
			Interval:        0,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(tokenErrorResponse{Error: "authorization_pending"})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	var promptBuf bytes.Buffer
	_, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		ts.Client(),
	)
	if !errors.Is(err, ErrDeviceCodeExpired) {
		t.Fatalf("expected ErrDeviceCodeExpired, got: %v", err)
	}
}

func TestNewDeviceCodeInterceptor_AccessDenied(t *testing.T) {
	withInstantPoll(t)
	ts := newDeviceCodeServer(t, 0, 0, "access_denied")

	var promptBuf bytes.Buffer
	_, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		ts.Client(),
	)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got: %v", err)
	}
}

func TestNewDeviceCodeInterceptor_DeviceAuthEndpointError(t *testing.T) {
	withInstantPoll(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	var promptBuf bytes.Buffer
	_, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		ts.Client(),
	)
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("expected error to mention HTTP 500, got: %v", err)
	}
}

func TestNewDeviceCodeInterceptor_ContextCancelled(t *testing.T) {
	// Do NOT use withInstantPoll here — we need the real sleep to detect cancellation.
	// Instead, override pollSleepFn with one that respects ctx.Done() immediately.
	orig := pollSleepFn
	t.Cleanup(func() { pollSleepFn = orig })
	pollSleepFn = func(ctx context.Context, _ time.Duration) error {
		// Block until cancelled (simulates a long sleep interrupted by cancel).
		<-ctx.Done()
		return ctx.Err()
	}
	// Server always returns authorization_pending.
	mux := http.NewServeMux()
	mux.HandleFunc("/device/code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:      "test-device-code",
			UserCode:        "TEST-CODE",
			VerificationURI: "https://example.com/device",
			ExpiresIn:       300,
			Interval:        0,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(tokenErrorResponse{Error: "authorization_pending"})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithCancel(t.Context())
	// Cancel immediately so the first poll timer select picks up ctx.Done().
	cancel()

	var promptBuf bytes.Buffer
	_, err := NewDeviceCodeInterceptor(
		ctx,
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		ts.Client(),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestNewDeviceCodeInterceptor_PromptOutput(t *testing.T) {
	withInstantPoll(t)
	ts := newDeviceCodeServer(t, 0, 0, "")

	var promptBuf bytes.Buffer
	_, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		ts.Client(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := promptBuf.String()
	if !strings.Contains(output, "https://example.com/device") {
		t.Errorf("prompt missing verification_uri: %s", output)
	}
	if !strings.Contains(output, "TEST-CODE") {
		t.Errorf("prompt missing user_code: %s", output)
	}
}

func TestNewDeviceCodeInterceptor_VerificationURIComplete(t *testing.T) {
	withInstantPoll(t)
	// Custom server that returns verification_uri_complete.
	mux := http.NewServeMux()
	mux.HandleFunc("/device/code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:              "test-device-code",
			UserCode:                "TEST-CODE",
			VerificationURI:         "https://example.com/device",
			VerificationURIComplete: "https://example.com/device?user_code=TEST-CODE",
			ExpiresIn:               300,
			Interval:                0,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{AccessToken: "test-access-token", TokenType: "Bearer"})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	var promptBuf bytes.Buffer
	_, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		ts.Client(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := promptBuf.String()
	if !strings.Contains(output, "https://example.com/device?user_code=TEST-CODE") {
		t.Errorf("prompt missing verification_uri_complete: %s", output)
	}
	if !strings.Contains(output, "Or visit") {
		t.Errorf("prompt missing fallback text: %s", output)
	}
}

func TestDeviceCodeInterceptor_Before(t *testing.T) {
	interceptor := NewDeviceCodeInterceptorFromToken("my-token")

	req := &a2aclient.Request{
		ServiceParams: make(a2aclient.ServiceParams),
	}
	_, _, err := interceptor.Before(t.Context(), req)
	if err != nil {
		t.Fatalf("Before() error: %v", err)
	}

	got := req.ServiceParams.Get("authorization")
	want := "Bearer my-token"
	if len(got) != 1 || got[0] != want {
		t.Errorf("authorization header: got %v, want [%q]", got, want)
	}
}

func TestDeviceCodeInterceptor_GetToken(t *testing.T) {
	interceptor := NewDeviceCodeInterceptorFromToken("test-token")

	token, err := interceptor.GetToken()
	if err != nil {
		t.Fatalf("GetToken() error: %v", err)
	}
	if token != "test-token" {
		t.Errorf("token: got %q, want %q", token, "test-token")
	}
}

func TestNewDeviceCodeInterceptor_MissingConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     DeviceCodeConfig
		wantErr error
	}{
		{
			name:    "missing client ID",
			cfg:     DeviceCodeConfig{DeviceAuthorizationURL: "https://x/device", TokenURL: "https://x/token"},
			wantErr: ErrMissingClientID,
		},
		{
			name:    "missing device auth URL",
			cfg:     DeviceCodeConfig{ClientID: "id", TokenURL: "https://x/token"},
			wantErr: ErrMissingDeviceAuthURL,
		},
		{
			name:    "missing token URL",
			cfg:     DeviceCodeConfig{ClientID: "id", DeviceAuthorizationURL: "https://x/device"},
			wantErr: ErrMissingTokenURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := NewDeviceCodeInterceptor(t.Context(), tt.cfg, &buf, nil)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewDeviceCodeInterceptor_NilPromptOutput(t *testing.T) {
	withInstantPoll(t)
	ts := newDeviceCodeServer(t, 0, 0, "")

	_, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		nil,
		ts.Client(),
	)
	if !errors.Is(err, ErrMissingPromptOutput) {
		t.Fatalf("expected ErrMissingPromptOutput, got: %v", err)
	}
}

func TestNewDeviceCodeInterceptor_Before_PreservesContext(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(t.Context(), ctxKey{}, "preserved")
	interceptor := NewDeviceCodeInterceptorFromToken("tok")

	req := &a2aclient.Request{ServiceParams: make(a2aclient.ServiceParams)}
	outCtx, _, err := interceptor.Before(ctx, req)
	if err != nil {
		t.Fatalf("Before() error: %v", err)
	}
	if outCtx.Value(ctxKey{}) != "preserved" {
		t.Error("context value not preserved")
	}
}

func TestNewDeviceCodeInterceptor_Before_AppendSemantics(t *testing.T) {
	interceptor := NewDeviceCodeInterceptorFromToken("tok")
	req := &a2aclient.Request{ServiceParams: make(a2aclient.ServiceParams)}

	// Pre-set an authorization value.
	req.ServiceParams.Append("authorization", "existing")

	_, _, err := interceptor.Before(t.Context(), req)
	if err != nil {
		t.Fatalf("Before() error: %v", err)
	}

	got := req.ServiceParams.Get("authorization")
	if len(got) != 2 {
		t.Fatalf("expected 2 authorization values, got %d: %v", len(got), got)
	}
	if got[0] != "existing" {
		t.Errorf("first value: got %q, want %q", got[0], "existing")
	}
	if got[1] != "Bearer tok" {
		t.Errorf("second value: got %q, want %q", got[1], "Bearer tok")
	}
}

func TestRequestDeviceCode_ScopesSent(t *testing.T) {
	var capturedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := fmt.Fprintf(w, "")
		_ = body
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		capturedBody = r.PostForm.Encode()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:      "dc",
			UserCode:        "UC",
			VerificationURI: "https://example.com/device",
			ExpiresIn:       300,
		})
	}))
	t.Cleanup(ts.Close)

	_, err := requestDeviceCode(t.Context(), ts.Client(), DeviceCodeConfig{
		ClientID:               "cid",
		DeviceAuthorizationURL: ts.URL,
		Scopes:                 []string{"openid", "profile"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedBody, "scope=openid+profile") {
		t.Errorf("expected scopes in body, got: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "client_id=cid") {
		t.Errorf("expected client_id in body, got: %s", capturedBody)
	}
}

func TestRequestDeviceCode_MissingFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Missing device_code, user_code, verification_uri.
		json.NewEncoder(w).Encode(DeviceCodeResponse{})
	}))
	t.Cleanup(ts.Close)

	_, err := requestDeviceCode(t.Context(), ts.Client(), DeviceCodeConfig{
		ClientID:               "cid",
		DeviceAuthorizationURL: ts.URL,
	})
	if err == nil {
		t.Fatal("expected error for missing fields, got nil")
	}
	if !strings.Contains(err.Error(), "missing required fields") {
		t.Errorf("expected missing fields error, got: %v", err)
	}
}

func TestPollForToken_NonJSONError(t *testing.T) {
	withInstantPoll(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "<html>error</html>")
	}))
	t.Cleanup(ts.Close)

	_, err := pollForToken(t.Context(), ts.Client(), ts.URL, "cid", "dc", 0, 300)
	if err == nil {
		t.Fatal("expected error for non-JSON response, got nil")
	}
	if !strings.Contains(err.Error(), "non-JSON body") {
		t.Errorf("expected non-JSON error message, got: %v", err)
	}
}

func TestNewDeviceCodeInterceptor_NilHTTPClient(t *testing.T) {
	withInstantPoll(t)
	ts := newDeviceCodeServer(t, 0, 0, "")

	var promptBuf bytes.Buffer
	interceptor, err := NewDeviceCodeInterceptor(
		t.Context(),
		DeviceCodeConfig{
			ClientID:               "test-client",
			DeviceAuthorizationURL: ts.URL + "/device/code",
			TokenURL:               ts.URL + "/token",
		},
		&promptBuf,
		nil, // nil httpClient → uses http.DefaultClient
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	token, _ := interceptor.GetToken()
	if token != "test-access-token" {
		t.Errorf("token: got %q, want %q", token, "test-access-token")
	}
}

func TestRequestToken_UnknownError(t *testing.T) {
	withInstantPoll(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(tokenErrorResponse{Error: "some_unknown_error"})
	}))
	t.Cleanup(ts.Close)

	_, err := pollForToken(t.Context(), ts.Client(), ts.URL, "cid", "dc", 0, 300)
	if err == nil {
		t.Fatal("expected error for unknown error code, got nil")
	}
	if !strings.Contains(err.Error(), "some_unknown_error") {
		t.Errorf("expected error to contain unknown code, got: %v", err)
	}
}

func TestRequestToken_HTTPRequestError(t *testing.T) {
	withInstantPoll(t)
	// Use an invalid URL to trigger HTTP request failure.
	_, err := pollForToken(t.Context(), http.DefaultClient, "http://127.0.0.1:1/token", "cid", "dc", 0, 300)
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
}

func TestRequestToken_EmptyAccessToken(t *testing.T) {
	withInstantPoll(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{TokenType: "Bearer"}) // no access_token
	}))
	t.Cleanup(ts.Close)

	_, err := pollForToken(t.Context(), ts.Client(), ts.URL, "cid", "dc", 0, 300)
	if err == nil {
		t.Fatal("expected error for empty access_token, got nil")
	}
	if !strings.Contains(err.Error(), "missing access_token") {
		t.Errorf("expected missing access_token error, got: %v", err)
	}
}

func TestRequestToken_InvalidSuccessJSON(t *testing.T) {
	withInstantPoll(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "not-json")
	}))
	t.Cleanup(ts.Close)

	_, err := pollForToken(t.Context(), ts.Client(), ts.URL, "cid", "dc", 0, 300)
	if err == nil {
		t.Fatal("expected error for invalid JSON success response, got nil")
	}
}

func TestPollForToken_ExpiredToken(t *testing.T) {
	withInstantPoll(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(tokenErrorResponse{Error: "expired_token"})
	}))
	t.Cleanup(ts.Close)

	_, err := pollForToken(t.Context(), ts.Client(), ts.URL, "cid", "dc", 0, 300)
	if !errors.Is(err, ErrDeviceCodeExpired) {
		t.Fatalf("expected ErrDeviceCodeExpired, got: %v", err)
	}
}
