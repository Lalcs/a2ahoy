// Package cardcheck performs static validation of A2A Agent Cards.
//
// The package inspects a resolved *a2a.AgentCard and returns a list of
// Issues describing structural problems, specification violations, and
// known compatibility gotchas (e.g., the a2a-go v0.3 HTTP+JSON "/v1"
// path prefix bug compensated for by internal/client.applyV1PathPrefix).
//
// cardcheck is a pure inspection layer: it has no dependency on networking,
// authentication, presentation, or the rest of the a2ahoy application. It
// only depends on github.com/a2aproject/a2a-go/v2/a2a. This keeps the
// package trivially testable and reusable from other commands (e.g., a
// future `a2ahoy doctor` subcommand).
//
// The presentation layer is deliberately separated: see
// internal/presenter/card.go PrintValidation and PrintValidationSummary
// for the human- and machine-readable renderers that consume Result.
package cardcheck

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// Level is the severity of a validation Issue.
//
// Ordering: LevelInfo < LevelWarning < LevelError. Run returns issues
// sorted by Level descending (errors first) so callers that truncate the
// output still see the most important items.
type Level int

const (
	// LevelInfo is informational commentary that does not indicate a
	// problem. Users can safely ignore these unless they are diagnosing
	// a specific issue.
	LevelInfo Level = iota
	// LevelWarning indicates a likely problem that a2ahoy can still
	// work around (e.g., v0.3 HTTP+JSON URL missing the "/v1" prefix
	// is automatically rewritten by internal/client.applyV1PathPrefix).
	LevelWarning
	// LevelError indicates a structural problem severe enough that
	// downstream commands (send/stream/get/cancel) are expected to
	// fail. The `card` command exits non-zero when any LevelError
	// issue is present.
	LevelError
)

// String returns the lower-case level name used in issue output.
func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "info"
	case LevelWarning:
		return "warning"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// Issue describes a single validation finding.
//
// Code is a stable machine-readable identifier (e.g.,
// "V03_HTTPJSON_MISSING_V1"). Scripts can grep for Code without worrying
// about Message wording changes.
//
// Field is a dot-path pointing at the offending part of the card (e.g.,
// "supportedInterfaces[0].url"). Empty when the issue is not tied to a
// specific field.
//
// Hint is an optional suggested fix. Populated only for mechanical fixes
// ("append \"/v1\" to the URL"); left empty for structural issues where
// there is no canned remedy.
type Issue struct {
	Level   Level
	Code    string
	Message string
	Field   string
	Hint    string
}

// Result is the aggregated output of Run.
type Result struct {
	Issues []Issue
}

// HasIssues reports whether any issues were found, regardless of level.
func (r Result) HasIssues() bool {
	return len(r.Issues) > 0
}

// HasErrors reports whether any LevelError issues were found.
func (r Result) HasErrors() bool {
	for _, iss := range r.Issues {
		if iss.Level == LevelError {
			return true
		}
	}
	return false
}

// Count returns the number of issues at the given level.
func (r Result) Count(level Level) int {
	n := 0
	for _, iss := range r.Issues {
		if iss.Level == level {
			n++
		}
	}
	return n
}

// ByLevel returns the issues at the given level, in their original order.
func (r Result) ByLevel(level Level) []Issue {
	out := make([]Issue, 0, len(r.Issues))
	for _, iss := range r.Issues {
		if iss.Level == level {
			out = append(out, iss)
		}
	}
	return out
}

// allChecks is the ordered list of check functions executed by Run. Each
// check is independent: it inspects the card and returns zero or more
// Issues without touching shared state.
//
// To add a new check: write a function with this signature, place it in
// this file, and register it here. The position in the slice determines
// the order within a single Level in Run's output.
var allChecks = []func(*a2a.AgentCard) []Issue{
	// Priority A — structural / spec-mandatory.
	checkName,
	checkVersion,
	checkSupportedInterfacesEmpty,
	checkInterfaces,
	checkV03HTTPJSONMissingV1,

	// Priority B — recommended sanity checks.
	checkStreamingCapabilityHasTransport,
	checkDuplicateInterfaceURLBinding,
	checkSkills,
	checkProtocolVersionRecognized,
}

