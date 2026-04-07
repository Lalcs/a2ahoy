package auth

import (
	"context"
	"errors"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// ErrEmptyBearerToken is returned by NewBearerTokenInterceptor when the token is empty.
var ErrEmptyBearerToken = errors.New("bearer token is empty")

// BearerTokenInterceptor injects a static Bearer token into every outgoing A2A request.
type BearerTokenInterceptor struct {
	a2aclient.PassthroughInterceptor
	token string
}

// NewBearerTokenInterceptor creates a new interceptor. An empty token returns ErrEmptyBearerToken.
func NewBearerTokenInterceptor(token string) (*BearerTokenInterceptor, error) {
	if token == "" {
		return nil, ErrEmptyBearerToken
	}
	return &BearerTokenInterceptor{token: token}, nil
}

// Before injects the Authorization header with the configured token.
func (b *BearerTokenInterceptor) Before(ctx context.Context, req *a2aclient.Request) (context.Context, any, error) {
	req.ServiceParams.Append("authorization", "Bearer "+b.token)
	return ctx, nil, nil
}
