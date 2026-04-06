package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	flagGCPAuth  bool
	flagJSON     bool
	flagVertexAI bool
)

var rootCmd = &cobra.Command{
	Use:   "a2ahoy",
	Short: "A2A protocol CLI tool",
	Long:  "a2ahoy is a CLI tool for interacting with A2A (Agent-to-Agent) protocol agents.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if flagGCPAuth && flagVertexAI {
			return fmt.Errorf("--gcp-auth and --vertex-ai cannot be used together")
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagGCPAuth, "gcp-auth", false, "Enable GCP ADC authentication (ID token as Bearer)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON")
	rootCmd.PersistentFlags().BoolVar(&flagVertexAI, "vertex-ai", false, "Treat the URL as a Vertex AI Agent Engine endpoint")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
