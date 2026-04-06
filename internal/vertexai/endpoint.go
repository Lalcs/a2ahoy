package vertexai

import (
	"fmt"
	"net/url"
	"strings"
)

// Endpoint holds a normalized Vertex AI Reasoning Engine base URL and
// provides methods to construct A2A-specific endpoint paths.
type Endpoint struct {
	base string // v1beta1-normalized base URL without trailing slash
}

// ParseEndpoint parses a Vertex AI Reasoning Engine URL and normalizes it.
// It replaces "/v1/" with "/v1beta1/" (required for A2A endpoints) and
// strips any trailing slash.
func ParseEndpoint(rawURL string) (*Endpoint, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("empty URL")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("URL must include scheme and host: %s", rawURL)
	}

	// Normalize: replace /v1/ with /v1beta1/ for A2A endpoint compatibility.
	// Only replace the API version segment, not arbitrary occurrences.
	path := u.Path
	path = normalizeAPIVersion(path)
	path = strings.TrimRight(path, "/")
	path = strings.TrimSuffix(path, ":query")
	u.Path = path

	return &Endpoint{base: u.String()}, nil
}

// normalizeAPIVersion replaces the Vertex AI API version segment
// "/v1/" or "/v1" (at end of relevant path segment) with "/v1beta1/".
func normalizeAPIVersion(path string) string {
	// Match "/v1/projects/" specifically to avoid replacing "/v1beta1/" or other paths.
	if strings.Contains(path, "/v1/projects/") {
		return strings.Replace(path, "/v1/projects/", "/v1beta1/projects/", 1)
	}
	return path
}

// CardURL returns the URL for fetching the Agent Card.
func (e *Endpoint) CardURL() string {
	return e.base + "/a2a/v1/card"
}

// SendURL returns the URL for sending a message (blocking or non-blocking).
func (e *Endpoint) SendURL() string {
	return e.base + "/a2a/v1/message:send"
}

// StreamURL returns the URL for streaming a message via SSE.
func (e *Endpoint) StreamURL() string {
	return e.base + "/a2a/v1/message:stream"
}

// TaskURL returns the URL for retrieving a specific task by ID.
func (e *Endpoint) TaskURL(taskID string) string {
	return e.base + "/a2a/v1/tasks/" + taskID
}

// CancelTaskURL returns the URL for cancelling a specific task.
func (e *Endpoint) CancelTaskURL(taskID string) string {
	return e.base + "/a2a/v1/tasks/" + taskID + ":cancel"
}
