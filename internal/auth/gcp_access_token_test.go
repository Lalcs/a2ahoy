package auth

import (
	"context"
	"errors"
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
