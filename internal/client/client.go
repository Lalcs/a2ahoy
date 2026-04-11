package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Lalcs/a2ahoy/internal/auth"
	"github.com/Lalcs/a2ahoy/internal/httptrace"
	"github.com/Lalcs/a2ahoy/internal/vertexai"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"github.com/a2aproject/a2a-go/v2/a2acompat/a2av0"
)

// newGCPAuthInterceptorFn, newGCPAccessTokenInterceptorFn, and
// newBearerTokenInterceptorFn are package-level factory functions that can be
// overridden in tests. Production code uses the defaults (auth.NewGCP*,
// auth.NewBearerTokenInterceptor).
var (
	newGCPAuthInterceptorFn        = auth.NewGCPAuthInterceptor
	newGCPAccessTokenInterceptorFn = auth.NewGCPAccessTokenInterceptor
	newBearerTokenInterceptorFn    = auth.NewBearerTokenInterceptor
	newDeviceCodeInterceptorFn     = auth.NewDeviceCodeInterceptor
)

// Options configures client creation.
type Options struct {
	BaseURL  string
	GCPAuth  bool
	VertexAI bool
	// V03RESTMount, when true, rewrites HTTP+JSON v0.3 interface URLs to
	// the "/v1" mount-point convention used by the Python a2a-sdk REST
	// client, Google ADK, and Vertex AI Agent Engine's non-Vertex route.
	// Disabled in the Options zero value so non-CLI callers opt in
	// explicitly; the CLI chooses its own default above this package.
	// Applies to both standard and Vertex AI code paths.
	// See applyV03RESTMountPrefix for the full rationale.
	V03RESTMount bool
	// Headers holds raw "KEY=VALUE" strings from the --header flag;
	// parsed inside New and ResolveCard via auth.ParseHeaders.
	Headers []string
	// BearerToken is a static bearer token from --bearer-token or
	// A2A_BEARER_TOKEN. Mutually exclusive with GCPAuth and VertexAI.
	BearerToken string
	// Timeout overrides the HTTP client timeout for all transports.
	// Zero means use library defaults (30s card resolution, 3min for both standard and Vertex AI).
	Timeout time.Duration
	// Verbose, when true, dumps every HTTP request and response to
	// VerboseOutput via httputil.DumpRequestOut / DumpResponse.
	Verbose bool
	// VerboseOutput is the writer for HTTP trace output when Verbose is
	// true. Callers must set this to a non-nil value when Verbose is true
	// (typically os.Stderr).
	VerboseOutput io.Writer
	// MaxRetries is the maximum number of retries for failed non-streaming requests.
	// Zero means no retry.
	MaxRetries int
	// DeviceAuth enables OAuth2 Device Authorization Grant (RFC 8628).
	// URLs and scopes are auto-detected from the agent card's SecuritySchemes.
	// Only DeviceAuthClientID is required (no card-level equivalent).
	// Mutually exclusive with GCPAuth, VertexAI, and BearerToken.
	DeviceAuth bool
	// DeviceAuthClientID is the OAuth2 client ID for the device code flow.
	// Required when DeviceAuth is true.
	DeviceAuthClientID string
	// DeviceAuthURL optionally overrides the device authorization endpoint URL.
	// When empty, the URL is extracted from the agent card's DeviceCodeOAuthFlow.
	DeviceAuthURL string
	// DeviceAuthTokenURL optionally overrides the token endpoint URL.
	// When empty, the URL is extracted from the agent card's DeviceCodeOAuthFlow.
	DeviceAuthTokenURL string
	// DeviceAuthScopes optionally overrides the OAuth2 scopes.
	// When empty, scopes are extracted from the agent card's DeviceCodeOAuthFlow.
	DeviceAuthScopes []string
	// PromptOutput is the writer for interactive device code prompts
	// (typically os.Stderr). Required when DeviceAuth is true.
	PromptOutput io.Writer
}

// httpClientFromTimeout returns an *http.Client with the given timeout,
// or nil when d is zero (signaling "use library default").
func httpClientFromTimeout(d time.Duration) *http.Client {
	if d == 0 {
		return nil
	}
	return &http.Client{Timeout: d}
}

// buildHTTPClient creates an *http.Client from opts, applying the verbose
// transport wrapper when requested. Returns nil when no custom timeout is
// set and verbose is off (signaling "use library default").
func buildHTTPClient(opts Options) *http.Client {
	hc := httpClientFromTimeout(opts.Timeout)
	if opts.Verbose {
		hc = httptrace.WrapClient(hc, opts.VerboseOutput)
	}
	return hc
}

