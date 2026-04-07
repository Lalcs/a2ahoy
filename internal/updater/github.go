package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultGitHubAPI is the upstream releases endpoint that the install.sh
// script targets. The Go module path (github.com/khayashi/a2ahoy) differs
// from the GitHub repository path because the binaries are published under
// the Lalcs/a2ahoy organisation.
const defaultGitHubAPI = "https://api.github.com/repos/Lalcs/a2ahoy/releases/latest"

// userAgent is sent with every API request. GitHub returns 403 if the
// User-Agent header is missing on unauthenticated calls.
const userAgent = "a2ahoy-update-cli"

// Release is a minimal projection of the GitHub Releases API response. We
// only decode the fields that the update flow actually needs so the struct
// is forward-compatible with new GitHub fields.
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset represents one downloadable file attached to a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// FindAssetForPlatform locates an asset whose Name matches the given
// platform-specific filename. It returns an error if no matching asset
// exists in the release.
func (r *Release) FindAssetForPlatform(name string) (*Asset, error) {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i], nil
		}
	}
	available := make([]string, 0, len(r.Assets))
	for i := range r.Assets {
		available = append(available, r.Assets[i].Name)
	}
	return nil, fmt.Errorf(
		"no asset named %q in release %s (available: %s)",
		name, r.TagName, strings.Join(available, ", "))
}

// Fetcher abstracts retrieval of the latest release. The interface exists
// so command-level code can be tested with a fake implementation if it ever
// grows beyond a thin wire-up layer.
type Fetcher interface {
	FetchLatestRelease(ctx context.Context) (*Release, error)
}

// GitHubFetcher is the production Fetcher implementation. It talks directly
// to the public GitHub REST API.
type GitHubFetcher struct {
	httpClient *http.Client
	apiURL     string
}

// NewGitHubFetcher returns a Fetcher targeting the canonical Lalcs/a2ahoy
// release endpoint with a 30-second HTTP timeout.
func NewGitHubFetcher() *GitHubFetcher {
	return &GitHubFetcher{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiURL:     defaultGitHubAPI,
	}
}

// FetchLatestRelease implements Fetcher. It returns a parsed Release on
// success or a wrapped error describing the failure.
func (f *GitHubFetcher) FetchLatestRelease(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, readGitHubError(resp)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode github release: %w", err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("github release has empty tag_name")
	}
	return &rel, nil
}

// readGitHubError reads up to 4 KiB of the response body and formats a
// helpful error. When GitHub returns 403 with a rate-limit message we
// surface a hint so the user understands how to recover.
func readGitHubError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyStr := strings.TrimSpace(string(body))

	if resp.StatusCode == http.StatusForbidden &&
		strings.Contains(strings.ToLower(bodyStr), "rate limit") {
		return fmt.Errorf(
			"github API rate limit exceeded (HTTP %d): %s\nhint: try again later or use install.sh",
			resp.StatusCode, bodyStr)
	}
	return fmt.Errorf("github API returned HTTP %d: %s", resp.StatusCode, bodyStr)
}
