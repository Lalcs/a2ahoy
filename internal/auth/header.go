package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// HeaderEntry is a single parsed header pair.
type HeaderEntry struct {
	Key   string
	Value string
}

// ErrInvalidHeader is returned when a --header argument cannot be parsed.
// Wrap with fmt.Errorf("...%w...", ErrInvalidHeader) to allow callers to
// classify errors with errors.Is.
var ErrInvalidHeader = errors.New("invalid --header value")

// ParseHeaders parses raw "KEY=VALUE" strings (typically the value of the
// --header flag, which may be repeated) into HeaderEntry values.
//
// Parsing rules:
//   - ""                   → ErrInvalidHeader (empty entry)
//   - "key"                → ErrInvalidHeader (no '=' separator)
//   - "=value"             → ErrInvalidHeader (empty key)
//   - "key="               → OK; value is empty (allowed, matches curl -H)
//   - "key=a=b=c"          → key="key", value="a=b=c" (split on first '=' only)
//   - " key = value "      → key/value are NOT trimmed; users supply exactly
//     what they intend to send on the wire
//
// Input order is preserved so callers can reproduce multi-value HTTP headers
// (e.g. --header Accept=text/html --header Accept=application/json).
func ParseHeaders(entries []string) ([]HeaderEntry, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	parsed := make([]HeaderEntry, 0, len(entries))
	for i, raw := range entries {
		if raw == "" {
			return nil, fmt.Errorf("%w: entry %d is empty", ErrInvalidHeader, i)
		}
		key, val, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("%w: %q (expected KEY=VALUE)", ErrInvalidHeader, raw)
		}
		if key == "" {
			return nil, fmt.Errorf("%w: %q (key is empty)", ErrInvalidHeader, raw)
		}
		parsed = append(parsed, HeaderEntry{Key: key, Value: val})
	}
	return parsed, nil
}

// HeaderInterceptor injects pre-parsed HTTP headers into every outgoing A2A
// request. Use ParseHeaders to obtain the entries from raw --header flag
// values.
type HeaderInterceptor struct {
	a2aclient.PassthroughInterceptor
	headers []HeaderEntry
}

// NewHeaderInterceptor wraps pre-parsed HeaderEntry values in an interceptor.
// The caller retains ownership of the slice; the interceptor does not copy
// or mutate it.
func NewHeaderInterceptor(headers []HeaderEntry) *HeaderInterceptor {
	return &HeaderInterceptor{headers: headers}
}

// Before injects the configured headers into the outgoing request via
// ServiceParams.Append. Per a2aclient.ServiceParams semantics, keys are
// normalised to lowercase; identical key+value pairs are deduped, while
// distinct values for the same key are preserved as multi-value HTTP
// headers.
func (h *HeaderInterceptor) Before(ctx context.Context, req *a2aclient.Request) (context.Context, any, error) {
	for _, e := range h.headers {
		req.ServiceParams.Append(e.Key, e.Value)
	}
	return ctx, nil, nil
}