// New creates an A2A client and resolves the agent card.
// When VertexAI is true, it creates a Vertex AI-specific client with
// OAuth2 access token authentication. Otherwise, it creates a standard
// A2A client optionally with GCP ID token authentication.
func New(ctx context.Context, opts Options) (A2AClient, *a2a.AgentCard, error) {
	var c A2AClient
	var card *a2a.AgentCard
	var err error

	if opts.VertexAI {
		// resolveVertexAICard returns a concrete *vertexai.Client, which
		// satisfies A2AClient. Tuple return types must match exactly, so
		// we destructure and re-return rather than forwarding directly.
		var vc *vertexai.Client
		vc, card, err = resolveVertexAICard(ctx, opts)
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
		c = vc
	} else {
		c, card, err = newStandard(ctx, opts)
		if err != nil {
			return nil, nil, err
		}
	}

	if opts.MaxRetries > 0 {
		c = &retryClient{inner: c, maxRetries: opts.MaxRetries}
	}

	return c, card, nil
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

	interceptor, err := newGCPAccessTokenInterceptorFn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("GCP access token auth setup failed: %w", err)
	}

	vc := vertexai.NewClient(endpoint, interceptor.GetToken, buildHTTPClient(opts))

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
	// deviceAuthCard is set when --device-auth pre-fetches the card to
	// read SecuritySchemes before authentication. When non-nil, the
	// resolver step at the bottom is skipped.
	var deviceAuthCard *a2a.AgentCard

	// Build once; reused by transports, resolver, and device-auth flow.
	hc := buildHTTPClient(opts)

	// BearerToken takes precedence over GCPAuth so library callers get
	// deterministic behavior even if both fields are set; the CLI layer
	// additionally enforces mutual exclusion.
	if opts.BearerToken != "" {
		interceptor, err := newBearerTokenInterceptorFn(opts.BearerToken)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create bearer token auth: %w", err)
		}
		resolveOpts = appendBearerResolveOpts(resolveOpts, opts.BearerToken)
		clientOpts = append(clientOpts, a2aclient.WithCallInterceptors(interceptor))
	} else if opts.GCPAuth {
		interceptor, err := newGCPAuthInterceptorFn(ctx, opts.BaseURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create GCP auth: %w", err)
		}

		token, err := interceptor.GetToken()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to obtain initial token: %w", err)
		}
		resolveOpts = appendBearerResolveOpts(resolveOpts, token)
		clientOpts = append(clientOpts, a2aclient.WithCallInterceptors(interceptor))
	} else if opts.DeviceAuth {
		interceptor, preCard, err := runDeviceCodeAuth(ctx, opts, hc)
		if err != nil {
			return nil, nil, err
		}
		token, err := interceptor.GetToken()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to obtain device auth token: %w", err)
		}
		resolveOpts = appendBearerResolveOpts(resolveOpts, token)
		// The card is already resolved; skip the resolver below.
		deviceAuthCard = preCard
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

	// When a custom timeout or verbose mode is specified, override the
	// default transports with ones using the custom HTTP client. When hc
	// is nil (timeout=0, verbose=off), the library's auto-registered
	// defaults (3min) are used.
	if hc != nil {
		clientOpts = append(clientOpts,
			a2aclient.WithJSONRPCTransport(hc),
			a2aclient.WithRESTTransport(hc),
		)
	}

	// Register v0.3 compat transports in addition to the auto-registered
	// v1.0 transports. selectTransport prefers newer protocol versions, so
	// v1.0 servers continue to use v1.0 transports without regression.
	// When hc is nil the config's zero Client falls back to the library
	// default (3min timeout).
	clientOpts = append(clientOpts,
		a2aclient.WithCompatTransport(
			a2av0.Version,
			a2a.TransportProtocolJSONRPC,
			a2av0.NewJSONRPCTransportFactory(a2av0.JSONRPCTransportConfig{Client: hc}),
		),
		a2aclient.WithCompatTransport(
			a2av0.Version,
			a2a.TransportProtocolHTTPJSON,
			a2av0.NewRESTTransportFactory(a2av0.RESTTransportConfig{Client: hc}),
		),
	)

	// When the card was pre-fetched by the device auth flow, skip the
	// normal resolver and return the pre-fetched card directly.
	if deviceAuthCard != nil {
		return deviceAuthCard, clientOpts, nil
	}

	card, err := buildResolver(hc).Resolve(ctx, opts.BaseURL, resolveOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve agent card: %w", err)
	}

	return card, clientOpts, nil
}

// buildResolver creates an agentcard.Resolver configured with the v0
// compat parser. When hc is non-nil, it overrides the default HTTP client.
func buildResolver(hc *http.Client) *agentcard.Resolver {
	resolverClient := agentcard.DefaultResolver.Client
	if hc != nil {
		resolverClient = hc
	}
	return &agentcard.Resolver{
		Client:     resolverClient,
		CardParser: a2av0.NewAgentCardParser(),
	}
}

