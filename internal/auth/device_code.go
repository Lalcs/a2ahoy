package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// Device Code flow sentinel errors.
var (
	// ErrDeviceCodeExpired is returned when the device code expires before
	// the user completes authentication.
	ErrDeviceCodeExpired = errors.New("device code expired before user completed authentication")

	// ErrAccessDenied is returned when the user denies the authorization request.
	ErrAccessDenied = errors.New("user denied the authorization request")

	// ErrMissingClientID is returned when no client ID is provided for the
	// device code flow.
	ErrMissingClientID = errors.New("client ID is required for device code auth")

	// ErrMissingDeviceAuthURL is returned when no device authorization
	// endpoint URL is provided.
	ErrMissingDeviceAuthURL = errors.New("device authorization URL is required for device code auth")

	// ErrMissingTokenURL is returned when no token endpoint URL is provided.
	ErrMissingTokenURL = errors.New("token URL is required for device code auth")

	// ErrMissingPromptOutput is returned when no prompt writer is provided.
	ErrMissingPromptOutput = errors.New("prompt output is required for device code auth")
)

// defaultPollInterval is the default polling interval in seconds when the
// server does not specify one (RFC 8628 section 3.2).
const defaultPollInterval = 5

// slowDownIncrement is added to the polling interval when the server
// returns a "slow_down" error (RFC 8628 section 3.5).
const slowDownIncrement = 5

// deviceCodeGrantType is the grant_type value for the device code flow.
const deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// Token endpoint error codes defined in RFC 8628 section 3.5.
const (
	tokenErrAuthorizationPending = "authorization_pending"
	tokenErrSlowDown             = "slow_down"
	tokenErrExpiredToken         = "expired_token"
	tokenErrAccessDenied         = "access_denied"
)

// pollSleepFn is the function used to wait between polling attempts.
// It can be overridden in tests to eliminate real sleeps.
var pollSleepFn = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// DeviceCodeConfig holds the parameters needed to execute the device code flow.
type DeviceCodeConfig struct {
	// ClientID is the OAuth2 client ID (required).
	ClientID string
	// DeviceAuthorizationURL is the device authorization endpoint (required).
	DeviceAuthorizationURL string
	// TokenURL is the token endpoint (required).
	TokenURL string
	// Scopes is an optional list of OAuth2 scopes.
	Scopes []string
}

// validate checks that all required fields are set.
func (c DeviceCodeConfig) validate() error {
	if c.ClientID == "" {
		return ErrMissingClientID
	}
	if c.DeviceAuthorizationURL == "" {
		return ErrMissingDeviceAuthURL
	}
	if c.TokenURL == "" {
		return ErrMissingTokenURL
	}
	return nil
}

// DeviceCodeResponse represents the server's response to the device
// authorization request (RFC 8628 section 3.2).
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// tokenResponse represents a successful token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
}

// tokenErrorResponse represents an error from the token endpoint.
type tokenErrorResponse struct {
	Error string `json:"error"`
}

// DeviceCodeInterceptor injects an OAuth2 access token obtained via the
// Device Authorization Grant (RFC 8628) as a Bearer token into every
// outgoing A2A request. The token is obtained once at construction time
// and reused for the lifetime of the interceptor.
type DeviceCodeInterceptor struct {
	a2aclient.PassthroughInterceptor
	token string
}

