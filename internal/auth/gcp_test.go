package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"golang.org/x/oauth2"
)

type mockTokenSource struct {
	token *oauth2.Token
	err   error
}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	return m.token, m.err
}

func TestGetToken_Success(t *testing.T) {
	interceptor := &GCPAuthInterceptor{
		tokenSource: &mockTokenSource{
			token: &oauth2.Token{AccessToken: "test-id-token"},
		},
	}

	got, err := interceptor.GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "test-id-token" {
		t.Errorf("got %q, want %q", got, "test-id-token")
	}
}

var errTokenSourceFailure = errors.New("token source failure")

func TestGetToken_Error(t *testing.T) {
	interceptor := &GCPAuthInterceptor{
		tokenSource: &mockTokenSource{
			err: errTokenSourceFailure,
		},
	}

	_, err := interceptor.GetToken()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errTokenSourceFailure) {
		t.Errorf("expected wrapped error containing %v, got: %v", errTokenSourceFailure, err)
	}
}

func TestBefore_Success(t *testing.T) {
	interceptor := &GCPAuthInterceptor{
		tokenSource: &mockTokenSource{
			token: &oauth2.Token{AccessToken: "bearer-token"},
		},
	}

	req := &a2aclient.Request{
		ServiceParams: make(a2aclient.ServiceParams),
	}

	ctx, result, err := interceptor.Before(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
	if result != nil {
		t.Errorf("result should be nil, got %v", result)
	}

	authValues := req.ServiceParams.Get("authorization")
	if len(authValues) != 1 {
		t.Fatalf("expected 1 auth value, got %d", len(authValues))
	}
	if authValues[0] != "Bearer bearer-token" {
		t.Errorf("got %q, want %q", authValues[0], "Bearer bearer-token")
	}
}

var errTokenFailure = errors.New("token failure")

func TestBefore_Error(t *testing.T) {
	interceptor := &GCPAuthInterceptor{
		tokenSource: &mockTokenSource{
			err: errTokenFailure,
		},
	}

	req := &a2aclient.Request{
		ServiceParams: make(a2aclient.ServiceParams),
	}

	_, _, err := interceptor.Before(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errTokenFailure) {
		t.Errorf("expected wrapped error containing %v, got: %v", errTokenFailure, err)
	}

	// Authorization header should NOT be set on error
	authValues := req.ServiceParams.Get("authorization")
	if len(authValues) != 0 {
		t.Errorf("expected no auth values on error, got %v", authValues)
	}
}

// fakeJWT returns a structurally valid but unsigned JWT that the
// idtoken library accepts when parsing a token response.
func fakeJWT() string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"test","aud":"https://example.com","exp":9999999999,"iat":1234567890}`))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fakesignature"))
	return strings.Join([]string{header, payload, sig}, ".")
}

// writeServiceAccountJSON generates a temporary service account JSON file
// with a freshly-generated RSA key and the given token URI. It returns the
// file path. The file is placed inside dir.
func writeServiceAccountJSON(t *testing.T, dir, tokenURI string) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	sa := map[string]string{
		"type":                        "service_account",
		"project_id":                  "test-project",
		"private_key_id":              "key-id",
		"private_key":                 string(keyPEM),
		"client_email":                "test@test-project.iam.gserviceaccount.com",
		"client_id":                   "123456789",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   tokenURI,
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url":        "https://www.googleapis.com/robot/v1/metadata/x509/test",
	}
	data, err := json.Marshal(sa)
	if err != nil {
		t.Fatalf("failed to marshal service account JSON: %v", err)
	}

	p := filepath.Join(dir, "sa.json")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("failed to write service account JSON: %v", err)
	}
	return p
}

func TestNewGCPAuthInterceptor_Success(t *testing.T) {
	// Start a mock token server that returns a valid-looking JWT response.
	jwt := fakeJWT()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": jwt,
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     jwt,
		})
	}))
	defer srv.Close()

	credsFile := writeServiceAccountJSON(t, t.TempDir(), srv.URL)

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credsFile)

	interceptor, err := NewGCPAuthInterceptor(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
}

func TestNewGCPAuthInterceptorFromSource(t *testing.T) {
	ts := &mockTokenSource{token: &oauth2.Token{AccessToken: "from-source"}}
	interceptor := NewGCPAuthInterceptorFromSource(ts)
	if interceptor == nil {
		t.Fatal("expected non-nil interceptor")
	}
	got, err := interceptor.GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "from-source" {
		t.Errorf("got %q, want %q", got, "from-source")
	}
}

func TestNewGCPAuthInterceptor_Error(t *testing.T) {
	// Create a temporary file with invalid JSON so idtoken.NewTokenSource fails.
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

	_, err := NewGCPAuthInterceptor(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error when credentials are invalid, got nil")
	}
}