// runDeviceCodeAuth pre-fetches the agent card (without auth) to read its
// SecuritySchemes, extracts the DeviceCodeOAuthFlow configuration, runs the
// RFC 8628 flow, and returns the interceptor plus the pre-fetched card.
//
// CLI flag overrides (DeviceAuthURL, DeviceAuthTokenURL, DeviceAuthScopes)
// take precedence over card-derived values.
func runDeviceCodeAuth(ctx context.Context, opts Options, hc *http.Client) (*auth.DeviceCodeInterceptor, *a2a.AgentCard, error) {
	// Pre-fetch card without auth to read SecuritySchemes.
	preCard, err := resolveCardWithoutAuth(ctx, opts, hc)
	if err != nil {
		// If card fetch fails but explicit URLs are provided, continue
		// without the card.
		if opts.DeviceAuthURL == "" || opts.DeviceAuthTokenURL == "" {
			return nil, nil, fmt.Errorf("failed to fetch agent card for device auth auto-detection (provide --device-auth-url and --device-token-url to skip): %w", err)
		}
		preCard = nil
	}

	// Build config from card + overrides.
	cfg := auth.DeviceCodeConfig{
		ClientID: opts.DeviceAuthClientID,
	}

	// Extract from card if available.
	if preCard != nil {
		if cardCfg, findErr := findDeviceCodeFlow(preCard); findErr == nil {
			cfg.DeviceAuthorizationURL = cardCfg.DeviceAuthorizationURL
			cfg.TokenURL = cardCfg.TokenURL
			cfg.Scopes = cardCfg.Scopes
		} else if opts.DeviceAuthURL == "" || opts.DeviceAuthTokenURL == "" {
			return nil, nil, fmt.Errorf("failed to auto-detect device auth flow from agent card: %w", findErr)
		}
	}

	// Apply CLI flag overrides.
	if opts.DeviceAuthURL != "" {
		cfg.DeviceAuthorizationURL = opts.DeviceAuthURL
	}
	if opts.DeviceAuthTokenURL != "" {
		cfg.TokenURL = opts.DeviceAuthTokenURL
	}
	if len(opts.DeviceAuthScopes) > 0 {
		cfg.Scopes = opts.DeviceAuthScopes
	}

	interceptor, err := newDeviceCodeInterceptorFn(ctx, cfg, opts.PromptOutput, hc)
	if err != nil {
		return nil, nil, fmt.Errorf("device code auth failed: %w", err)
	}

	return interceptor, preCard, nil
}

// findDeviceCodeFlow searches the agent card's SecuritySchemes for an
// OAuth2SecurityScheme containing a DeviceCodeOAuthFlow and returns a
// DeviceCodeConfig populated with the flow's URLs and scopes.
func findDeviceCodeFlow(card *a2a.AgentCard) (*auth.DeviceCodeConfig, error) {
	if card == nil || card.SecuritySchemes == nil {
		return nil, fmt.Errorf("agent card has no security schemes")
	}
	seen := make(map[string]struct{})
	var matches []*auth.DeviceCodeConfig
	for _, scheme := range card.SecuritySchemes {
		oauth2Scheme, ok := scheme.(a2a.OAuth2SecurityScheme)
		if !ok {
			continue
		}
		dcFlow, ok := oauth2Scheme.Flows.(a2a.DeviceCodeOAuthFlow)
		if !ok {
			continue
		}
		scopes := make([]string, 0, len(dcFlow.Scopes))
		for scope := range dcFlow.Scopes {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)
		cfg := &auth.DeviceCodeConfig{
			DeviceAuthorizationURL: dcFlow.DeviceAuthorizationURL,
			TokenURL:               dcFlow.TokenURL,
			Scopes:                 scopes,
		}
		key := cfg.DeviceAuthorizationURL + "\x00" + cfg.TokenURL + "\x00" + strings.Join(cfg.Scopes, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		matches = append(matches, cfg)
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no device code OAuth2 flow found in agent card security schemes")
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("multiple device code OAuth2 flows found in agent card security schemes; provide --device-auth-url and --device-token-url to select one explicitly")
	}
}

// resolveCardWithoutAuth fetches the agent card without any auth headers.
// Used by the device-auth flow to read SecuritySchemes before authentication.
// Custom --header entries are still applied (they may carry non-auth headers).
func resolveCardWithoutAuth(ctx context.Context, opts Options, hc *http.Client) (*a2a.AgentCard, error) {
	var resolveOpts []agentcard.ResolveOption

	headerEntries, err := parseOptionHeaders(opts.Headers)
	if err != nil {
		return nil, err
	}
	resolveOpts = appendHeaderResolveOpts(resolveOpts, headerEntries)

	card, err := buildResolver(hc).Resolve(ctx, opts.BaseURL, resolveOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent card: %w", err)
	}
	return card, nil
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
