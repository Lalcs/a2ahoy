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
	// Headers holds raw "KEY=VALUE" strings from the --header flag;
	// parsed inside New and ResolveCard via auth.ParseHeaders.
	Headers []string
	// BearerToken is a static bearer token from --bearer-token or
	// A2A_BEARER_TOKEN. Mutually exclusive with GCPAuth and VertexAI.
	BearerToken string
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

	if err := applyVertexAIHeaders(vc, opts.Headers); err != nil {
		return nil, nil, err
	}

	card, err := vc.FetchCard(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch Vertex AI agent card: %w", err)
	}

	return vc, card, nil
}

// parseOptionHeaders parses Options.Headers and wraps parse errors with the
// "invalid --header: ..." prefix used across all client constructors.
func parseOptionHeaders(headers []string) ([]auth.HeaderEntry, error) {
	entries, err := auth.ParseHeaders(headers)
	if err != nil {
		return nil, fmt.Errorf("invalid --header: %w", err)
	}
	return entries, nil
}

// appendHeaderResolveOpts expands parsed headers into agentcard.ResolveOption
// entries appended to the given slice, preserving input order.
func appendHeaderResolveOpts(resolveOpts []agentcard.ResolveOption, entries []auth.HeaderEntry) []agentcard.ResolveOption {
	for _, e := range entries {
		resolveOpts = append(resolveOpts, agentcard.WithRequestHeader(e.Key, e.Value))
	}
	return resolveOpts
}

// appendBearerResolveOpts appends an "Authorization: Bearer <token>" header resolve option when token is non-empty.
func appendBearerResolveOpts(resolveOpts []agentcard.ResolveOption, token string) []agentcard.ResolveOption {
	if token == "" {
		return resolveOpts
	}
	return append(resolveOpts, agentcard.WithRequestHeader("Authorization", "Bearer "+token))
}

// applyVertexAIHeaders parses --header entries and applies them to the
// Vertex AI client. No-op when headers is empty.
func applyVertexAIHeaders(vc *vertexai.Client, headers []string) error {
	entries, err := parseOptionHeaders(headers)
	if err != nil || len(entries) == 0 {
		return err
	}
	veEntries := make([]vertexai.HeaderEntry, len(entries))
	for i, e := range entries {
		veEntries[i] = vertexai.HeaderEntry{Key: e.Key, Value: e.Value}
	}
	vc.SetExtraHeaders(veEntries)
	return nil
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

	// BearerToken takes precedence over GCPAuth so library callers get
	// deterministic behavior even if both fields are set; the CLI layer
	// additionally enforces mutual exclusion.
	if opts.BearerToken != "" {
		interceptor, err := auth.NewBearerTokenInterceptor(opts.BearerToken)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create bearer token auth: %w", err)
		}
		resolveOpts = appendBearerResolveOpts(resolveOpts, opts.BearerToken)
		clientOpts = append(clientOpts, a2aclient.WithCallInterceptors(interceptor))
	} else if opts.GCPAuth {
		interceptor, err := auth.NewGCPAuthInterceptor(ctx, opts.BaseURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create GCP auth: %w", err)
		}

		token, err := interceptor.GetToken()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to obtain initial token: %w", err)
		}
		resolveOpts = appendBearerResolveOpts(resolveOpts, token)
		clientOpts = append(clientOpts, a2aclient.WithCallInterceptors(interceptor))
	}

	// Inject user-supplied headers (--header flag) into both the card
	// resolver and the call interceptor chain so every outgoing request
	// carries them.
	headerEntries, err := parseOptionHeaders(opts.Headers)
	if err != nil {
		return nil, nil, err
	}
	if len(headerEntries) > 0 {
		resolveOpts = appendHeaderResolveOpts(resolveOpts, headerEntries)
		clientOpts = append(clientOpts, a2aclient.WithCallInterceptors(auth.NewHeaderInterceptor(headerEntries)))
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
		if err := applyVertexAIHeaders(vc, opts.Headers); err != nil {
			return nil, err
		}
		card, err := vc.FetchCard(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Vertex AI agent card: %w", err)
		}
		return card, nil
	}

	var resolveOpts []agentcard.ResolveOption
	if opts.BearerToken != "" {
		resolveOpts = appendBearerResolveOpts(resolveOpts, opts.BearerToken)
	} else if opts.GCPAuth {
		interceptor, err := auth.NewGCPAuthInterceptor(ctx, opts.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create GCP auth: %w", err)
		}

		token, err := interceptor.GetToken()
		if err != nil {
			return nil, fmt.Errorf("failed to obtain initial token: %w", err)
		}
		resolveOpts = appendBearerResolveOpts(resolveOpts, token)
	}

	// Inject user-supplied headers (--header flag) into the card
	// resolution request. No call interceptor is needed here because
	// ResolveCard does not construct a long-lived A2A client.
	headerEntries, err := parseOptionHeaders(opts.Headers)
	if err != nil {
		return nil, err
	}
	resolveOpts = appendHeaderResolveOpts(resolveOpts, headerEntries)

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
