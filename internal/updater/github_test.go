package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestFetcher(server *httptest.Server) *GitHubFetcher {
	return &GitHubFetcher{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		apiURL:     server.URL,
	}
}

func TestGitHubFetcher_Success(t *testing.T) {
	const body = `{
        "tag_name": "v1.2.3",
        "name": "Release v1.2.3",
        "html_url": "https://github.com/Lalcs/a2ahoy/releases/tag/v1.2.3",
        "assets": [
            {
                "name": "a2ahoy-darwin-arm64",
                "browser_download_url": "https://example.com/a2ahoy-darwin-arm64",
                "size": 1234
            },
            {
                "name": "a2ahoy-linux-amd64",
                "browser_download_url": "https://example.com/a2ahoy-linux-amd64",
                "size": 5678
            }
        ]
    }`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header = %q, want application/vnd.github+json", got)
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Error("User-Agent header is required")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	fetcher := newTestFetcher(server)
	rel, err := fetcher.FetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v1.2.3" {
		t.Errorf("TagName = %q, want v1.2.3", rel.TagName)
	}
	if rel.Name != "Release v1.2.3" {
		t.Errorf("Name = %q, want Release v1.2.3", rel.Name)
	}
	if got := len(rel.Assets); got != 2 {
		t.Fatalf("Assets length = %d, want 2", got)
	}
	if rel.Assets[0].Name != "a2ahoy-darwin-arm64" {
		t.Errorf("Assets[0].Name = %q", rel.Assets[0].Name)
	}
	if rel.Assets[0].Size != 1234 {
		t.Errorf("Assets[0].Size = %d, want 1234", rel.Assets[0].Size)
	}
}

func TestGitHubFetcher_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	fetcher := newTestFetcher(server)
	_, err := fetcher.FetchLatestRelease(context.Background())
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should include status code: %v", err)
	}
}

func TestGitHubFetcher_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	t.Cleanup(server.Close)

	fetcher := newTestFetcher(server)
	_, err := fetcher.FetchLatestRelease(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 rate limit")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "rate limit") {
		t.Errorf("error should mention rate limit: %v", err)
	}
	if !strings.Contains(msg, "install.sh") {
		t.Errorf("error should hint at install.sh: %v", err)
	}
}

func TestGitHubFetcher_EmptyTagName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "", "assets": []}`))
	}))
	t.Cleanup(server.Close)

	fetcher := newTestFetcher(server)
	_, err := fetcher.FetchLatestRelease(context.Background())
	if err == nil {
		t.Fatal("expected error for empty tag_name")
	}
	if !strings.Contains(err.Error(), "empty tag_name") {
		t.Errorf("error should mention empty tag_name: %v", err)
	}
}

func TestGitHubFetcher_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	t.Cleanup(server.Close)

	fetcher := newTestFetcher(server)
	_, err := fetcher.FetchLatestRelease(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error should mention decode failure: %v", err)
	}
}

func TestGitHubFetcher_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until context is cancelled
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	fetcher := newTestFetcher(server)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := fetcher.FetchLatestRelease(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestRelease_FindAssetForPlatform_Found(t *testing.T) {
	rel := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{Name: "a2ahoy-linux-amd64", BrowserDownloadURL: "https://example.com/1"},
			{Name: "a2ahoy-darwin-arm64", BrowserDownloadURL: "https://example.com/2"},
		},
	}
	asset, err := rel.FindAssetForPlatform("a2ahoy-darwin-arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asset.Name != "a2ahoy-darwin-arm64" {
		t.Errorf("Name = %q", asset.Name)
	}
	if asset.BrowserDownloadURL != "https://example.com/2" {
		t.Errorf("BrowserDownloadURL = %q", asset.BrowserDownloadURL)
	}
}

func TestRelease_FindAssetForPlatform_NotFound(t *testing.T) {
	rel := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{Name: "a2ahoy-linux-amd64"},
		},
	}
	_, err := rel.FindAssetForPlatform("a2ahoy-darwin-arm64")
	if err == nil {
		t.Fatal("expected error for missing asset")
	}
	if !strings.Contains(err.Error(), "a2ahoy-darwin-arm64") {
		t.Errorf("error should include the asset name: %v", err)
	}
	if !strings.Contains(err.Error(), "v1.0.0") {
		t.Errorf("error should include the tag: %v", err)
	}
	if !strings.Contains(err.Error(), "a2ahoy-linux-amd64") {
		t.Errorf("error should list available assets: %v", err)
	}
}