// NewDeviceCodeInterceptor executes the full device code flow:
//  1. POST to device_authorization_endpoint to get user_code + verification_uri
//  2. Print instructions to promptOut (typically os.Stderr)
//  3. Poll token_endpoint until user completes browser auth
//  4. Return interceptor with the resulting access token
//
// httpClient is optional (nil uses http.DefaultClient). promptOut is
// where interactive prompts are written.
func NewDeviceCodeInterceptor(ctx context.Context, cfg DeviceCodeConfig, promptOut io.Writer, httpClient *http.Client) (*DeviceCodeInterceptor, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if promptOut == nil {
		return nil, ErrMissingPromptOutput
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Step 1: Request device code.
	resp, err := requestDeviceCode(ctx, httpClient, cfg)
	if err != nil {
		return nil, fmt.Errorf("device authorization request failed: %w", err)
	}

	// Step 2: Display prompt to user.
	printDeviceCodePrompt(promptOut, resp)

	// Step 3: Poll for token.
	interval := resp.Interval
	if interval <= 0 {
		interval = defaultPollInterval
	}
	token, err := pollForToken(ctx, httpClient, cfg.TokenURL, cfg.ClientID, resp.DeviceCode, interval, resp.ExpiresIn)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(promptOut, "Authentication successful!")

	return &DeviceCodeInterceptor{token: token}, nil
}

// NewDeviceCodeInterceptorFromToken creates a DeviceCodeInterceptor from a
// pre-existing token. Useful for testing and callers that obtain tokens
// through alternative mechanisms.
func NewDeviceCodeInterceptorFromToken(token string) *DeviceCodeInterceptor {
	return &DeviceCodeInterceptor{token: token}
}

// GetToken returns the access token for use outside the interceptor
// (e.g., for adding auth headers to agent card resolution requests).
func (d *DeviceCodeInterceptor) GetToken() (string, error) {
	return d.token, nil
}

// Before injects the Authorization header with the access token.
func (d *DeviceCodeInterceptor) Before(ctx context.Context, req *a2aclient.Request) (context.Context, any, error) {
	req.ServiceParams.Append("authorization", "Bearer "+d.token)
	return ctx, nil, nil
}

// printDeviceCodePrompt writes the user-facing authentication instructions
// to the given writer.
func printDeviceCodePrompt(w io.Writer, resp *DeviceCodeResponse) {
	if resp.VerificationURIComplete != "" {
		fmt.Fprintln(w, "To authenticate, open this URL in your browser:")
		fmt.Fprintf(w, "  %s\n\n", resp.VerificationURIComplete)
		fmt.Fprintf(w, "Or visit %s and enter code: %s\n", resp.VerificationURI, resp.UserCode)
	} else {
		fmt.Fprintf(w, "To authenticate, visit: %s\n", resp.VerificationURI)
		fmt.Fprintf(w, "Enter code: %s\n", resp.UserCode)
	}
	fmt.Fprintln(w, "Waiting for authentication...")
}

// requestDeviceCode sends the initial POST to the device authorization
// endpoint (RFC 8628 section 3.1).
func requestDeviceCode(ctx context.Context, httpClient *http.Client, cfg DeviceCodeConfig) (*DeviceCodeResponse, error) {
	data := url.Values{
		"client_id": {cfg.ClientID},
	}
	if len(cfg.Scopes) > 0 {
		data.Set("scope", strings.Join(cfg.Scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.DeviceAuthorizationURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device authorization endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var dcResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcResp); err != nil {
		return nil, fmt.Errorf("failed to decode device authorization response: %w", err)
	}

	if dcResp.DeviceCode == "" || dcResp.UserCode == "" || dcResp.VerificationURI == "" {
		return nil, fmt.Errorf("device authorization response missing required fields (device_code, user_code, or verification_uri)")
	}

	return &dcResp, nil
}

// pollForToken polls the token endpoint until the user completes
// authentication, the device code expires, or the context is cancelled
// (RFC 8628 section 3.4–3.5).
func pollForToken(ctx context.Context, httpClient *http.Client, tokenURL, clientID, deviceCode string, interval, expiresIn int) (string, error) {
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)

	for {
		// Check deadline before sleeping.
		if expiresIn > 0 && time.Now().After(deadline) {
			return "", ErrDeviceCodeExpired
		}

		// Wait for the polling interval, respecting context cancellation.
		if err := pollSleepFn(ctx, time.Duration(interval)*time.Second); err != nil {
			return "", err
		}

		// Check deadline after sleeping.
		if expiresIn > 0 && time.Now().After(deadline) {
			return "", ErrDeviceCodeExpired
		}

		token, retry, err := requestToken(ctx, httpClient, tokenURL, clientID, deviceCode)
		if err != nil {
			if errors.Is(err, errSlowDown) {
				// RFC 8628 section 3.5: increase interval by 5 seconds.
				interval += slowDownIncrement
				continue
			}
			return "", err
		}
		if !retry {
			return token, nil
		}
	}
}

// requestToken sends a single token request and interprets the response.
// Returns the access token on success, or (empty, true, nil) to keep
// polling. Returns errSlowDown to signal interval increase.
func requestToken(ctx context.Context, httpClient *http.Client, tokenURL, clientID, deviceCode string) (token string, retry bool, err error) {
	data := url.Values{
		"grant_type":  {deviceCodeGrantType},
		"device_code": {deviceCode},
		"client_id":   {clientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", false, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("failed to read token response: %w", err)
	}

	// Success case: 200 OK with access_token.
	if resp.StatusCode == http.StatusOK {
		var tokenResp tokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return "", false, fmt.Errorf("failed to decode token response: %w", err)
		}
		if tokenResp.AccessToken == "" {
			return "", false, fmt.Errorf("token response missing access_token")
		}
		return tokenResp.AccessToken, false, nil
	}

	// Error case: parse the error field.
	var errResp tokenErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return "", false, fmt.Errorf("token endpoint returned HTTP %d with non-JSON body: %s", resp.StatusCode, string(body))
	}

	switch errResp.Error {
	case tokenErrAuthorizationPending:
		return "", true, nil
	case tokenErrSlowDown:
		return "", false, errSlowDown
	case tokenErrExpiredToken:
		return "", false, ErrDeviceCodeExpired
	case tokenErrAccessDenied:
		return "", false, ErrAccessDenied
	default:
		return "", false, fmt.Errorf("token endpoint error: %s", errResp.Error)
	}
}

// errSlowDown is an internal sentinel used to signal that pollForToken
// should increase its polling interval.
var errSlowDown = errors.New("slow_down")
