package cmd

import (
	"io"
	"testing"
)

func TestPushCommand_HasSubcommands(t *testing.T) {
	commands := pushCmd.Commands()
	names := make(map[string]bool)
	for _, cmd := range commands {
		names[cmd.Name()] = true
	}

	expected := []string{"set", "get", "list", "delete"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("push command missing subcommand %q", name)
		}
	}
}

func TestPushSetCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "set"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestPushSetCommand_OneArg(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "set", "url"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for insufficient args")
	}
}

func TestPushSetCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "set", "url", "task-1", "extra"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestPushSetCommand_HasRequiredURLFlag(t *testing.T) {
	f := pushSetCmd.Flags().Lookup(flagNamePushURL)
	if f == nil {
		t.Fatal("push set command missing --url flag")
	}
}

func TestPushSetCommand_HasOptionalFlags(t *testing.T) {
	flags := []string{
		flagNamePushID,
		flagNamePushToken,
		flagNamePushAuthScheme,
		flagNamePushAuthCredentials,
	}
	for _, name := range flags {
		f := pushSetCmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("push set command missing --%s flag", name)
		}
	}
}

func TestPushGetCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "get"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestPushGetCommand_TwoArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "get", "url", "task-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for insufficient args")
	}
}

func TestPushGetCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "get", "url", "task-1", "cfg-1", "extra"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestPushListCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "list"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestPushListCommand_OneArg(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "list", "url"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for insufficient args")
	}
}

func TestPushListCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "list", "url", "task-1", "extra"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestPushListCommand_HasFlags(t *testing.T) {
	flags := []string{flagNamePushPageSize, flagNamePushPageToken}
	for _, name := range flags {
		f := pushListCmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("push list command missing --%s flag", name)
		}
	}
}

func TestPushDeleteCommand_MissingArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "delete"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestPushDeleteCommand_TwoArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "delete", "url", "task-1"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for insufficient args")
	}
}

func TestPushDeleteCommand_TooManyArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"push", "delete", "url", "task-1", "cfg-1", "extra"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}
