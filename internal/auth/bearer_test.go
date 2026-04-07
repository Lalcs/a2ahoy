package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

func TestNewBearerTokenInterceptor(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		wantErr    error
		wantToken  string
		wantNilPtr bool
	}{
		{
			name:      "valid token",
			token:     "abc",
			wantToken: "abc",
		},
		{
			name:      "jwt-like token",
			token:     "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig",
			wantToken: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig",
		},
		{
			name:       "empty token returns ErrEmptyBearerToken",
			token:      "",
			wantErr:    ErrEmptyBearerToken,
			wantNilPtr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewBearerTokenInterceptor(tt.token)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected errors.Is(err, %v), got: %v", tt.wantErr, err)
				}
				if tt.wantNilPtr && got != nil {
					t.Errorf("expected nil interceptor on error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("interceptor should not be nil")
			}
			if got.token != tt.wantToken {
				t.Errorf("got token %q, want %q", got.token, tt.wantToken)
			}
		})
	}
}

func TestBearerTokenInterceptor_Before(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		wantAuth string
	}{
		{
			name:     "simple ascii token",
			token:    "abc",
			wantAuth: "Bearer abc",
		},
		{
			name:     "jwt-like token",
			token:    "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig",
			wantAuth: "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig",
		},
		{
			name:     "token with special chars",
			token:    "token-with-+/=",
			wantAuth: "Bearer token-with-+/=",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor, err := NewBearerTokenInterceptor(tt.token)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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
			if authValues[0] != tt.wantAuth {
				t.Errorf("got %q, want %q", authValues[0], tt.wantAuth)
			}
		})
	}
}

func TestBearerTokenInterceptor_Before_PreservesContext(t *testing.T) {
	interceptor, err := NewBearerTokenInterceptor("any")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type ctxKey string
	const myKey ctxKey = "marker"
	ctxIn := context.WithValue(context.Background(), myKey, "value")

	req := &a2aclient.Request{
		ServiceParams: make(a2aclient.ServiceParams),
	}

	ctxOut, _, err := interceptor.Before(ctxIn, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := ctxOut.Value(myKey); got != "value" {
		t.Errorf("context value lost: got %v, want %q", got, "value")
	}
}

func TestBearerTokenInterceptor_Before_AppendsNotReplaces(t *testing.T) {
	// Composition with other interceptors requires append semantics.
	interceptor, err := NewBearerTokenInterceptor("new-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &a2aclient.Request{
		ServiceParams: make(a2aclient.ServiceParams),
	}
	req.ServiceParams.Append("authorization", "Bearer existing-token")

	if _, _, err := interceptor.Before(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	authValues := req.ServiceParams.Get("authorization")
	if len(authValues) != 2 {
		t.Fatalf("expected 2 auth values after append, got %d: %v", len(authValues), authValues)
	}
	foundExisting := false
	foundNew := false
	for _, v := range authValues {
		if v == "Bearer existing-token" {
			foundExisting = true
		}
		if v == "Bearer new-token" {
			foundNew = true
		}
	}
	if !foundExisting {
		t.Errorf("existing auth value was replaced: %v", authValues)
	}
	if !foundNew {
		t.Errorf("new auth value was not appended: %v", authValues)
	}
}