// Run executes all registered checks against card and returns a Result
// with issues sorted by Level descending (Error → Warning → Info). Within
// a single Level, the order follows the registration order in allChecks
// and the order individual checks return issues, giving deterministic
// output.
//
// A nil card yields an empty Result. Callers typically get a non-nil
// card from client.ResolveCard, but defensive handling avoids panics in
// tests and future call sites.
func Run(card *a2a.AgentCard) Result {
	if card == nil {
		return Result{}
	}
	var issues []Issue
	for _, check := range allChecks {
		issues = append(issues, check(card)...)
	}
	return Result{Issues: sortByLevelDesc(issues)}
}

// sortByLevelDesc returns a new slice with issues ordered Error →
// Warning → Info while preserving the relative order of issues at the
// same level (stable sort). Because Level is defined via iota with
// LevelError > LevelWarning > LevelInfo, descending order by Level
// is just `a > b`.
func sortByLevelDesc(issues []Issue) []Issue {
	if len(issues) <= 1 {
		return issues
	}
	out := make([]Issue, len(issues))
	copy(out, issues)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Level > out[j].Level
	})
	return out
}

// -----------------------------------------------------------------------------
// Priority A checks
// -----------------------------------------------------------------------------

// checkName reports an error when the card name is missing. Name is
// required by both the A2A v0.3 and v1.0 specifications and is shown as
// the primary identifier in every presenter.
func checkName(card *a2a.AgentCard) []Issue {
	if strings.TrimSpace(card.Name) != "" {
		return nil
	}
	return []Issue{{
		Level:   LevelError,
		Code:    "EMPTY_NAME",
		Message: "agent card has no name; the spec requires a non-empty name.",
		Field:   "name",
	}}
}

// checkVersion reports a warning when the card version is missing. The
// spec requires it, but many real-world servers omit the field and
// a2ahoy still functions, so this is a warning rather than an error.
func checkVersion(card *a2a.AgentCard) []Issue {
	if strings.TrimSpace(card.Version) != "" {
		return nil
	}
	return []Issue{{
		Level:   LevelWarning,
		Code:    "EMPTY_VERSION",
		Message: "agent card has no version; the spec requires a non-empty version string.",
		Field:   "version",
	}}
}

// checkSupportedInterfacesEmpty reports an error when no interfaces are
// advertised. Without at least one entry, send/stream/get/cancel cannot
// route a request and will fail later with an unfriendly "no supported
// interfaces" error. Emitting this as an explicit check helps users
// diagnose the problem at `card` time rather than at send time.
func checkSupportedInterfacesEmpty(card *a2a.AgentCard) []Issue {
	if len(card.SupportedInterfaces) > 0 {
		return nil
	}
	return []Issue{{
		Level:   LevelError,
		Code:    "EMPTY_SUPPORTED_INTERFACES",
		Message: "agent card advertises no interfaces; send/stream/get/cancel will fail.",
		Field:   "supportedInterfaces",
		Hint:    "ensure the server publishes at least one entry in supportedInterfaces (v1.0) or top-level url + preferredTransport (v0.3).",
	}}
}

// knownProtocolBindings lists the transport bindings registered in
// a2a-go v2.1.0. Unrecognized values are allowed by the spec (custom
// bindings are permitted) but a2ahoy cannot route against them.
var knownProtocolBindings = map[a2a.TransportProtocol]bool{
	a2a.TransportProtocolJSONRPC:  true,
	a2a.TransportProtocolGRPC:     true,
	a2a.TransportProtocolHTTPJSON: true,
}

// validURLSchemes enumerates the URL schemes a2ahoy can actually talk
// to. http/https cover JSONRPC and HTTP+JSON; grpc/grpcs cover gRPC
// transports.
var validURLSchemes = map[string]bool{
	"http":  true,
	"https": true,
	"grpc":  true,
	"grpcs": true,
}

