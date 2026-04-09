// Package httptrace provides HTTP transport-level diagnostics for the CLI.
package httptrace

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
)

// VerboseTransport is an http.RoundTripper that dumps every request and
// response to Out using httputil.DumpRequestOut / DumpResponse.
// It wraps an Inner transport that performs the actual network I/O.
type VerboseTransport struct {
	Out   io.Writer         // destination for dumps (typically os.Stderr)
	Inner http.RoundTripper // actual transport; uses http.DefaultTransport if nil
}

// RoundTrip implements http.RoundTripper. It dumps the outgoing request,
// delegates to the inner transport, and dumps the response (or error).
func (t *VerboseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	inner := t.Inner
	if inner == nil {
		inner = http.DefaultTransport
	}

	// DumpRequestOut is the client-side variant: it correctly handles
	// Transfer-Encoding and reads from req.Body via a pipe.
	dump, err := httputil.DumpRequestOut(req, true)
	if err == nil {
		_, _ = fmt.Fprintf(t.Out, "--- REQUEST ---\n%s\n", dump)
	}

	resp, rtErr := inner.RoundTrip(req)

	if rtErr != nil {
		_, _ = fmt.Fprintf(t.Out, "--- ERROR ---\n%s\n", rtErr)
		return nil, rtErr
	}

	// For streaming responses (SSE), dump headers only to avoid buffering
	// the entire stream into memory and blocking until the server closes
	// the connection.
	dumpBody := !isStreamingResponse(resp)
	dump, err = httputil.DumpResponse(resp, dumpBody)
	if err == nil {
		_, _ = fmt.Fprintf(t.Out, "--- RESPONSE ---\n%s\n", dump)
	}

	return resp, nil
}

// isStreamingResponse reports whether resp is a server-sent event stream
// whose body must not be fully buffered (DumpResponse with body=true would
// block until the server closes the connection).
func isStreamingResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "text/event-stream")
}

// WrapClient returns a shallow copy of hc whose Transport is replaced with
// a VerboseTransport writing to w. If hc is nil a new http.Client is
// created. The original client is never mutated.
func WrapClient(hc *http.Client, w io.Writer) *http.Client {
	if hc == nil {
		hc = &http.Client{}
	}
	// Shallow copy so Timeout, Jar, CheckRedirect, etc. are preserved.
	clone := *hc
	clone.Transport = &VerboseTransport{
		Out:   w,
		Inner: hc.Transport, // nil is fine — RoundTrip falls back to DefaultTransport
	}
	return &clone
}
