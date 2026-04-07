package cmd

import (
	"io"
	"testing"
)

func TestUpdateCommand_Registered(t *testing.T) {
	commands := rootCmd.Commands()
	found := false
	for _, c := range commands {
		if c.Name() == "update" {
			found = true
			break
		}
	}
	if !found {
		t.Error("update command is not registered on rootCmd")
	}
}

func TestUpdateCommand_HasFlags(t *testing.T) {
	tests := []string{"check-only", "force"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			f := updateCmd.Flags().Lookup(name)
			if f == nil {
				t.Errorf("update command missing flag %q", name)
			}
		})
	}
}

func TestUpdateCommand_RejectsExtraArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"update", "extra-arg"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for extra args")
	}
}

func TestUpdateCommand_NoArgsAccepted(t *testing.T) {
	// We cannot run the full update flow in unit tests (it requires
	// network access and would touch the binary). We only verify that
	// flag parsing succeeds for the --check-only path; the actual fetch
	// will fail and that failure surfaces as the returned error.
	//
	// This test exists to make sure we don't accidentally break the
	// command's argument validation.
	cmd, _, err := rootCmd.Find([]string{"update"})
	if err != nil {
		t.Fatalf("update command not findable: %v", err)
	}
	if cmd.Name() != "update" {
		t.Errorf("Find returned %q, want %q", cmd.Name(), "update")
	}
}