// checkInterfaces inspects each SupportedInterfaces entry for structural
// problems. Combined into a single loop so each interface is checked
// against all rules in one pass:
//
//   - INTERFACE_INVALID_URL (error) — URL is not parseable or uses an
//     unrecognized scheme.
//   - INTERFACE_EMPTY_PROTOCOL_VERSION (warning) — cannot reason about
//     version-specific rules without a ProtocolVersion.
//   - INTERFACE_UNKNOWN_PROTOCOL_BINDING (warning) — binding is not one
//     of JSONRPC/GRPC/HTTP+JSON.
func checkInterfaces(card *a2a.AgentCard) []Issue {
	var out []Issue
	for i, iface := range card.SupportedInterfaces {
		if iface == nil {
			continue
		}
		prefix := fmt.Sprintf("supportedInterfaces[%d]", i)

		if msg := validateInterfaceURL(iface.URL); msg != "" {
			out = append(out, Issue{
				Level:   LevelError,
				Code:    "INTERFACE_INVALID_URL",
				Message: msg,
				Field:   prefix + ".url",
			})
		}

		if strings.TrimSpace(string(iface.ProtocolVersion)) == "" {
			out = append(out, Issue{
				Level:   LevelWarning,
				Code:    "INTERFACE_EMPTY_PROTOCOL_VERSION",
				Message: "interface has no protocolVersion; version-specific checks are skipped.",
				Field:   prefix + ".protocolVersion",
			})
		}

		if iface.ProtocolBinding != "" && !knownProtocolBindings[iface.ProtocolBinding] {
			out = append(out, Issue{
				Level: LevelWarning,
				Code:  "INTERFACE_UNKNOWN_PROTOCOL_BINDING",
				Message: fmt.Sprintf(
					"interface uses unrecognized protocolBinding %q; a2ahoy only routes against JSONRPC, GRPC, and HTTP+JSON.",
					iface.ProtocolBinding,
				),
				Field: prefix + ".protocolBinding",
			})
		}
	}
	return out
}

// validateInterfaceURL returns a human-readable error message for an
// interface URL, or "" if the URL is acceptable. The three failure
// modes are: unparseable, relative (no scheme), and unrecognized
// scheme.
func validateInterfaceURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "interface url is empty."
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Sprintf("interface url is not parseable: %v.", err)
	}
	if u.Scheme == "" {
		return fmt.Sprintf("interface url %q is relative (no scheme); must be an absolute URL.", raw)
	}
	if !validURLSchemes[strings.ToLower(u.Scheme)] {
		return fmt.Sprintf("interface url %q uses unrecognized scheme %q; expected http, https, grpc, or grpcs.", raw, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Sprintf("interface url %q has no host component.", raw)
	}
	return ""
}

// -----------------------------------------------------------------------------
// Priority B checks
// -----------------------------------------------------------------------------

// checkStreamingCapabilityHasTransport reports a warning when the card
// advertises streaming support but has no interfaces at all. The spec does
// not restrict streaming to a specific transport (JSONRPC, HTTP+JSON, and
// gRPC all support streaming in A2A), so we only flag the degenerate
// empty-interfaces case here. A more aggressive per-transport check would
// raise false positives for legitimate deployments.
func checkStreamingCapabilityHasTransport(card *a2a.AgentCard) []Issue {
	if !card.Capabilities.Streaming {
		return nil
	}
	if len(card.SupportedInterfaces) > 0 {
		return nil
	}
	return []Issue{{
		Level:   LevelWarning,
		Code:    "STREAMING_NO_COMPATIBLE_TRANSPORT",
		Message: "capabilities.streaming=true but no interfaces are advertised; streaming requests will fail.",
		Field:   "capabilities.streaming",
	}}
}

// checkDuplicateInterfaceURLBinding reports a warning when the same
// (URL, ProtocolBinding) pair appears more than once in
// SupportedInterfaces. Different ProtocolVersion values on the same
// (URL, binding) pair are legitimate (common migration pattern where a
// server advertises both v0.3 and v1.0 on the same endpoint) and are not
// flagged.
func checkDuplicateInterfaceURLBinding(card *a2a.AgentCard) []Issue {
	type key struct {
		url     string
		binding a2a.TransportProtocol
	}
	seen := make(map[key]int)
	var out []Issue
	for i, iface := range card.SupportedInterfaces {
		if iface == nil {
			continue
		}
		k := key{url: iface.URL, binding: iface.ProtocolBinding}
		if first, ok := seen[k]; ok {
			out = append(out, Issue{
				Level: LevelWarning,
				Code:  "DUPLICATE_INTERFACE_URL_BINDING",
				Message: fmt.Sprintf(
					"supportedInterfaces[%d] duplicates the (url, protocolBinding) of supportedInterfaces[%d].",
					i, first,
				),
				Field: fmt.Sprintf("supportedInterfaces[%d]", i),
			})
			continue
		}
		seen[k] = i
	}
	return out
}

