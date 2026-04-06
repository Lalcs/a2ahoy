package cmd

import (
	"io"
	"strings"
	"testing"
)

func TestExecute_NoArgs(t *testing.T) {
	rootCmd.SetArgs([]string{})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRootCommand_HasSubcommands(t *testing.T) {
	commands := rootCmd.Commands()
	names := make(map[string]bool)
	for _, cmd := range commands {
		names[cmd.Name()] = true
	}

	expected := []string{"card", "send", "stream"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestRootCommand_HasPersistentFlags(t *testing.T) {
	tests := []struct {
		flag string
	}{
		{"gcp-auth"},
		{"json"},
	}
	for _, tt := range tests {
		f := rootCmd.PersistentFlags().Lookup(tt.flag)
		if f == nil {
			t.Errorf("missing persistent flag %q", tt.flag)
		}
	}
}

func TestCardCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"card"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestCardCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"card", "url1", "url2"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestSendCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"send"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestSendCommand_OneArg(t *testing.T) {
	rootCmd.SetArgs([]string{"send", "url"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for insufficient args")
	}
}

func TestSendCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"send", "url", "msg", "extra"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestStreamCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"stream"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestStreamCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"stream", "url", "msg", "extra"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestRootCommand_GCPAuthAndVertexAIMutuallyExclusive(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "both flags returns error",
			args:    []string{"--gcp-auth", "--vertex-ai", "card", "http://example.com"},
			wantErr: true,
		},
		{
			name:    "gcp-auth only is allowed",
			args:    []string{"--gcp-auth", "card", "http://example.com"},
			wantErr: false,
		},
		{
			name:    "vertex-ai only is allowed",
			args:    []string{"--vertex-ai", "card", "http://example.com"},
			wantErr: false,
		},
		{
			name:    "neither flag is allowed",
			args:    []string{"card", "http://example.com"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags before each test case.
			flagGCPAuth = false
			flagVertexAI = false

			rootCmd.SetArgs(tt.args)
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)

			err := rootCmd.Execute()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !strings.Contains(err.Error(), "cannot be used together") {
					t.Errorf("unexpected error message: %v", err)
				}
			}
			// Non-error cases may still fail on network access; only check the
			// mutual-exclusion error is absent.
			if !tt.wantErr && err != nil && strings.Contains(err.Error(), "cannot be used together") {
				t.Fatalf("unexpected mutual-exclusion error: %v", err)
			}
		})
	}
}

func TestRootCommand_Description(t *testing.T) {
	if !strings.Contains(rootCmd.Long, "A2A") {
		t.Errorf("root command long description should mention A2A, got: %s", rootCmd.Long)
	}
}
