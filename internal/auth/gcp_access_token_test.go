package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"golang.org/x/oauth2"
)

var errAccessTokenFailure = errors.New("access token failure")

func TestGCPAccessTokenInterceptor_GetToken(t *testing.T) {
	mock := &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: "test-access-token-123",
			Expiry:      time.Now().Add(time.Hour),
		},
	}

	interceptor := &GCPAccessTokenInterceptor{tokenSource: mock}

	got, err := interceptor.GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "test-access-token-123" {
		t.Errorf("token mismatch: got %q, want %q", got, "test-access-token-123")
	}
}

func TestGCPAccessTokenInterceptor_GetToken_Error(t *testing.T) {
	mock := &mockTokenSource{err: errAccessTokenFailure}

	interceptor := &GCPAccessTokenInterceptor{tokenSource: mock}

	_, err := interceptor.GetToken()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGCPAccessTokenInterceptor_Before(t *testing.T) {
	mock := &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: "bearer-access-token",
			Expiry:      time.Now().Add(time.Hour),
		},
	}

	interceptor := &GCPAccessTokenInterceptor{tokenSource: mock}

	req := &a2aclient.Request{
		ServiceParams: a2aclient.ServiceParams{},
	}

	_, _, err := interceptor.Before(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vals := req.ServiceParams.Get("authorization")
	if len(vals) == 0 {
		t.Fatal("authorization header not set")
	}
	want := "Bearer bearer-access-token"
	if vals[0] != want {
		t.Errorf("authorization header: got %q, want %q", vals[0], want)
	}
}

func TestGCPAccessTokenInterceptor_Before_Error(t *testing.T) {
	mock := &mockTokenSource{err: errAccessTokenFailure}

	interceptor := &GCPAccessTokenInterceptor{tokenSource: mock}

	req := &a2aclient.Request{
		ServiceParams: a2aclient.ServiceParams{},
	}

	_, _, err := interceptor.Before(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewGCPAccessTokenInterceptor_Success(t *testing.T) {
	// writeServiceAccountJSON is declared in gcp_test.go (same package).
	credsFile := writeServiceAccountJSON(t, t.TempDir(), "https://oauth2.googleapis.com/token")

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credsFile)

	interceptor, err := NewGCPAccessTokenInterceptor(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
}

func TestNewGCPAccessTokenInterceptorFromSource(t *testing.T) {
	ts := &mockTokenSource{token: &oauth2.Token{AccessToken: "from-source-access"}}
	interceptor := NewGCPAccessTokenInterceptorFromSource(ts)
	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
	got, err := interceptor.GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-source-access" {
		t.Errorf("got %q, want %q", got, "from-source-access")
	}
}

func TestNewGCPAccessTokenInterceptor_Error(t *testing.T) {
	// Create a temporary file with invalid JSON so google.FindDefaultCredentials fails.
	tmpDir := t.TempDir()
	badCreds := filepath.Join(tmpDir, "bad_creds.json")
	if err := os.WriteFile(badCreds, []byte("not valid json"), 0o600); err != nil {
		t.Fatalf("failed to create temp credentials file: %v", err)
	}

	// Point GCP credential discovery at the invalid file and clear
	// environment variables that could supply alternative credentials.
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", badCreds)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GCLOUD_PROJECT", "")
	t.Setenv("CLOUDSDK_CORE_PROJECT", "")

	_, err := NewGCPAccessTokenInterceptor(context.Background())
	if err == nil {
		t.Fatal("expected error when credentials are invalid, got nil")
	}
}