// checkSkills inspects each skill for duplicate IDs and empty names.
// These are minor integrity issues; they do not affect routing but
// break client-side displays and analytics that key on skill.id.
func checkSkills(card *a2a.AgentCard) []Issue {
	var out []Issue
	seenID := make(map[string]int)
	for i, skill := range card.Skills {
		// SKILL_DUPLICATE_ID
		if skill.ID != "" {
			if first, ok := seenID[skill.ID]; ok {
				out = append(out, Issue{
					Level: LevelWarning,
					Code:  "SKILL_DUPLICATE_ID",
					Message: fmt.Sprintf(
						"skills[%d].id %q duplicates skills[%d].id.",
						i, skill.ID, first,
					),
					Field: fmt.Sprintf("skills[%d].id", i),
				})
			} else {
				seenID[skill.ID] = i
			}
		}

		// SKILL_EMPTY_NAME: only flag when an ID is present (otherwise
		// the skill is so broken we'd rather not double-report; the
		// missing ID itself is a separate concern we don't currently
		// check — a future check could cover it).
		if skill.ID != "" && strings.TrimSpace(skill.Name) == "" {
			out = append(out, Issue{
				Level: LevelWarning,
				Code:  "SKILL_EMPTY_NAME",
				Message: fmt.Sprintf(
					"skills[%d] has id %q but an empty name.",
					i, skill.ID,
				),
				Field: fmt.Sprintf("skills[%d].name", i),
			})
		}
	}
	return out
}

// checkProtocolVersionRecognized reports an INFO-level note when an
// interface advertises a ProtocolVersion a2ahoy does not recognize
// (anything that does not start with "0.3" or "1."). Custom protocol
// versions are allowed by the spec, so this is informational only —
// it helps users diagnose "why isn't a2ahoy picking up my transport?".
func checkProtocolVersionRecognized(card *a2a.AgentCard) []Issue {
	var out []Issue
	for i, iface := range card.SupportedInterfaces {
		if iface == nil {
			continue
		}
		ver := strings.TrimSpace(string(iface.ProtocolVersion))
		if ver == "" {
			// Already reported by checkInterfaces as
			// INTERFACE_EMPTY_PROTOCOL_VERSION; skip here to avoid
			// double-flagging.
			continue
		}
		if strings.HasPrefix(ver, "0.3") || strings.HasPrefix(ver, "1.") {
			continue
		}
		out = append(out, Issue{
			Level: LevelInfo,
			Code:  "PROTOCOL_VERSION_UNRECOGNIZED",
			Message: fmt.Sprintf(
				"supportedInterfaces[%d] advertises protocolVersion %q; a2ahoy recognizes only 0.3.x and 1.x.",
				i, ver,
			),
			Field: fmt.Sprintf("supportedInterfaces[%d].protocolVersion", i),
		})
	}
	return out
}

// checkV03HTTPJSONMissingV1 reports a warning when an HTTP+JSON interface
// advertises A2A v0.3 and the URL does not end with "/v1".
//
// Background: the a2a-go v2.1.0 REST compat transport omits the "/v1"
// prefix from paths like /message:send, but the A2A v0.3 specification
// (and its reference implementation, Python a2a-sdk) serves these routes
// under /v1/*. Without the workaround in internal/client.applyV1PathPrefix
// (see client.go), `send`/`stream`/`get`/`cancel` against such servers
// fail with 404.
//
// See also: internal/client/client.go applyV1PathPrefix — the live
// mutation path used by `send` et al. The condition here must remain
// identical to the one there. If one side changes, the other should
// change too. The test suites are intentionally mirrored.
func checkV03HTTPJSONMissingV1(card *a2a.AgentCard) []Issue {
	var out []Issue
	for i, iface := range card.SupportedInterfaces {
		if iface == nil {
			continue
		}
		if iface.ProtocolBinding != a2a.TransportProtocolHTTPJSON {
			continue
		}
		// Match any 0.3.x — the constant a2av0.Version is "0.3", but
		// cards commonly carry "0.3.0", "0.3.1", etc.
		if !strings.HasPrefix(string(iface.ProtocolVersion), "0.3") {
			continue
		}
		trimmed := strings.TrimRight(iface.URL, "/")
		if strings.HasSuffix(trimmed, "/v1") {
			continue
		}
		out = append(out, Issue{
			Level: LevelWarning,
			Code:  "V03_HTTPJSON_MISSING_V1",
			Message: fmt.Sprintf(
				"HTTP+JSON interface advertises A2A v0.3 but URL %q lacks the required /v1 path prefix.",
				iface.URL,
			),
			Field: fmt.Sprintf("supportedInterfaces[%d].url", i),
			Hint:  fmt.Sprintf("append \"/v1\" to the URL (e.g., %s/v1).", trimmed),
		})
	}
	return out
}
