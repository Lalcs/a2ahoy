package client

import (
	"context"
	"fmt"
	"strings"

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
		// resolveVertexAICard returns a concrete *vertexai.Client, which
		// satisfies A2AClient. Tuple return types must match exactly, so
		// we destructure and re-return rather than forwarding directly.
		vc, card, err := resolveVertexAICard(ctx, opts)
		if err != nil {
			return nil, nil, err
		}
		return vc, card, nil
	}
	return newStandard(ctx, opts)
}

// resolveVertexAICard parses the Vertex AI endpoint, configures the GCP
// OAuth2 access-token interceptor, applies --header entries, and fetches
// the agent card.
//
// Shared by New (uses the returned client) and ResolveCard (discards
// it — vertexai.Client.Destroy() is a no-op, see internal/vertexai/client.go).
// applyVertexAIHeaders is called before FetchCard so that --header values are
// applied to the card-fetch request itself.
func resolveVertexAICard(ctx context.Context, opts Options) (*vertexai.Client, *a2a.AgentCard, error) {
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

// applyV1PathPrefix appends "/v1" to the URL of every HTTP+JSON transport
// interface in the card that advertises A2A v0.3.
//
// Rationale: a2a-go v2 (as of v2.1.0) omits the "/v1" prefix from REST
// paths like "/message:send" (see internal/rest/rest.go in that module),
// but the A2A v0.3 spec and its reference implementation, Python a2a-sdk,
// register REST routes under "/v1/*". Without this fix, a2ahoy commands
// (send/stream/get/cancel) against Python a2a-sdk 0.3.x servers fail with
// 404. This is a client-side workaround for the upstream bug.
//
// The fix is:
//   - idempotent — skipped when the URL already ends in "/v1";
//   - scoped — applies only to v0.3 HTTP+JSON interfaces, leaving
//     JSON-RPC and v1.0 interfaces untouched;
//   - safe for display — no caller of New currently displays
//     SupportedInterfaces[].URL from the card it receives; the `card`
//     subcommand uses ResolveCard, which does not invoke this function.
//
// See also: internal/cardcheck.checkV03HTTPJSONMissingV1 is the display-
// side counterpart that reports this condition as a warning without
// mutating the card. The predicate (HTTP+JSON && protocol version starts
// with "0.3" && URL does not end in "/v1") must stay in sync between the
// two functions; tests in both packages enumerate the same matrix so
// drift is caught in CI.
func applyV1PathPrefix(card *a2a.AgentCard) {
	if card == nil {
		return
	}
	for _, iface := range card.SupportedInterfaces {
		if iface == nil {
			continue
		}
		if iface.ProtocolBinding != a2a.TransportProtocolHTTPJSON {
			continue
		}
		// Match any 0.3.x version — the constant a2av0.Version is "0.3",
		// but cards may carry "0.3.0", "0.3.1", etc.
		if !strings.HasPrefix(string(iface.ProtocolVersion), "0.3") {
			continue
		}
		trimmed := strings.TrimRight(iface.URL, "/")
		if strings.HasSuffix(trimmed, "/v1") {
			continue
		}
		iface.URL = trimmed + "/v1"
	}
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

// newStandard creates a standard A2A client by resolving the agent card
// and then building a client from it.
//
// applyV1PathPrefix is invoked here (not inside resolveStandardCard) so
// that ResolveCard, which shares resolveStandardCard, does not rewrite
// the card URLs — preserving the distinction that the `card` subcommand
// displays raw URLs while send/stream/get/cancel use the workaround.
func newStandard(ctx context.Context, opts Options) (A2AClient, *a2a.AgentCard, error) {
	card, clientOpts, err := resolveStandardCard(ctx, opts)
	if err != nil {
		return nil, nil, err
	}

	// Workaround for upstream a2a-go bug: the v0.3 REST compat transport
	// omits the "/v1" prefix from paths like /message:send, but the A2A
	// v0.3 spec (and Python a2a-sdk, its reference implementation) serves
	// routes under /v1/*. Without this, `send`/`stream`/`get`/`cancel`
	// against Python a2a-sdk servers returns 404. See applyV1PathPrefix.
	applyV1PathPrefix(card)

	client, err := a2aclient.NewFromCard(ctx, card, clientOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create A2A client: %w", err)
	}
	return client, card, nil
}

// resolveStandardCard assembles auth resolveOpts and clientOpts from the
// caller's Options, runs the v0-compat agent-card resolver, and returns
// both the parsed card and the clientOpts slice.
//
// Shared by newStandard (which also calls applyV1PathPrefix and
// a2aclient.NewFromCard) and ResolveCard (which discards clientOpts
// because it never constructs a long-lived client).
//
// The client is configured to support both A2A spec v1.0 and v0.3 servers:
//   - The agent card is parsed with the v0 compat parser, which handles both
//     formats as a union type (verified by upstream test TestAgentCard_NewToNew).
//   - In addition to the default v1.0 transports, v0.3 JSON-RPC and REST
//     transports are registered via WithCompatTransport so that send/stream
//     commands work against both v1.0 and v0.3 servers (e.g., Python a2a-sdk
//     0.3.x / Google ADK).
func resolveStandardCard(ctx context.Context, opts Options) (*a2a.AgentCard, []a2aclient.FactoryOption, error) {
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
	// carries them. The interceptor is only registered when there is at
	// least one header entry, so ResolveCard's one-shot path does not
	// allocate an empty interceptor.
	headerEntries, err := parseOptionHeaders(opts.Headers)
	if err != nil {
		return nil, nil, err
	}
	resolveOpts = appendHeaderResolveOpts(resolveOpts, headerEntries)
	if len(headerEntries) > 0 {
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

	return card, clientOpts, nil
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
//
// Card-resolution logic is shared with New via resolveVertexAICard and
// resolveStandardCard; ResolveCard deliberately skips client construction
// and the applyV1PathPrefix URL rewrite applied by newStandard.
func ResolveCard(ctx context.Context, opts Options) (*a2a.AgentCard, error) {
	if opts.VertexAI {
		// resolveVertexAICard is shared with newVertexAI; the returned
		// *vertexai.Client is discarded here because Destroy() is a no-op.
		_, card, err := resolveVertexAICard(ctx, opts)
		if err != nil {
			return nil, err
		}
		return card, nil
	}

	// resolveStandardCard is shared with newStandard; the returned
	// clientOpts are discarded here because ResolveCard never constructs
	// a long-lived A2A client. applyV1PathPrefix is also intentionally
	// skipped so that the `card` subcommand displays raw URLs from the
	// server rather than the workaround-rewritten URLs.
	card, _, err := resolveStandardCard(ctx, opts)
	if err != nil {
		return nil, err
	}
	return card, nil
}
