package auth

import (
	"context"
	"fmt"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

// GCPAuthInterceptor injects a GCP ID token (via ADC) as a Bearer token
// into every outgoing A2A request.
type GCPAuthInterceptor struct {
	a2aclient.PassthroughInterceptor
	tokenSource oauth2.TokenSource
}

// NewGCPAuthInterceptor creates a new interceptor that obtains ID tokens
// using Application Default Credentials for the given audience (agent URL).
func NewGCPAuthInterceptor(ctx context.Context, audience string) (*GCPAuthInterceptor, error) {
	ts, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP ID token source: %w", err)
	}
	return &GCPAuthInterceptor{tokenSource: ts}, nil
}

// NewGCPAuthInterceptorFromSource creates a GCPAuthInterceptor from a
// pre-existing oauth2.TokenSource. This is useful for testing and for
// callers that obtain their token source through non-ADC mechanisms.
func NewGCPAuthInterceptorFromSource(ts oauth2.TokenSource) *GCPAuthInterceptor {
	return &GCPAuthInterceptor{tokenSource: ts}
}

// GetToken returns a fresh access token string for use outside the interceptor
// (e.g., for adding auth headers to agent card resolution requests).
func (g *GCPAuthInterceptor) GetToken() (string, error) {
	token, err := g.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("failed to obtain GCP ID token: %w", err)
	}
	return token.AccessToken, nil
}

// Before injects the Authorization header with the ID token.
func (g *GCPAuthInterceptor) Before(ctx context.Context, req *a2aclient.Request) (context.Context, any, error) {
	token, err := g.tokenSource.Token()
	if err != nil {
		return ctx, nil, fmt.Errorf("failed to obtain GCP ID token: %w", err)
	}
	req.ServiceParams.Append("authorization", "Bearer "+token.AccessToken)
	return ctx, nil, nil
}
