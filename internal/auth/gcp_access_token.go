package auth

import (
	"context"
	"fmt"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GCPAccessTokenInterceptor injects a GCP OAuth2 access token (via ADC)
// as a Bearer token into every outgoing request. Unlike GCPAuthInterceptor
// which uses ID tokens, this uses access tokens with the cloud-platform
// scope — required by Vertex AI Agent Engine.
type GCPAccessTokenInterceptor struct {
	a2aclient.PassthroughInterceptor
	tokenSource oauth2.TokenSource
}

// NewGCPAccessTokenInterceptor creates a new interceptor that obtains
// OAuth2 access tokens using Application Default Credentials with the
// cloud-platform scope.
func NewGCPAccessTokenInterceptor(ctx context.Context) (*GCPAccessTokenInterceptor, error) {
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("failed to find default credentials: %w", err)
	}
	return &GCPAccessTokenInterceptor{tokenSource: creds.TokenSource}, nil
}

// GetToken returns a fresh OAuth2 access token string for use outside the
// interceptor (e.g., for adding auth headers to agent card resolution requests).
func (g *GCPAccessTokenInterceptor) GetToken() (string, error) {
	token, err := g.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("failed to obtain GCP access token: %w", err)
	}
	return token.AccessToken, nil
}

// Before injects the Authorization header with the OAuth2 access token.
func (g *GCPAccessTokenInterceptor) Before(ctx context.Context, req *a2aclient.Request) (context.Context, any, error) {
	token, err := g.tokenSource.Token()
	if err != nil {
		return ctx, nil, fmt.Errorf("failed to obtain GCP access token: %w", err)
	}
	req.ServiceParams.Append("authorization", "Bearer "+token.AccessToken)
	return ctx, nil, nil
}
