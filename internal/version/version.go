// Package version exposes the build-time version of the a2ahoy binary.
//
// Version is intended to be set at build time via -ldflags:
//
//	go build -ldflags "-X github.com/Lalcs/a2ahoy/internal/version.Version=v1.2.3" .
//
// When built without -ldflags injection (e.g. `go build .` or `go run .`),
// Version retains its default value of "dev", which the update command
// treats as a development build that should always be replaced.
package version

// devVersion is the sentinel value indicating a non-release build.
const devVersion = "dev"

// Version is the build version of the running binary.
//
// It is declared as a package-level var (not const) so the Go linker can
// rewrite it via -ldflags "-X". Tests may also override it temporarily.
var Version = devVersion

// Current returns the version string of the running binary.
func Current() string {
	return Version
}

// IsDev reports whether the running binary is a development build (i.e.
// Version was not injected at build time).
func IsDev() bool {
	return Version == devVersion
}
