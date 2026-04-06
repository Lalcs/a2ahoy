package client

import (
	"context"
	"fmt"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"github.com/khayashi/a2ahoy/internal/auth"
	"github.com/khayashi/a2ahoy/internal/vertexai"
)

// Options configures client creation.
type Options struct {
	BaseURL  string
	GCPAuth  bool
	VertexAI bool
}

// New creates an A2A client and resolves the agent card.
// When VertexAI is true, it creates a Vertex AI-specific client with
// OAuth2 access token authentication. Otherwise, it creates a standard
// A2A client optionally with GCP ID token authentication.
func New(ctx context.Context, opts Options) (A2AClient, *a2a.AgentCard, error) {
	if opts.VertexAI {
		return newVertexAI(ctx, opts)
	}
	return newStandard(ctx, opts)
}

// newVertexAI creates a Vertex AI A2A client.
func newVertexAI(ctx context.Context, opts Options) (A2AClient, *a2a.AgentCard, error) {
	endpoint, err := vertexai.ParseEndpoint(opts.BaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid Vertex AI endpoint: %w", err)
	}

	interceptor, err := auth.NewGCPAccessTokenInterceptor(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("GCP access token auth setup failed: %w", err)
	}

	vc := vertexai.NewClient(endpoint, interceptor.GetToken)

	card, err := vc.FetchCard(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch Vertex AI agent card: %w", err)
	}

	return vc, card, nil
}

// newStandard creates a standard A2A client.
func newStandard(ctx context.Context, opts Options) (A2AClient, *a2a.AgentCard, error) {
	resolveOpts := []agentcard.ResolveOption{}

	if opts.GCPAuth {
		interceptor, err := auth.NewGCPAuthInterceptor(ctx, opts.BaseURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create GCP auth: %w", err)
		}

		token, tokenErr := interceptor.GetToken()
		if tokenErr != nil {
			return nil, nil, fmt.Errorf("failed to obtain initial token: %w", tokenErr)
		}
		resolveOpts = append(resolveOpts, agentcard.WithRequestHeader("Authorization", "Bearer "+token))

		card, err := agentcard.DefaultResolver.Resolve(ctx, opts.BaseURL, resolveOpts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve agent card: %w", err)
		}

		client, err := a2aclient.NewFromCard(ctx, card, a2aclient.WithCallInterceptors(interceptor))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create A2A client: %w", err)
		}
		return client, card, nil
	}

	card, err := agentcard.DefaultResolver.Resolve(ctx, opts.BaseURL, resolveOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve agent card: %w", err)
	}

	client, err := a2aclient.NewFromCard(ctx, card)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create A2A client: %w", err)
	}
	return client, card, nil
}
