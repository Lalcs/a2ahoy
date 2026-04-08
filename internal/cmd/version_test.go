package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/Lalcs/a2ahoy/internal/version"
)

// TestVersionCommand_PrintsVersion verifies that `a2ahoy version` writes the
// current build version in the canonical "a2ahoy version <ver>\n" format.
// This format matches the --version flag output (see SetVersionTemplate in
// root.go) so scripts parsing either entry point see identical bytes.
func TestVersionCommand_PrintsVersion(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetArgs([]string{"version"})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "a2ahoy version " + version.Current() + "\n"
	if got := buf.String(); got != want {
		t.Errorf("version output: got %q, want %q", got, want)
	}
}

// TestVersionCommand_NoArgs guards the cobra.NoArgs constraint — the version
// command takes no positional arguments and should error if any are supplied.
func TestVersionCommand_NoArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"version", "unexpected"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for extra args")
	}
}

// TestVersionCommand_RespectsInjectedVersion simulates a release build by
// temporarily overriding version.Version (as -ldflags would at build time)
// and verifies the command reflects the injected value. t.Cleanup restores
// the original so subsequent tests see the default "dev".
func TestVersionCommand_RespectsInjectedVersion(t *testing.T) {
	original := version.Version
	t.Cleanup(func() { version.Version = original })

	version.Version = "v9.9.9-test"

	var buf bytes.Buffer
	rootCmd.SetArgs([]string{"version"})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "a2ahoy version v9.9.9-test\n"
	if got := buf.String(); got != want {
		t.Errorf("injected version output: got %q, want %q", got, want)
	}
}

// TestRootCommand_VersionFlag confirms the --version flag, enabled by setting
// rootCmd.Version in init(), produces the same format as the version
// subcommand. Keeping both paths aligned means users (and their scripts)
// don't have to care which entry point they use.
func TestRootCommand_VersionFlag(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetArgs([]string{"--version"})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "a2ahoy version " + version.Current() + "\n"
	if got := buf.String(); got != want {
		t.Errorf("--version output: got %q, want %q", got, want)
	}
}

// TestRootCommand_HelpContainsVersion ensures that `a2ahoy --help` surfaces
// the current version somewhere in its output. We don't assert the full
// rendered help (Cobra owns that formatting) — only that the version string
// is present, which is the user-facing guarantee this feature makes.
func TestRootCommand_HelpContainsVersion(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetArgs([]string{"--help"})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := buf.String(); !strings.Contains(got, version.Current()) {
		t.Errorf("--help output missing version %q; got:\n%s", version.Current(), got)
	}
}
