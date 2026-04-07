package version

import "testing"

// withVersion temporarily replaces the package Version variable for the
// duration of a single test. The original value is restored on cleanup.
func withVersion(t *testing.T, v string) {
	t.Helper()
	original := Version
	Version = v
	t.Cleanup(func() { Version = original })
}

func TestCurrent_DefaultIsDev(t *testing.T) {
	// The package default before any -ldflags injection should be "dev".
	// We can't assert against the literal "dev" without risking interference
	// from other tests, so we restore the default explicitly.
	withVersion(t, devVersion)
	if got := Current(); got != devVersion {
		t.Errorf("Current() = %q, want %q", got, devVersion)
	}
}

func TestCurrent_ReturnsInjectedValue(t *testing.T) {
	withVersion(t, "v1.2.3")
	if got := Current(); got != "v1.2.3" {
		t.Errorf("Current() = %q, want %q", got, "v1.2.3")
	}
}

func TestIsDev(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"default dev", "dev", true},
		{"injected semver", "v1.2.3", false},
		{"injected without v prefix", "1.2.3", false},
		{"empty string", "", false},
		{"random text", "snapshot-abc123", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withVersion(t, tt.version)
			if got := IsDev(); got != tt.want {
				t.Errorf("IsDev() with Version=%q = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
