package vertexai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// Client communicates with a Vertex AI Agent Engine A2A endpoint.
// It translates between standard a2a.* types and the Vertex AI
// Protobuf JSON wire format.
type Client struct {
	httpClient   *http.Client
	endpoint     *Endpoint
	getToken     func() (string, error)
	extraHeaders []HeaderEntry
}

// HeaderEntry is a single HTTP header pair injected into every request.
// It mirrors auth.HeaderEntry to avoid a circular import (internal/client
// imports both packages, and vertexai importing auth would create a cycle).
type HeaderEntry struct {
	Key   string
	Value string
}

// NewClient creates a Vertex AI A2A client.
// getToken is called before each request to obtain a fresh OAuth2 access token.
func NewClient(endpoint *Endpoint, getToken func() (string, error)) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 5 * time.Minute},
		endpoint:   endpoint,
		getToken:   getToken,
	}
}

// SetExtraHeaders configures additional HTTP headers to inject into every
// outgoing request. Intended for the --header CLI flag. Must be called
// before any request is made.
//
// Headers are applied with http.Header.Set (after the Authorization header),
// so users can intentionally override Authorization in custom setups.
func (c *Client) SetExtraHeaders(entries []HeaderEntry) {
	c.extraHeaders = entries
}

// FetchCard retrieves the Agent Card from the Vertex AI A2A card endpoint.
func (c *Client) FetchCard(ctx context.Context) (*a2a.AgentCard, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.endpoint.CardURL(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("card request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, readErrorResponse(resp)
	}

	var card a2a.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("failed to decode agent card: %w", err)
	}
	return &card, nil
}

// SendMessage sends a message to the Vertex AI A2A endpoint and returns
// the completed task. It always sends with blocking: true.
func (c *Client) SendMessage(ctx context.Context, a2aReq *a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	wireReq := buildSendRequest(a2aReq.Message)

	body, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.endpoint.SendURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, readErrorResponse(resp)
	}

	var wireResp sendResponse
	if err := json.NewDecoder(resp.Body).Decode(&wireResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return toA2ATask(wireResp.Task), nil
}

// SendStreamingMessage sends a message to the Vertex AI streaming endpoint
// and yields events as they arrive. Multi-line JSON events from sse-starlette
// are buffered per the SSE Living Standard and dispatched on blank lines.
func (c *Client) SendStreamingMessage(ctx context.Context, a2aReq *a2a.SendMessageRequest) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		wireReq := buildStreamRequest(a2aReq.Message)

		body, err := json.Marshal(wireReq)
		if err != nil {
			yield(nil, fmt.Errorf("failed to marshal request: %w", err))
			return
		}

		req, err := c.newRequest(ctx, http.MethodPost, c.endpoint.StreamURL(), bytes.NewReader(body))
		if err != nil {
			yield(nil, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			yield(nil, fmt.Errorf("stream request failed: %w", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			yield(nil, readErrorResponse(resp))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)

		var dataBuf bytes.Buffer

		// dispatch parses the accumulated data buffer as one stream event
		// and yields it. Returns false if the caller stopped iterating.
		dispatch := func() bool {
			if dataBuf.Len() == 0 {
				return true
			}
			// parseStreamEvent must run before Reset(): Buffer.Bytes()
			// aliases the internal slice, which Reset() makes reusable.
			event, err := parseStreamEvent(dataBuf.Bytes())
			dataBuf.Reset()
			if err != nil {
				return yield(nil, err)
			}
			if event == nil {
				return true
			}
			return yield(event, nil)
		}

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				if !dispatch() {
					return
				}
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue
			}

			// Per SSE spec: split "field:value", strip a single leading
			// space from value (not TrimSpace — trailing whitespace matters).
			var field, value string
			if idx := strings.IndexByte(line, ':'); idx >= 0 {
				field = line[:idx]
				value = strings.TrimPrefix(line[idx+1:], " ")
			} else {
				field = line
			}

			if field == "data" {
				if dataBuf.Len() > 0 {
					dataBuf.WriteByte('\n')
				}
				dataBuf.WriteString(value)
			}
			// event, id, retry fields are ignored.
		}

		if err := scanner.Err(); err != nil {
			yield(nil, fmt.Errorf("stream read error: %w", err))
			return
		}

		dispatch()
	}
}

// GetTask retrieves a task by ID from the Vertex AI A2A endpoint.
// HistoryLength, when set on the request, is sent as a ?historyLength=N
// query parameter (camelCase to match the rest of the Vertex AI wire format).
func (c *Client) GetTask(ctx context.Context, a2aReq *a2a.GetTaskRequest) (*a2a.Task, error) {
	taskURL := c.endpoint.TaskURL(string(a2aReq.ID))
	if a2aReq.HistoryLength != nil {
		taskURL += "?historyLength=" + strconv.Itoa(*a2aReq.HistoryLength)
	}

	req, err := c.newRequest(ctx, http.MethodGet, taskURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("task get request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, readErrorResponse(resp)
	}

	var wireResp wireTask
	if err := json.NewDecoder(resp.Body).Decode(&wireResp); err != nil {
		return nil, fmt.Errorf("failed to decode task response: %w", err)
	}
	return toA2ATask(wireResp), nil
}

// CancelTask cancels a task by ID via the Vertex AI A2A endpoint.
func (c *Client) CancelTask(ctx context.Context, a2aReq *a2a.CancelTaskRequest) (*a2a.Task, error) {
	cancelURL := c.endpoint.CancelTaskURL(string(a2aReq.ID))

	req, err := c.newRequest(ctx, http.MethodPost, cancelURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("task cancel request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, readErrorResponse(resp)
	}

	var wireResp wireTask
	if err := json.NewDecoder(resp.Body).Decode(&wireResp); err != nil {
		return nil, fmt.Errorf("failed to decode cancel response: %w", err)
	}
	return toA2ATask(wireResp), nil
}

// Destroy is a no-op for the Vertex AI client (no persistent resources).
func (c *Client) Destroy() error {
	return nil
}

// newRequest creates an HTTP request with the authorization header set.
func (c *Client) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	token, err := c.getToken()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain access token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// Inject user-supplied headers from --header. Set (not Add) so users
	// can intentionally override prior headers such as Authorization.
	for _, h := range c.extraHeaders {
		req.Header.Set(h.Key, h.Value)
	}

	return req, nil
}

// readErrorResponse reads the response body and returns a descriptive error.
func readErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
}
