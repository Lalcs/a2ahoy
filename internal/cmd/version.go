package cmd

import (
	"fmt"

	"github.com/Lalcs/a2ahoy/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the a2ahoy version",
	Long: `Prints the version of the running a2ahoy binary.

Outputs "dev" for non-release builds (built without -ldflags injection).
Released binaries carry their git tag (e.g. v1.2.3).`,
	Args: cobra.NoArgs,
	RunE: runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// runVersion writes the current binary version to the command's stdout.
//
// The output format intentionally mirrors the --version flag (configured via
// SetVersionTemplate in root.go) so both entry points produce identical
// output, which is easier for scripts to parse consistently.
func runVersion(cmd *cobra.Command, args []string) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "a2ahoy version %s\n", version.Current())
	return err
}
