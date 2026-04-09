package cmd

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

	expected := []string{"card", "chat", "push", "send", "stream", "task", "update", "version"}
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
		{"vertex-ai"},
		{"header"},
		{"bearer-token"},
	}
	for _, tt := range tests {
		f := rootCmd.PersistentFlags().Lookup(tt.flag)
		if f == nil {
			t.Errorf("missing persistent flag %q", tt.flag)
		}
	}
}

func TestRootCommand_HeaderFlagIsStringArray(t *testing.T) {
	// The --header flag must use StringArrayVar (not StringSliceVar) so
	// that values containing commas are preserved verbatim (e.g.,
	// `--header "Accept=application/json, text/plain"`).
	f := rootCmd.PersistentFlags().Lookup("header")
	if f == nil {
		t.Fatal("header flag missing")
	}
	if got := f.Value.Type(); got != "stringArray" {
		t.Errorf("--header flag type: got %q, want %q", got, "stringArray")
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

func TestTaskCommand_HasSubcommands(t *testing.T) {
	commands := taskCmd.Commands()
	names := make(map[string]bool)
	for _, cmd := range commands {
		names[cmd.Name()] = true
	}

	expected := []string{"cancel", "get", "list"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing task subcommand %q", name)
		}
	}
}

func TestTaskGetCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"task", "get"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestTaskGetCommand_OneArg(t *testing.T) {
	rootCmd.SetArgs([]string{"task", "get", "url"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for insufficient args")
	}
}

func TestTaskGetCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"task", "get", "url", "task-1", "extra"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestTaskCancelCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"task", "cancel"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestTaskCancelCommand_OneArg(t *testing.T) {
	rootCmd.SetArgs([]string{"task", "cancel", "url"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for insufficient args")
	}
}

func TestTaskCancelCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"task", "cancel", "url", "task-1", "extra"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestChatCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"chat"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestChatCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"chat", "url1", "url2"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestChatCommand_HasSimpleFlag(t *testing.T) {
	// The chat command must expose --simple as a local flag so users can
	// opt out of the TUI when IME or pipeline constraints demand
	// line-mode input.
	f := chatCmd.Flags().Lookup("simple")
	if f == nil {
		t.Fatal("chat command missing --simple flag")
	}
	if got := f.Value.Type(); got != "bool" {
		t.Errorf("--simple flag type: got %q, want %q", got, "bool")
	}
}

func TestCommands_HaveFileFlags(t *testing.T) {
	cmds := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"send", sendCmd},
		{"stream", streamCmd},
		{"chat", chatCmd},
	}
	for _, tt := range cmds {
		for _, flag := range []string{"file", "file-url"} {
			t.Run(tt.name+"/"+flag, func(t *testing.T) {
				f := tt.cmd.Flags().Lookup(flag)
				if f == nil {
					t.Fatalf("%s command missing --%s flag", tt.name, flag)
				}
				if got := f.Value.Type(); got != "stringArray" {
					t.Errorf("--%s flag type: got %q, want %q", flag, got, "stringArray")
				}
			})
		}
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

func TestRootCommand_BearerTokenMutualExclusion(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "bearer-token + gcp-auth returns error",
			args:    []string{"--bearer-token=x", "--gcp-auth", "card", "http://example.com"},
			wantErr: true,
		},
		{
			name:    "bearer-token + vertex-ai returns error",
			args:    []string{"--bearer-token=x", "--vertex-ai", "card", "http://example.com"},
			wantErr: true,
		},
		{
			name:    "bearer-token only is allowed",
			args:    []string{"--bearer-token=x", "card", "http://example.com"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags before each test case.
			flagGCPAuth = false
			flagVertexAI = false
			flagBearerToken = ""
			// Ensure env var cannot leak into the test.
			t.Setenv(bearerTokenEnvVar, "")

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
			// Non-error cases may still fail on network access; only check
			// the mutual-exclusion error is absent.
			if !tt.wantErr && err != nil && strings.Contains(err.Error(), "cannot be used together") {
				t.Fatalf("unexpected mutual-exclusion error: %v", err)
			}
		})
	}
}

func TestRootCommand_BearerTokenEnvFallback(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		setEnv    bool
		flagValue string
		setFlag   bool
		want      string
	}{
		{
			name:     "env set, flag empty",
			envValue: "foo",
			setEnv:   true,
			want:     "foo",
		},
		{
			name:      "flag set, env empty",
			flagValue: "bar",
			setFlag:   true,
			want:      "bar",
		},
		{
			name:      "flag set, env set (flag wins)",
			envValue:  "foo",
			setEnv:    true,
			flagValue: "bar",
			setFlag:   true,
			want:      "bar",
		},
		{
			name: "neither set",
			want: "",
		},
		{
			name:     "env whitespace only is trimmed to empty",
			envValue: "   ",
			setEnv:   true,
			want:     "",
		},
		{
			name:     "env surrounding whitespace is trimmed",
			envValue: " token ",
			setEnv:   true,
			want:     "token",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags before each test case.
			flagGCPAuth = false
			flagVertexAI = false
			flagBearerToken = ""

			// Always override the env var so ambient environment cannot leak.
			if tt.setEnv {
				t.Setenv(bearerTokenEnvVar, tt.envValue)
			} else {
				t.Setenv(bearerTokenEnvVar, "")
			}

			args := []string{}
			if tt.setFlag {
				args = append(args, "--bearer-token="+tt.flagValue)
			}
			args = append(args, "card", "http://127.0.0.1:1")

			rootCmd.SetArgs(args)
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)

			// Execute triggers PersistentPreRunE which is what we're testing.
			// The card subcommand itself will fail on network access, which
			// is fine — we only care about the post-PreRunE value of
			// flagBearerToken.
			_ = rootCmd.Execute()

			if flagBearerToken != tt.want {
				t.Errorf("flagBearerToken: got %q, want %q", flagBearerToken, tt.want)
			}
		})
	}
}
