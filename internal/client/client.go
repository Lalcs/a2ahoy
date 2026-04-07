package client

import (
	"context"
	"fmt"

	"github.com/Lalcs/a2ahoy/internal/auth"
	"github.com/Lalcs/a2ahoy/internal/vertexai"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"github.com/a2aproject/a2a-go/v2/a2acompat/a2av0"
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
//
// The client is configured to support both A2A spec v1.0 and v0.3 servers:
//   - The agent card is parsed with the v0 compat parser, which handles both
//     formats as a union type (verified by upstream test TestAgentCard_NewToNew).
//   - In addition to the default v1.0 transports, v0.3 JSON-RPC and REST
//     transports are registered via WithCompatTransport so that send/stream
//     commands work against both v1.0 and v0.3 servers (e.g., Python a2a-sdk
//     0.3.x / Google ADK).
func newStandard(ctx context.Context, opts Options) (A2AClient, *a2a.AgentCard, error) {
	var resolveOpts []agentcard.ResolveOption
	var clientOpts []a2aclient.FactoryOption

	if opts.GCPAuth {
		interceptor, err := auth.NewGCPAuthInterceptor(ctx, opts.BaseURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create GCP auth: %w", err)
		}

		token, err := interceptor.GetToken()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to obtain initial token: %w", err)
		}
		resolveOpts = append(resolveOpts, agentcard.WithRequestHeader("Authorization", "Bearer "+token))
		clientOpts = append(clientOpts, a2aclient.WithCallInterceptors(interceptor))
	}

	// Register v0.3 compat transports in addition to the auto-registered
	// v1.0 transports. selectTransport prefers newer protocol versions, so
	// v1.0 servers continue to use v1.0 transports without regression.
	clientOpts = append(clientOpts,
		a2aclient.WithCompatTransport(
			a2av0.Version,
			a2a.TransportProtocolJSONRPC,
			a2av0.NewJSONRPCTransportFactory(a2av0.JSONRPCTransportConfig{}),
		),
		a2aclient.WithCompatTransport(
			a2av0.Version,
			a2a.TransportProtocolHTTPJSON,
			a2av0.NewRESTTransportFactory(a2av0.RESTTransportConfig{}),
		),
	)

	// Use a Resolver with the v0 compat card parser. The parser handles
	// both v0.3 and v1.0 card formats via a union struct.
	resolver := &agentcard.Resolver{
		Client:     agentcard.DefaultResolver.Client,
		CardParser: a2av0.NewAgentCardParser(),
	}
	card, err := resolver.Resolve(ctx, opts.BaseURL, resolveOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve agent card: %w", err)
	}

	client, err := a2aclient.NewFromCard(ctx, card, clientOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create A2A client: %w", err)
	}
	return client, card, nil
}

// ResolveCard fetches and parses an agent card without creating an A2A client.
//
// This is useful for commands that only need to read the card (e.g., the
// `card` subcommand) and do not need to send or stream messages. By skipping
// client creation, ResolveCard avoids the "agent card has no supported
// interfaces" error that occurs when v0.3 servers are accessed via the
// default v1.0-only client constructor.
//
// Both v1.0 and v0.3 card formats are supported via the v0 compat parser.
func ResolveCard(ctx context.Context, opts Options) (*a2a.AgentCard, error) {
	if opts.VertexAI {
		endpoint, err := vertexai.ParseEndpoint(opts.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid Vertex AI endpoint: %w", err)
		}

		interceptor, err := auth.NewGCPAccessTokenInterceptor(ctx)
		if err != nil {
			return nil, fmt.Errorf("GCP access token auth setup failed: %w", err)
		}

		vc := vertexai.NewClient(endpoint, interceptor.GetToken)
		card, err := vc.FetchCard(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Vertex AI agent card: %w", err)
		}
		return card, nil
	}

	var resolveOpts []agentcard.ResolveOption
	if opts.GCPAuth {
		interceptor, err := auth.NewGCPAuthInterceptor(ctx, opts.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create GCP auth: %w", err)
		}

		token, err := interceptor.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to obtain initial token: %w", err)
		}
		resolveOpts = append(resolveOpts, agentcard.WithRequestHeader("Authorization", "Bearer "+token))
	}

	resolver := &agentcard.Resolver{
		Client:     agentcard.DefaultResolver.Client,
		CardParser: a2av0.NewAgentCardParser(),
	}
	card, err := resolver.Resolve(ctx, opts.BaseURL, resolveOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent card: %w", err)
	}
	return card, nil
}
