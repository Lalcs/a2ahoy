package updater

import (
	"runtime"
	"strings"
	"testing"
)

func TestDetectPlatform_Supported(t *testing.T) {
	tests := []struct {
		goos     string
		goarch   string
		wantName string
	}{
		{"linux", "amd64", "a2ahoy-linux-amd64"},
		{"linux", "arm64", "a2ahoy-linux-arm64"},
		{"darwin", "amd64", "a2ahoy-darwin-amd64"},
		{"darwin", "arm64", "a2ahoy-darwin-arm64"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"/"+tt.goarch, func(t *testing.T) {
			p, err := DetectPlatform(tt.goos, tt.goarch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.OS != tt.goos {
				t.Errorf("OS = %q, want %q", p.OS, tt.goos)
			}
			if p.Arch != tt.goarch {
				t.Errorf("Arch = %q, want %q", p.Arch, tt.goarch)
			}
			if got := p.AssetName(); got != tt.wantName {
				t.Errorf("AssetName() = %q, want %q", got, tt.wantName)
			}
		})
	}
}

func TestDetectPlatform_Windows(t *testing.T) {
	_, err := DetectPlatform("windows", "amd64")
	if err == nil {
		t.Fatal("expected error for Windows")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Windows") {
		t.Errorf("error should mention Windows: %v", err)
	}
	if !strings.Contains(msg, "https://github.com/Lalcs/a2ahoy/releases") {
		t.Errorf("error should include the releases URL: %v", err)
	}
}

func TestDetectPlatform_UnsupportedOS(t *testing.T) {
	tests := []string{"freebsd", "openbsd", "netbsd", "plan9", "solaris", ""}
	for _, goos := range tests {
		t.Run(goos, func(t *testing.T) {
			_, err := DetectPlatform(goos, "amd64")
			if err == nil {
				t.Fatalf("expected error for %q", goos)
			}
			if !strings.Contains(err.Error(), "unsupported OS") {
				t.Errorf("error should mention 'unsupported OS': %v", err)
			}
		})
	}
}

func TestDetectPlatform_UnsupportedArch(t *testing.T) {
	tests := []string{"386", "arm", "mips", "ppc64le", "riscv64", ""}
	for _, goarch := range tests {
		t.Run(goarch, func(t *testing.T) {
			_, err := DetectPlatform("linux", goarch)
			if err == nil {
				t.Fatalf("expected error for %q", goarch)
			}
			if !strings.Contains(err.Error(), "unsupported architecture") {
				t.Errorf("error should mention 'unsupported architecture': %v", err)
			}
		})
	}
}

func TestCurrentPlatform_MatchesRuntime(t *testing.T) {
	// CurrentPlatform succeeds only on platforms supported by the release
	// pipeline. We skip the test on unsupported runtimes rather than
	// failing, since the unit test process itself may be running there.
	p, err := CurrentPlatform()
	if err != nil {
		t.Skipf("self-update not supported on %s/%s: %v", runtime.GOOS, runtime.GOARCH, err)
	}
	if p.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", p.OS, runtime.GOOS)
	}
	if p.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", p.Arch, runtime.GOARCH)
	}
}

func TestSupportedPlatform_AssetName(t *testing.T) {
	p := SupportedPlatform{OS: "darwin", Arch: "arm64"}
	if got := p.AssetName(); got != "a2ahoy-darwin-arm64" {
		t.Errorf("AssetName() = %q, want %q", got, "a2ahoy-darwin-arm64")
	}
}
