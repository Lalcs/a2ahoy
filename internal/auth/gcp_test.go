package auth

import (
	"context"
	"errors"
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
