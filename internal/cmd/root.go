package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	flagGCPAuth  bool
	flagJSON     bool
	flagVertexAI bool
	flagNoColor  bool
)

var rootCmd = &cobra.Command{
	Use:   "a2ahoy",
	Short: "A2A protocol CLI tool",
	Long:  "a2ahoy is a CLI tool for interacting with A2A (Agent-to-Agent) protocol agents.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if flagNoColor {
			color.NoColor = true
		}
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
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
