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
	// V03RESTMount, when true, rewrites HTTP+JSON v0.3 interface URLs to
	// the "/v1" mount-point convention used by the Python a2a-sdk REST
	// client, Google ADK, and Vertex AI Agent Engine's non-Vertex route.
	// Disabled by default so native a2a-go v2 spec-compliant servers are
	// addressed as-is. Applies to both standard and Vertex AI code paths.
	// See applyV03RESTMountPrefix for the full rationale.
	V03RESTMount bool
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
		// vertexai.Client stores the card by reference and derives each
		// request URL from it on demand, so the in-place rewrite below
		// is automatically picked up by subsequent SendMessage/GetTask/
		// CancelTask calls. Done here (not inside resolveVertexAICard)
		// so ResolveCard — which shares that helper for the `card`
		// subcommand — continues to surface the raw URLs.
		if opts.V03RESTMount {
			applyV03RESTMountPrefix(card)
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

// v03RESTMountSuffix is the path segment Python a2a-sdk / Google ADK /
// Vertex AI Agent Engine non-Vertex routes mount v0.3 HTTP+JSON endpoints
// under. applyV03RESTMountPrefix appends this to advertised URLs when
// the caller opts in via Options.V03RESTMount.
const v03RESTMountSuffix = "/v1"

// applyV03RESTMountPrefix appends v03RESTMountSuffix to every HTTP+JSON
// transport interface in the card that advertises A2A v0.3, so REST calls
// resolve under the mount point convention used by the Python v0.3 ecosystem.
//
// Background: A2A v0.3 has an interpretation split around
// AgentInterface.url. The v0.3 spec example (and the Python a2a-sdk
// type definition mirroring it) reads "https://api.example.com/a2a/v1"
// — URL-as-mountpoint. But the Python a2a-sdk REST client
// implementation (client/transports/rest.py) treats URL as a bare base
// and hardcodes "/v1" on top. As a result, canonical v0.3 peers (the
// Python a2a-sdk reference server, Google ADK's to_a2a(), and Vertex AI
// Agent Engine's non-Vertex route) publish cards whose HTTP+JSON URL
// lacks "/v1" while their routes actually mount under "/v1/*".
//
// a2a-go v2 follows the v0.3 spec example literally and joins
// "/message:send" directly onto iface.URL, which 404s against the
// Python side. This function patches the card client-side to bridge
// the gap, so send/stream/get/cancel succeed against v0.3 peers.
// It is NOT a workaround for a bug in a2a-go — a2a-go is faithful to
// the v0.3 spec example. The A2A v1.0 spec later removed "/v1" from
// HTTP bindings, which can be read as the spec authors resolving this
// interpretation split on the URL-as-bare-base side.
//
// Properties:
//   - idempotent — URLs already ending in "/v1" are skipped;
//   - scoped — only v0.3 HTTP+JSON interfaces are touched; JSON-RPC and
//     v1.0 interfaces are left untouched;
//   - display-safe — ResolveCard (used by the `card` subcommand)
//     intentionally skips this rewrite so users see the raw URLs the
//     server advertised.
//
// See also: internal/cardcheck.checkV03HTTPJSONMissingV1 is the display-
// side counterpart that reports the same condition without mutating the
// card. The predicate (HTTP+JSON && protocol version starts with "0.3"
// && URL does not end in "/v1") must stay in sync between the two
// functions; mirrored test matrices in both packages catch drift in CI.
func applyV03RESTMountPrefix(card *a2a.AgentCard) {
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
		if strings.HasSuffix(trimmed, v03RESTMountSuffix) {
			continue
		}
		iface.URL = trimmed + v03RESTMountSuffix
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
// and then building a client from it. The v0.3 REST mount-point rewrite
// is applied here (and not inside resolveStandardCard) so ResolveCard,
// which shares that helper, can keep surfacing raw URLs for the `card`
// subcommand while send/stream/get/cancel opt into the compatibility
// rewrite via Options.V03RESTMount.
func newStandard(ctx context.Context, opts Options) (A2AClient, *a2a.AgentCard, error) {
	card, clientOpts, err := resolveStandardCard(ctx, opts)
	if err != nil {
		return nil, nil, err
	}

	// See applyV03RESTMountPrefix for the rationale.
	if opts.V03RESTMount {
		applyV03RESTMountPrefix(card)
	}

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
// Shared by newStandard (which also calls applyV03RESTMountPrefix and
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
// and the applyV03RESTMountPrefix URL rewrite applied by newStandard.
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
	// a long-lived A2A client. applyV03RESTMountPrefix is also
	// intentionally skipped so that the `card` subcommand displays raw
	// URLs from the server rather than the compatibility-rewritten URLs.
	card, _, err := resolveStandardCard(ctx, opts)
	if err != nil {
		return nil, err
	}
	return card, nil
}
