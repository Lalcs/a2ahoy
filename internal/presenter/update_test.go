package presenter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Lalcs/a2ahoy/internal/updater"
)

func TestPrintUpdateChecking(t *testing.T) {
	var buf bytes.Buffer
	PrintUpdateChecking(&buf)
	out := buf.String()
	if !strings.Contains(out, "[update]") {
		t.Errorf("expected [update] tag, got %q", out)
	}
	if !strings.Contains(out, "Checking latest release") {
		t.Errorf("expected progress message, got %q", out)
	}
}

func TestPrintUpdateDecision(t *testing.T) {
	tests := []struct {
		name     string
		decision updater.Decision
		wantSubs []string
	}{
		{
			name: "update available",
			decision: updater.Decision{
				Action:  updater.ActionUpdate,
				Current: "v1.0.0",
				Latest:  "v1.1.0",
			},
			wantSubs: []string{"=== Update ===", "Current:", "v1.0.0", "Latest:", "v1.1.0", "update available"},
		},
		{
			name: "up to date",
			decision: updater.Decision{
				Action:  updater.ActionUpToDate,
				Current: "v1.1.0",
				Latest:  "v1.1.0",
			},
			wantSubs: []string{"v1.1.0", "up to date"},
		},
		{
			name: "development",
			decision: updater.Decision{
				Action:  updater.ActionDevelopment,
				Current: "dev",
				Latest:  "v1.1.0",
			},
			wantSubs: []string{"dev", "development build"},
		},
		{
			name: "ahead",
			decision: updater.Decision{
				Action:  updater.ActionAhead,
				Current: "v2.0.0",
				Latest:  "v1.1.0",
			},
			wantSubs: []string{"ahead of latest"},
		},
		{
			name: "force reinstall",
			decision: updater.Decision{
				Action:  updater.ActionForceReinstall,
				Current: "v1.1.0",
				Latest:  "v1.1.0",
			},
			wantSubs: []string{"force reinstall"},
		},
		{
			name: "invalid latest",
			decision: updater.Decision{
				Action:  updater.ActionInvalidLatest,
				Current: "v1.0.0",
				Latest:  "garbage",
			},
			wantSubs: []string{"invalid latest tag"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			PrintUpdateDecision(&buf, tt.decision)
			out := buf.String()
			for _, sub := range tt.wantSubs {
				if !strings.Contains(out, sub) {
					t.Errorf("output missing %q\nfull output:\n%s", sub, out)
				}
			}
		})
	}
}

func TestPrintUpdateAlreadyLatest(t *testing.T) {
	var buf bytes.Buffer
	PrintUpdateAlreadyLatest(&buf, "v1.2.3")
	out := buf.String()
	if !strings.Contains(out, "Already up to date") {
		t.Errorf("expected 'Already up to date', got %q", out)
	}
	if !strings.Contains(out, "v1.2.3") {
		t.Errorf("expected version, got %q", out)
	}
}

func TestPrintUpdateAhead(t *testing.T) {
	var buf bytes.Buffer
	PrintUpdateAhead(&buf, "v2.0.0", "v1.0.0")
	out := buf.String()
	if !strings.Contains(out, "v2.0.0") || !strings.Contains(out, "v1.0.0") {
		t.Errorf("expected both versions in output: %q", out)
	}
	if !strings.Contains(out, "ahead") {
		t.Errorf("expected 'ahead' message: %q", out)
	}
}

func TestPrintUpdateAvailable(t *testing.T) {
	var buf bytes.Buffer
	PrintUpdateAvailable(&buf, "v1.0.0", "v1.1.0")
	out := buf.String()
	if !strings.Contains(out, "Update available") {
		t.Errorf("expected 'Update available', got %q", out)
	}
	if !strings.Contains(out, "v1.0.0") || !strings.Contains(out, "v1.1.0") {
		t.Errorf("expected both versions: %q", out)
	}
	if !strings.Contains(out, "a2ahoy update") {
		t.Errorf("expected install hint: %q", out)
	}
}

func TestPrintUpdateDownloading(t *testing.T) {
	var buf bytes.Buffer
	PrintUpdateDownloading(&buf, "a2ahoy-darwin-arm64", 1234567)
	out := buf.String()
	if !strings.Contains(out, "Downloading") {
		t.Errorf("expected 'Downloading', got %q", out)
	}
	if !strings.Contains(out, "a2ahoy-darwin-arm64") {
		t.Errorf("expected asset name, got %q", out)
	}
	if !strings.Contains(out, "MiB") {
		t.Errorf("expected MiB unit for large size, got %q", out)
	}
}

func TestPrintUpdateSuccess(t *testing.T) {
	var buf bytes.Buffer
	PrintUpdateSuccess(&buf, "v1.0.0", "v1.1.0", "/home/user/.local/bin/a2ahoy")
	out := buf.String()
	for _, sub := range []string{"Successfully updated", "v1.0.0", "v1.1.0", "/home/user/.local/bin/a2ahoy"} {
		if !strings.Contains(out, sub) {
			t.Errorf("output missing %q\nfull output:\n%s", sub, out)
		}
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024*1024 - 1, "1024.0 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024 * 1024 * 5, "5.0 MiB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := humanSize(tt.bytes); got != tt.want {
				t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestStyledUpdateAction(t *testing.T) {
	tests := []struct {
		action updater.Action
		want   string
	}{
		{updater.ActionUpToDate, "up to date"},
		{updater.ActionUpdate, "update available"},
		{updater.ActionDevelopment, "development build (will install latest)"},
		{updater.ActionAhead, "ahead of latest"},
		{updater.ActionForceReinstall, "force reinstall"},
		{updater.ActionInvalidLatest, "invalid latest tag"},
		{updater.Action(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			// color.NoColor is true via TestMain so we get plain text.
			if got := styledUpdateAction(tt.action); got != tt.want {
				t.Errorf("styledUpdateAction(%v) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}
