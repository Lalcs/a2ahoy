package updater

import (
	"fmt"
	"runtime"
)

// SupportedPlatform identifies an OS/architecture combo for which the
// release pipeline (.github/workflows/release.yml) builds a binary.
type SupportedPlatform struct {
	OS   string
	Arch string
}

// AssetName returns the release asset filename for this platform. The
// naming convention matches release.yml:
//
//	a2ahoy-{os}-{arch}
//
// e.g. a2ahoy-darwin-arm64, a2ahoy-linux-amd64.
func (p SupportedPlatform) AssetName() string {
	return fmt.Sprintf("a2ahoy-%s-%s", p.OS, p.Arch)
}

// CurrentPlatform returns the SupportedPlatform for the running process,
// or an error if self-update is not supported on this OS/architecture.
//
// It is a thin wrapper around DetectPlatform that injects runtime values.
func CurrentPlatform() (SupportedPlatform, error) {
	return DetectPlatform(runtime.GOOS, runtime.GOARCH)
}

// DetectPlatform validates an arbitrary GOOS/GOARCH pair. It is exported
// (rather than inlined into CurrentPlatform) so unit tests can exercise
// every combination without depending on runtime values.
func DetectPlatform(goos, goarch string) (SupportedPlatform, error) {
	switch goos {
	case "linux", "darwin":
		// supported
	case "windows":
		return SupportedPlatform{}, fmt.Errorf(
			"self-update is not supported on Windows; please download the latest release manually from https://github.com/Lalcs/a2ahoy/releases")
	default:
		return SupportedPlatform{}, fmt.Errorf(
			"unsupported OS %q for self-update (supported: linux, darwin)", goos)
	}

	switch goarch {
	case "amd64", "arm64":
		// supported
	default:
		return SupportedPlatform{}, fmt.Errorf(
			"unsupported architecture %q for self-update (supported: amd64, arm64)", goarch)
	}

	return SupportedPlatform{OS: goos, Arch: goarch}, nil
}
